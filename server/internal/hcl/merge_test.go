package hcl

import (
	"strings"
	"testing"
)

const (
	rgSrc = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

	// Same shape as the team's common/rg.tf: three hand-written module calls with
	// comments and expressions. This is the file the old generator destroyed.
	rgFile = `# Resource groups for the project. Other folders look these up with
# data "azurerm_resource_group" using local.rg_names.

module "rg_network" {
  source = "` + rgSrc + `"

  name     = local.rg_names.network
  location = var.location
  tags     = local.common_tags
}

module "rg_app" {
  source = "` + rgSrc + `"

  name       = local.rg_names.app
  location   = var.location
  lock_level = var.environment == "prod" ? "CanNotDelete" : ""
  tags       = local.common_tags
}

module "rg_data" {
  source = "` + rgSrc + `"

  name     = local.rg_names.data
  location = var.location
  tags     = merge(local.common_tags, { data_classification = "confidential" })
}
`
)

func repoWithRG() []GeneratedFile {
	return []GeneratedFile{
		{Folder: "common", Filename: "rg.tf", Content: rgFile},
		{Folder: "common", Filename: "variables.tf", Content: "variable \"location\" {}\n"},
	}
}

func newRG(label string) GenerateNode {
	return GenerateNode{
		ID: "n-" + label, ModuleName: "resource-group", ModuleVersion: "v1.2.0",
		InstanceName: label, TargetFolder: "common", Filename: "rg.tf", Source: rgSrc,
		Values: map[string]interface{}{"name": "test", "location": "southeastasia"},
	}
}

func byPath(t *testing.T, files []ChangedFile) map[string]ChangedFile {
	t.Helper()
	m := map[string]ChangedFile{}
	for _, f := range files {
		m[f.Folder+"/"+f.Filename] = f
	}
	return m
}

// Regression: a new resource group written to rg.tf used to replace the file and
// delete rg_network, rg_app and rg_data. Existing bytes must survive untouched.
func TestApplyAppendsToExistingFileWithoutLosingContent(t *testing.T) {
	out, err := Apply(GenerateInput{Nodes: []GenerateNode{newRG("resource_group")}}, repoWithRG())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("only the changed file should be returned, got %d", len(out))
	}
	f := out[0]
	if f.Folder != "common" || f.Filename != "rg.tf" || f.Status != FileModified {
		t.Fatalf("unexpected result %+v", f.GeneratedFile)
	}
	if !strings.HasPrefix(f.Content, rgFile) {
		t.Fatalf("existing content was not preserved byte for byte:\n%s", f.Content)
	}
	for _, label := range []string{"rg_network", "rg_app", "rg_data", "resource_group"} {
		if !strings.Contains(f.Content, `module "`+label+`"`) {
			t.Errorf("module %q missing from result", label)
		}
	}
	if !strings.Contains(f.Content, `"CanNotDelete"`) || !strings.Contains(f.Content, "local.rg_names.network") {
		t.Error("hand-written expressions were lost")
	}
}

func TestApplyCreatesNewFileMatchingGenerate(t *testing.T) {
	node := GenerateNode{
		ID: "n1", ModuleName: "storage-account", InstanceName: "sa_demo", TargetFolder: "storage",
		Source: "git::x//storage-account?ref=v1", Values: map[string]interface{}{"name": "stdemo01"},
	}
	want, err := Generate(GenerateInput{Nodes: []GenerateNode{node}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Apply(GenerateInput{Nodes: []GenerateNode{node}}, repoWithRG())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != FileAdded || got[0].Content != want.Files[0].Content {
		t.Fatalf("new file must equal plain Generate output: %+v", got)
	}
}

func TestApplyWithNoNodesChangesNothing(t *testing.T) {
	out, err := Apply(GenerateInput{}, repoWithRG())
	if err != nil || len(out) != 0 {
		t.Fatalf("got %v, %v", out, err)
	}
}

func TestApplyEditReplacesOnlyThatBlock(t *testing.T) {
	n := newRG("rg_app")
	n.Values = map[string]interface{}{"name": "rg-edited", "location": "japaneast"}
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}

	out, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, repoWithRG())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(out) != 1 || out[0].Status != FileModified {
		t.Fatalf("got %+v", out)
	}
	got := out[0].Content

	// Everything before rg_app and after it is byte-identical.
	i := strings.Index(rgFile, `module "rg_app"`)
	j := strings.Index(rgFile, `module "rg_data"`)
	before, after := rgFile[:i], rgFile[j:]
	if !strings.HasPrefix(got, before) || !strings.HasSuffix(got, after) {
		t.Fatalf("bytes outside the edited block changed:\n%s", got)
	}
	middle := got[len(before) : len(got)-len(after)]
	if !strings.Contains(middle, `"rg-edited"`) || !strings.Contains(middle, `"japaneast"`) {
		t.Errorf("edited values missing: %s", middle)
	}
	if strings.Count(got, `module "rg_app"`) != 1 {
		t.Error("edit must not duplicate the block")
	}
}

