package hcl

import "testing"

func TestFindUnrepresented(t *testing.T) {
	repo := files(map[string]string{
		"net/main.tf": "module \"plain\" {\n  source = \"x\"\n}\n" +
			"module \"many\" {\n  source = \"x\"\n  for_each = toset([\"a\"])\n}\n" +
			"resource \"azurerm_x\" \"dyn\" {\n  dynamic \"rule\" {\n    for_each = []\n    content {}\n  }\n}\n" +
			"resource \"azurerm_x\" \"counted\" {\n  count = 2\n}\n",
	})
	got, err := FindUnrepresented(repo)
	if err != nil || len(got) != 3 {
		t.Fatalf("%v %+v", err, got)
	}
	for _, it := range got {
		if it.Label == "plain" || it.Key == "" || it.Kind != DriftUnrepresented {
			t.Fatalf("%+v", it)
		}
	}
}

func TestVariableDrift(t *testing.T) {
	g := &Graph{Folders: []Folder{{Name: "net", Nodes: []Node{{
		ID: "net.module.rg", Folder: "net", Kind: KindModule, Label: "rg", Filename: "rg.tf",
		Inputs: []byte(`{"location":"japaneast","name":"x","size":{"$unknown":true},"org":"aic"}`),
	}}}}}
	st := &ProjectSettings{Variables: []ProjectVariable{
		{Name: "location", Values: map[string]string{"dev": "southeastasia"}},
		{Name: "org", Values: map[string]string{"dev": "aic"}},
		{Name: "size", Values: map[string]string{"dev": "S"}},
		{Name: "name", Values: map[string]string{"prod": "p"}},
	}}
	got := VariableDrift(g, st, "dev")
	if len(got) != 1 || got[0].Field != "location" || got[0].NodeID != "net.module.rg" {
		t.Fatalf("only a literal that differs counts (not equal, not unknown, not another environment): %+v", got)
	}
}

func TestDiffManifests(t *testing.T) {
	got := DiffManifests(map[string]string{"a/x.tf": "1", "a/y.tf": "1", "a/z.tf": "1"}, map[string]string{"a/x.tf": "1", "a/y.tf": "2", "b/n.tf": "1"})
	kinds := map[string]string{}
	for _, it := range got {
		kinds[it.File] = it.Kind
	}
	if len(got) != 3 || kinds["y.tf"] != DriftFileChanged || kinds["z.tf"] != DriftFileRemoved || kinds["n.tf"] != DriftFileAdded {
		t.Fatalf("%+v", got)
	}
}
