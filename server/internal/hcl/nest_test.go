package hcl

import (
	"strings"
	"testing"
)

func vnetWithSubnet() []GenerateNode {
	return []GenerateNode{
		{ID: "v", ModuleName: "virtual-network", ModuleVersion: "v1", InstanceName: "vnet_main", TargetFolder: "vnet",
			Source: "src//vnet", Values: map[string]interface{}{"name": "vnet-main", "address_space": []interface{}{"10.0.0.0/16"}}},
		{ID: "s", ModuleName: "subnet", InstanceName: "subnet_pe", TargetFolder: "vnet", ParentID: "v", NestField: "subnets",
			Values: map[string]interface{}{"name": "snet-pe", "address_prefixes": []interface{}{"10.0.1.0/24"}}},
		{ID: "p", ModuleName: "private-endpoint", ModuleVersion: "v1", InstanceName: "pe_kv", TargetFolder: "vnet", ParentID: "s",
			Source: "src//pe", Values: map[string]interface{}{
				"name":      "pe-kv",
				"subnet_id": map[string]interface{}{"$expr": `module.{module}.subnet_ids["{parent_name}"]`},
			}},
	}
}

func TestNestedSubnetFoldsIntoParentModule(t *testing.T) {
	out, err := Generate(GenerateInput{Nodes: vnetWithSubnet()})
	if err != nil {
		t.Fatal(err)
	}
	var main string
	for _, f := range out.Files {
		if f.Filename == "main.tf" {
			main = f.Content
		}
	}
	if strings.Count(main, "module ") != 2 {
		t.Fatalf("want 2 module calls (vnet + private endpoint), got:\n%s", main)
	}
	if strings.Contains(main, `module "subnet_pe"`) {
		t.Fatalf("subnet must not be its own module:\n%s", main)
	}
	for _, want := range []string{`"snet-pe" = {`, `address_prefixes`, `subnets`} {
		if !strings.Contains(main, want) {
			t.Errorf("missing %q in:\n%s", want, main)
		}
	}
	// $expr is written unquoted, with placeholders filled from the ancestry.
	if !strings.Contains(main, `subnet_id`) || !strings.Contains(main, `= module.vnet_main.subnet_ids["snet-pe"]`) {
		t.Errorf("expression not resolved:\n%s", main)
	}
}

func TestNestingDoesNotMutateInputAndRejectsOrphans(t *testing.T) {
	in := vnetWithSubnet()
	if _, err := Generate(GenerateInput{Nodes: in}); err != nil {
		t.Fatal(err)
	}
	if _, has := in[0].Values["subnets"]; has {
		t.Fatal("input node was mutated")
	}
	if _, ok := in[2].Values["subnet_id"].(map[string]interface{}); !ok {
		t.Fatal("input $expr was replaced in place")
	}

	orphan := []GenerateNode{{ID: "s", InstanceName: "subnet_x", ParentID: "gone", NestField: "subnets",
		Values: map[string]interface{}{"name": "x"}}}
	if _, err := Generate(GenerateInput{Nodes: orphan}); err == nil {
		t.Fatal("orphan nested node must be an error")
	}

	dup := append(vnetWithSubnet(), GenerateNode{ID: "s2", InstanceName: "subnet_dup", ParentID: "v", NestField: "subnets",
		Values: map[string]interface{}{"name": "snet-pe"}})
	if _, err := Generate(GenerateInput{Nodes: dup}); err == nil {
		t.Fatal("duplicate subnet name must be an error")
	}
}

func TestPerResourceFilenames(t *testing.T) {
	nodes := []GenerateNode{
		{ID: "a", InstanceName: "rg_a", TargetFolder: "common", Filename: "rg.tf", Source: "s", Values: map[string]interface{}{"name": "rg-a"}},
		{ID: "b", InstanceName: "rg_b", TargetFolder: "common", Filename: "rg.tf", Source: "s", Values: map[string]interface{}{"name": "rg-b"}},
		{ID: "c", InstanceName: "id_c", TargetFolder: "common", Source: "s", Values: map[string]interface{}{"name": "id-c"}},
		{ID: "d", InstanceName: "kv_d", TargetFolder: "keyvault", Filename: "kv.tf", Source: "s", Values: map[string]interface{}{"name": "kv-d"},
			PickRefs: []PickRef{{Field: "resource_group_name", DataType: "azurerm_resource_group", DataName: "rg", Output: "name",
				LookupArgs: map[string]string{"name": `"rg-a"`}}}},
	}
	out, err := Generate(GenerateInput{Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range out.Files {
		got[f.Folder+"/"+f.Filename] = f.Content
	}
	for _, want := range []string{"common/rg.tf", "common/main.tf", "keyvault/kv.tf", "keyvault/data.tf"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("missing %s; have %v", want, keys(got))
		}
	}
	if strings.Count(got["common/rg.tf"], "module ") != 2 || strings.Contains(got["common/main.tf"], "rg_a") {
		t.Fatalf("two resources sharing a file name must land in one file:\n%s", got["common/rg.tf"])
	}
}

func TestFilenameRulesAndDataFileMerge(t *testing.T) {
	for _, bad := range []string{"../x.tf", "a/b.tf", "x.txt", "main", ".tf", "a b.tf", "x.tf.sh"} {
		_, err := Generate(GenerateInput{Nodes: []GenerateNode{{ID: "a", InstanceName: "a", TargetFolder: "f", Filename: bad, Source: "s"}}})
		if err == nil {
			t.Errorf("file name %q must be rejected", bad)
		}
	}
	if _, err := Generate(GenerateInput{Nodes: []GenerateNode{{ID: "a", InstanceName: "a", TargetFolder: "../etc", Source: "s"}}}); err == nil {
		t.Error("folder with .. must be rejected")
	}

	// A resource that chose data.tf shares the file with the generated data blocks.
	n := GenerateNode{ID: "a", InstanceName: "a", TargetFolder: "f", Filename: "data.tf", Source: "s",
		Values:   map[string]interface{}{"name": "x"},
		PickRefs: []PickRef{{Field: "g", DataType: "azurerm_resource_group", DataName: "rg", Output: "name", LookupArgs: map[string]string{"name": `"n"`}}}}
	out, err := Generate(GenerateInput{Nodes: []GenerateNode{n}})
	if err != nil || len(out.Files) != 1 || !strings.Contains(out.Files[0].Content, "module ") || !strings.Contains(out.Files[0].Content, "data ") {
		t.Fatalf("data.tf must hold both: %v %+v", err, out)
	}
}

func keys(m map[string]string) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