func TestApplyEditCanRenameButNotCollide(t *testing.T) {
	n := newRG("rg_renamed")
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	out, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, repoWithRG())
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if strings.Contains(out[0].Content, `module "rg_app"`) || !strings.Contains(out[0].Content, `module "rg_renamed"`) {
		t.Error("rename not applied")
	}

	clash := newRG("rg_data")
	clash.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{clash}}, repoWithRG()); err == nil {
		t.Fatal("renaming onto another existing module must fail")
	}
}

func TestApplyRejectsDuplicateLabels(t *testing.T) {
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{newRG("rg_app")}}, repoWithRG()); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a new module named like an existing one must fail, got %v", err)
	}
	a, b := newRG("same"), newRG("same")
	b.ID = "n-other"
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{a, b}}, repoWithRG()); err == nil {
		t.Fatal("two canvas nodes with one label must fail")
	}
	// The same label in a different folder is fine.
	other := newRG("rg_app")
	other.TargetFolder = "other"
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{other}}, repoWithRG()); err != nil {
		t.Fatalf("same label in another folder: %v", err)
	}
}

func TestApplyEditFailsWhenTargetIsGone(t *testing.T) {
	missingBlock := newRG("rg_app")
	missingBlock.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_gone"}
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{missingBlock}}, repoWithRG()); err == nil ||
		!strings.Contains(err.Error(), "no longer") {
		t.Fatalf("missing block: %v", err)
	}
	missingFile := newRG("rg_app")
	missingFile.Existing = &ExistingRef{Folder: "common", Filename: "nope.tf", Label: "rg_app"}
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{missingFile}}, repoWithRG()); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestApplyEditFailsOnUnparsableFile(t *testing.T) {
	repo := []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: "module \"rg_app\" {\n  source = \n"}}
	n := newRG("rg_app")
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	if _, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, repo); err == nil {
		t.Fatal("editing a file that does not parse must fail, not guess")
	}
}

func TestApplyRejectsUnsafeTargets(t *testing.T) {
	for _, tc := range []struct{ folder, file string }{
		{"../x", "main.tf"}, {"common", "../main.tf"}, {"common", "main.txt"}, {`a\b`, "main.tf"}, {"/abs", "main.tf"},
	} {
		n := newRG("x")
		n.TargetFolder, n.Filename = tc.folder, tc.file
		if _, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, nil); err == nil {
			t.Errorf("%q/%q should be rejected", tc.folder, tc.file)
		}
		e := newRG("x")
		e.Existing = &ExistingRef{Folder: tc.folder, Filename: tc.file, Label: "x"}
		if _, err := Apply(GenerateInput{Nodes: []GenerateNode{e}}, nil); err == nil {
			t.Errorf("edit target %q/%q should be rejected", tc.folder, tc.file)
		}
	}
}

func TestApplyKeepsCRLFLineEndings(t *testing.T) {
	crlf := strings.ReplaceAll(rgFile, "\n", "\r\n")
	repo := []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: crlf}}

	out, err := Apply(GenerateInput{Nodes: []GenerateNode{newRG("added")}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out[0].Content, crlf) {
		t.Fatal("existing CRLF bytes changed")
	}
	if strings.Contains(strings.ReplaceAll(out[0].Content, "\r\n", ""), "\n") {
		t.Fatal("appended text must use the file's CRLF line endings")
	}
}

func TestApplyAppendsAfterFileWithoutTrailingNewline(t *testing.T) {
	existing := strings.TrimRight(rgFile, "\n")
	repo := []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: existing}}
	out, err := Apply(GenerateInput{Nodes: []GenerateNode{newRG("added")}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out[0].Content, existing+"\n\nmodule \"added\"") {
		t.Fatalf("blocks must be separated by one blank line:\n%s", out[0].Content)
	}
}

func pickRefNode(label string, args map[string]string) GenerateNode {
	return GenerateNode{
		ID: "n-" + label, ModuleName: "storage-account", InstanceName: label, TargetFolder: "storage",
		Source: "git::x//storage-account?ref=v1",
		Values: map[string]interface{}{"name": "stdemo01", "resource_group_name": "ignored"},
		PickRefs: []PickRef{{
			Field: "resource_group_name", DataType: "azurerm_resource_group", DataName: "app",
			Output: "name", LookupArgs: args,
		}},
	}
}

