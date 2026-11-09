package hcl

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Phase 7: user-controlled values must never change the structure of the
// generated HCL (CLAUDE.md, "never interpolate unvalidated user input").

func moduleNode(name, folder string, values map[string]interface{}) GenerateNode {
	return GenerateNode{
		ID: "n1", ModuleName: "resource-group", ModuleVersion: "v1",
		InstanceName: name, TargetFolder: folder,
		Source: "git::https://example.invalid/modules//resource-group?ref=v1", Values: values,
	}
}

func parseBody(t *testing.T, src string) *hclsyntax.Body {
	t.Helper()
	f, diags := hclsyntax.ParseConfig([]byte(src), "gen.tf", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("generated HCL does not parse: %v\n%s", diags, src)
	}
	return f.Body.(*hclsyntax.Body)
}

func TestGenerate_HostileValuesStayInsideTheirString(t *testing.T) {
	hostile := []string{
		"a\" }\nresource \"azurerm_resource_group\" \"evil\" {\n  name = \"x",
		"${file(\"/etc/passwd\")}",
		"%{ for x in [1] }y%{ endfor }",
		"line1\nline2\ttab \\ backslash \"quote\"",
		"}\nmodule \"evil\" {\n  source = \"./x\"\n}\n#",
		"*/ /* # // end",
		"héllo ✓ ไทย",
		"",
	}
	for _, v := range hostile {
		out, err := Generate(GenerateInput{Nodes: []GenerateNode{moduleNode("rg_one", "common", map[string]interface{}{"name": v, "location": "southeastasia"})}})
		if err != nil {
			t.Fatalf("value %q: %v", v, err)
		}
		body := parseBody(t, out.Files[0].Content)
		if len(body.Blocks) != 1 || body.Blocks[0].Type != "module" || body.Blocks[0].Labels[0] != "rg_one" {
			t.Fatalf("value %q changed the structure: %d blocks\n%s", v, len(body.Blocks), out.Files[0].Content)
		}
		attr := body.Blocks[0].Body.Attributes["name"]
		if attr == nil {
			t.Fatalf("value %q: name attribute missing", v)
		}
		got, diags := attr.Expr.Value(nil) // evaluating with no variables fails if it is not a plain string
		if diags.HasErrors() {
			t.Fatalf("value %q is not a plain literal: %v\n%s", v, diags, out.Files[0].Content)
		}
		if got.AsString() != v {
			t.Errorf("value changed: got %q, want %q", got.AsString(), v)
		}
	}
}

func TestGenerate_HostileMapKeysAndNestedValues(t *testing.T) {
	vals := map[string]interface{}{
		"name":     "rg",
		"location": "southeastasia",
		"tags": map[string]interface{}{
			"k\" = \"v\"\n  evil = \"1": "ok",
			"normal":                    "v\"}\nresource \"x\" \"y\" {}",
		},
		"list": []interface{}{"a\"b", "${var.x}"},
	}
	out, err := Generate(GenerateInput{Nodes: []GenerateNode{moduleNode("rg_one", "common", vals)}})
	if err != nil {
		t.Fatal(err)
	}
	body := parseBody(t, out.Files[0].Content)
	if len(body.Blocks) != 1 {
		t.Fatalf("structure changed:\n%s", out.Files[0].Content)
	}
	for name := range body.Blocks[0].Body.Attributes {
		if name != "name" && name != "location" && name != "tags" && name != "list" && name != "source" && name != "version" {
			t.Errorf("an attribute appeared from user input: %q", name)
		}
	}
}

func TestGenerate_RejectsUnsafeNamesAndPaths(t *testing.T) {
	badFolders := []string{"", "..", "../x", "a/../b", "/abs", `a\b`, "C:", "C:x"}
	for _, f := range badFolders {
		if _, err := Generate(GenerateInput{Nodes: []GenerateNode{moduleNode("rg_one", f, map[string]interface{}{"name": "x"})}}); err == nil {
			t.Errorf("folder %q must be refused", f)
		}
	}
	badFiles := []string{"../x.tf", "a/b.tf", `a\b.tf`, ".tf", "x.tf.sh", "x", "x.TF", "..tf", "a b.tf", "a\x00.tf", "x.tf\n", strings.Repeat("a", 80) + ".tf"}
	for _, name := range badFiles {
		n := moduleNode("rg_one", "common", map[string]interface{}{"name": "x"})
		n.Filename = name
		if _, err := Generate(GenerateInput{Nodes: []GenerateNode{n}}); err == nil {
			t.Errorf("file name %q must be refused", name)
		}
	}
	badLabels := []string{"", "1abc", "a b", "a\"b", "a{b", "a.b", "a/b", "evil\" {\n}\nmodule \"x"}
	for _, l := range badLabels {
		if _, err := Generate(GenerateInput{Nodes: []GenerateNode{moduleNode(l, "common", map[string]interface{}{"name": "x"})}}); err == nil {
			t.Errorf("module label %q must be refused", l)
		}
	}
}

func TestGenerate_RawResourceTypesAndNames(t *testing.T) {
	good := GenerateNode{ID: "r", InstanceName: "rg_one", TargetFolder: "common", Kind: "resource", ResourceType: "azurerm_resource_group",
		Values: map[string]interface{}{"name": "x", "location": "southeastasia"}}
	if _, err := Generate(GenerateInput{Nodes: []GenerateNode{good}}); err != nil {
		t.Fatalf("a normal raw resource must work: %v", err)
	}
	for _, typ := range []string{"", "AzureRM", "azurerm resource", "azurerm_x\" \"y\" {\n}\nresource \"z", "../x", "1azurerm_x", "azurerm.x"} {
		n := good
		n.ResourceType = typ
		if _, err := Generate(GenerateInput{Nodes: []GenerateNode{n}}); err == nil {
			t.Errorf("resource type %q must be refused", typ)
		}
	}
}

func TestGenerate_ExprIsTheOnlyWayToWriteAReference(t *testing.T) {
	// A plain string that looks like a reference stays a string...
	out, err := Generate(GenerateInput{Nodes: []GenerateNode{moduleNode("rg_one", "common", map[string]interface{}{"name": "var.name"})}})
	if err != nil {
		t.Fatal(err)
	}
	body := parseBody(t, out.Files[0].Content)
	v, diags := body.Blocks[0].Body.Attributes["name"].Expr.Value(nil)
	if diags.HasErrors() || v.AsString() != "var.name" {
		t.Fatalf("plain text must stay a string literal: %v %v", v, diags)
	}
	// ...and only the explicit marker writes an expression.
	out, err = Generate(GenerateInput{Nodes: []GenerateNode{moduleNode("rg_one", "common", map[string]interface{}{"name": map[string]interface{}{"$expr": "var.name"}})}})
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^\s+name\s+= var\.name$`).MatchString(out.Files[0].Content) {
		t.Fatalf("$expr must be written as a reference:\n%s", out.Files[0].Content)
	}
}
