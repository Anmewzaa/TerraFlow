package hcl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

const rawFile = `# keep this comment
resource "azurerm_public_ip" "gw" {
  name                = "pip-gw"
  location            = var.location
  allocation_method   = "Static"
  tags                = local.common_tags
}

resource "azurerm_network_interface" "nic" {
  name     = "nic-1"
  location = var.location

  ip_configuration {
    name                          = "internal"
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.gw.id
  }

  ip_configuration {
    name                          = "second"
    private_ip_address_allocation = "Dynamic"
  }
}

resource "azurerm_thing" "skipped" {
  dynamic "rule" {
    for_each = var.rules
    content { name = rule.value }
  }
}
`

func rawRepo(t *testing.T) (string, []hcl.GeneratedFile) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "net"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "net", "main.tf"), []byte(rawFile), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, []hcl.GeneratedFile{{Folder: "net", Filename: "main.tf", Content: rawFile}}
}

func TestResource_NewBlockIsAppendedAndKeepsExistingBytes(t *testing.T) {
	_, repo := rawRepo(t)
	node := hcl.GenerateNode{
		ID: "n1", Kind: "resource", ResourceType: "azurerm_resource_group", InstanceName: "rg",
		TargetFolder: "net", Filename: "main.tf",
		Values: map[string]interface{}{
			"name": "rg-test", "location": "southeastasia",
			"tags": map[string]interface{}{"env": "dev"},
			"timeouts": map[string]interface{}{"$blocks": []interface{}{
				map[string]interface{}{"create": "30m"},
			}},
		},
	}
	out, err := hcl.Apply(hcl.GenerateInput{Nodes: []hcl.GenerateNode{node}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].Content
	if !strings.HasPrefix(got, rawFile) {
		t.Fatal("existing bytes changed")
	}
	for _, want := range []string{`resource "azurerm_resource_group" "rg" {`, "timeouts {", `create`, `"30m"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	parses(t, "main.tf", got)
}

func TestResource_DuplicateNameIsRefusedButSameNameOtherTypeIsFine(t *testing.T) {
	_, repo := rawRepo(t)
	mk := func(rtype string) hcl.GenerateInput {
		return hcl.GenerateInput{Nodes: []hcl.GenerateNode{{
			ID: "x", Kind: "resource", ResourceType: rtype, InstanceName: "gw", TargetFolder: "net",
			Values: map[string]interface{}{"name": "x"},
		}}}
	}
	if _, err := hcl.Apply(mk("azurerm_public_ip"), repo); err == nil {
		t.Fatal("want a duplicate error")
	}
	if _, err := hcl.Apply(mk("azurerm_lb"), repo); err != nil {
		t.Fatalf("same name, other type must be allowed: %v", err)
	}
}

func TestResource_RejectsBadTypeAndName(t *testing.T) {
	for _, n := range []hcl.GenerateNode{
		{Kind: "resource", ResourceType: "Module", InstanceName: "a", TargetFolder: "net"},
		{Kind: "resource", ResourceType: "azurerm_x", InstanceName: "bad name", TargetFolder: "net"},
		{Kind: "resource", ResourceType: "azurerm_x", InstanceName: "a", TargetFolder: "net", ParentID: "p"},
	} {
		if _, err := hcl.Generate(hcl.GenerateInput{Nodes: []hcl.GenerateNode{n}}); err == nil {
			t.Errorf("want an error for %+v", n)
		}
	}
}

// Import every resource, apply them unchanged, import again: nothing about the
// resources may change, nested blocks and references included.
func TestResource_ImportApplyRoundTrip(t *testing.T) {
	dir, repo := rawRepo(t)
	first, err := hcl.ImportModules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("want 2 importable resources (the one with a dynamic block is left alone), got %d", len(first))
	}
	out, err := hcl.Apply(hcl.GenerateInput{Nodes: first}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out[0].Content, "# keep this comment") || !strings.Contains(out[0].Content, `dynamic "rule"`) {
		t.Fatal("bytes outside the edited blocks changed")
	}
	if err := os.WriteFile(filepath.Join(dir, "net", "main.tf"), []byte(out[0].Content), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := hcl.ImportModules(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("resources changed in the round trip:\n%s\n%s", a, b)
	}
}

func TestResource_EditCannotChangeType(t *testing.T) {
	_, repo := rawRepo(t)
	n := hcl.GenerateNode{
		ID: "e", Kind: "resource", ResourceType: "azurerm_lb", InstanceName: "gw", TargetFolder: "net",
		Existing: &hcl.ExistingRef{Folder: "net", Filename: "main.tf", Label: "azurerm_public_ip.gw"},
	}
	if _, err := hcl.Apply(hcl.GenerateInput{Nodes: []hcl.GenerateNode{n}}, repo); err == nil {
		t.Fatal("want an error when the type changes")
	}
}