func TestApplyReusesIdenticalDataBlockAndRejectsDifferentOne(t *testing.T) {
	args := map[string]string{"name": "local.rg_names.app"}
	data := "data \"azurerm_resource_group\" \"app\" {\n  name = local.rg_names.app\n}\n"
	repo := []GeneratedFile{{Folder: "storage", Filename: "data.tf", Content: data}}

	out, err := Apply(GenerateInput{Nodes: []GenerateNode{pickRefNode("sa", args)}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	m := byPath(t, out)
	if _, touched := m["storage/data.tf"]; touched {
		t.Error("an identical data block already exists, data.tf must not change")
	}
	if !strings.Contains(m["storage/main.tf"].Content, "data.azurerm_resource_group.app.name") {
		t.Error("module must still reference the data source")
	}

	_, err = Apply(GenerateInput{Nodes: []GenerateNode{pickRefNode("sa", map[string]string{"name": "local.other"})}}, repo)
	if err == nil || !strings.Contains(err.Error(), "different") {
		t.Fatalf("a data block with other arguments must fail, got %v", err)
	}
}

func TestApplyAppendsNewDataBlockToExistingDataFile(t *testing.T) {
	existing := "data \"azurerm_client_config\" \"current\" {}\n"
	repo := []GeneratedFile{{Folder: "storage", Filename: "data.tf", Content: existing}}
	out, err := Apply(GenerateInput{Nodes: []GenerateNode{pickRefNode("sa", map[string]string{"name": "local.rg_names.app"})}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	d := byPath(t, out)["storage/data.tf"]
	if d.Status != FileModified || !strings.HasPrefix(d.Content, existing) {
		t.Fatalf("data.tf must keep its content and gain the new block:\n%s", d.Content)
	}
}

func TestApplyEditWithPickRefWritesDataBlockOnce(t *testing.T) {
	repo := []GeneratedFile{{Folder: "storage", Filename: "storage.tf", Content: "module \"sa\" {\n  source = \"x\"\n}\n"}}
	edit := pickRefNode("sa", map[string]string{"name": "local.rg_names.app"})
	edit.Existing = &ExistingRef{Folder: "storage", Filename: "storage.tf", Label: "sa"}
	sibling := pickRefNode("sa2", map[string]string{"name": "local.rg_names.app"})
	sibling.ID = "n-sa2"

	out, err := Apply(GenerateInput{Nodes: []GenerateNode{edit, sibling}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	m := byPath(t, out)
	if c := strings.Count(m["storage/data.tf"].Content, `data "azurerm_resource_group" "app"`); c != 1 {
		t.Fatalf("shared data block written %d times", c)
	}
}

func TestApplyEditFoldsNewNestedChildIntoExistingParent(t *testing.T) {
	repo := []GeneratedFile{{Folder: "vnet", Filename: "vnet.tf", Content: "# network\nmodule \"vnet\" {\n  source = \"x\"\n}\n\nmodule \"other\" {\n  source = \"y\"\n}\n"}}
	parent := GenerateNode{
		ID: "p", ModuleName: "virtual-network", InstanceName: "vnet", Source: "x",
		Existing: &ExistingRef{Folder: "vnet", Filename: "vnet.tf", Label: "vnet"},
		Values:   map[string]interface{}{"name": "vnet-1"},
	}
	child := GenerateNode{
		ID: "c", ModuleName: "subnet", InstanceName: "snet", ParentID: "p", NestField: "subnets",
		Values: map[string]interface{}{"name": "snet-app", "address_prefixes": []interface{}{"10.0.1.0/24"}},
	}
	out, err := Apply(GenerateInput{Nodes: []GenerateNode{parent, child}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].Content
	if !strings.HasPrefix(got, "# network\n") || !strings.HasSuffix(got, "module \"other\" {\n  source = \"y\"\n}\n") {
		t.Fatalf("surrounding bytes changed:\n%s", got)
	}
	if !strings.Contains(got, `"snet-app"`) || strings.Contains(got, `module "snet"`) {
		t.Fatalf("subnet must be folded into the vnet call:\n%s", got)
	}
}

func TestApplyEditNodeIsNotEmittedByGenerate(t *testing.T) {
	n := newRG("rg_app")
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	out, err := Generate(GenerateInput{Nodes: []GenerateNode{n}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 0 {
		t.Fatalf("an edit is applied to the existing file, not generated as a new one: %+v", out.Files)
	}
}

func TestApplyIsDeterministicAndDoesNotMutateInputs(t *testing.T) {
	repo := repoWithRG()
	nodes := []GenerateNode{newRG("a"), newRG("b")}
	nodes[1].ID = "n-b"
	first, err := Apply(GenerateInput{Nodes: nodes}, repo)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Apply(GenerateInput{Nodes: nodes}, repo)
	if first[0].Content != second[0].Content {
		t.Fatal("output must be deterministic")
	}
	if repo[0].Content != rgFile {
		t.Fatal("Apply must not modify the repo slice it was given")
	}
}
