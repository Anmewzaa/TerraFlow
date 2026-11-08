package hcl

import (
	"strings"
	"testing"
)

func TestGenerateSimpleModuleCall(t *testing.T) {
	input := GenerateInput{
		Nodes: []GenerateNode{
			{
				ID:            "node-1",
				ModuleName:    "resource-group",
				ModuleVersion: "v1.2.0",
				InstanceName:  "rg_network",
				TargetFolder:  "common",
				Source:        "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0",
				Values: map[string]interface{}{
					"name":     "rg-proj1-dev-network",
					"location": "southeastasia",
					"tags":     map[string]interface{}{},
				},
			},
		},
	}

	out, err := Generate(input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(out.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(out.Files))
	}

	f := out.Files[0]
	if f.Folder != "common" {
		t.Errorf("folder = %q, want common", f.Folder)
	}
	if f.Filename != "main.tf" {
		t.Errorf("filename = %q, want main.tf", f.Filename)
	}

	want := `module "rg_network" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  location                 = "southeastasia"
  name                     = "rg-proj1-dev-network"
  tags                     = {}
}
`
	if f.Content != want {
		t.Errorf("content mismatch:\n--- got ---\n%s\n--- want ---\n%s", f.Content, want)
	}
}

func TestGenerateWithPickRef(t *testing.T) {
	input := GenerateInput{
		Nodes: []GenerateNode{
			{
				ID:            "node-2",
				ModuleName:    "key-vault",
				ModuleVersion: "v1.4.0",
				InstanceName:  "kv",
				TargetFolder:  "keyvault",
				Source:        "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//key-vault?ref=v1.4.0",
				Values: map[string]interface{}{
					"name":                     "kv-proj1-dev-01",
					"resource_group_name":      "rg-proj1-dev-app",
					"location":                 "southeastasia",
					"sku_name":                 "standard",
					"purge_protection_enabled": true,
				},
				PickRefs: []PickRef{
					{
						Field:      "resource_group_name",
						ModuleType: "resource-group",
						Output:     "name",
						DataName:   "app",
						DataType:   "azurerm_resource_group",
						LookupKey:  "rg-proj1-dev-app",
						LookupArgs: map[string]string{
							"name": "local.rg_names.app",
						},
					},
					{
						Field:      "location",
						ModuleType: "resource-group",
						Output:     "location",
						DataName:   "app",
						DataType:   "azurerm_resource_group",
						LookupArgs: map[string]string{
							"name": "local.rg_names.app",
						},
					},
				},
			},
		},
	}

	out, err := Generate(input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(out.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(out.Files))
	}

	var mainFile, dataFile GeneratedFile
	for _, f := range out.Files {
		switch f.Filename {
		case "main.tf":
			mainFile = f
		case "data.tf":
			dataFile = f
		}
	}

	if !strings.Contains(mainFile.Content, `resource_group_name      = data.azurerm_resource_group.app.name`) {
		t.Errorf("expected pick-ref in module call, got:\n%s", mainFile.Content)
	}

	if !strings.Contains(dataFile.Content, `data "azurerm_resource_group" "app"`) {
		t.Errorf("expected data block, got:\n%s", dataFile.Content)
	}

	if !strings.Contains(dataFile.Content, `name                     = local.rg_names.app`) {
		t.Errorf("expected lookup arg in data block, got:\n%s", dataFile.Content)
	}
}

func TestGenerateMultipleFolders(t *testing.T) {
	input := GenerateInput{
		Nodes: []GenerateNode{
			{
				ID:           "n1",
				InstanceName: "rg_app",
				TargetFolder: "common",
				Source:       "git::https://example.com//resource-group?ref=v1",
				Values:       map[string]interface{}{"name": "rg-app"},
			},
			{
				ID:           "n2",
				InstanceName: "kv",
				TargetFolder: "keyvault",
				Source:       "git::https://example.com//key-vault?ref=v1",
				Values:       map[string]interface{}{"name": "kv-01"},
			},
		},
	}

	out, err := Generate(input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	folders := map[string]bool{}
	for _, f := range out.Files {
		folders[f.Folder] = true
	}
	if !folders["common"] || !folders["keyvault"] {
		t.Errorf("expected files in common and keyvault, got folders: %v", folders)
	}
}
