package hcl

import "testing"

func envRepo(env string, extra bool) map[string]string {
	vals := map[string]string{"dev": "size = \"s\"\n", "prod": "size = \"p\"\nother = \"o\"\n"}
	r := map[string]string{
		"net/main.tf": "variable \"size\" {}\nvariable \"other\" {\n  default = \"d\"\n}\n" +
			"module \"rg\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name   = \"rg-${var.size}\"\n  extra  = var.other\n}\n" +
			"module \"same\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name   = \"same\"\n}\n",
		"net/env/" + env + ".tfvars": vals[env],
	}
	if extra {
		r["net/more.tf"] = "module \"extra\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name = \"e\"\n}\n"
	}
	return r
}

func TestDiffGraphs(t *testing.T) {
	build := func(env string, extra bool) *Graph {
		g, err := BuildGraphFS(files(envRepo(env, extra)), "p", env)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	d := DiffGraphs(build("dev", false), build("prod", true), "dev", "prod", &ProjectSettings{Variables: []ProjectVariable{
		{Name: "org", Values: map[string]string{"dev": "a"}},
		{Name: "both", Values: map[string]string{"dev": "a", "prod": "b"}},
	}})
	by := map[string]ResourceDiff{}
	for _, r := range d.Resources {
		by[r.Label] = r
	}
	if by["same"].Status != DiffSame || by["extra"].Status != DiffOnlyTo {
		t.Fatalf("same / only in prod: %+v", d.Resources)
	}
	rg := by["rg"]
	if rg.Status != DiffDiffers {
		t.Fatalf("rg: %+v", rg)
	}
	seen := map[string]string{}
	for _, a := range rg.Args {
		seen[a.Name] = a.Status
	}
	if seen["name"] != DiffDiffers || seen["extra"] != DiffDiffers {
		t.Fatalf("args: %+v", rg.Args)
	}
	if len(d.Variables) != 1 || d.Variables[0].Name != "org" || d.Variables[0].Has != "from" {
		t.Fatalf("variables: %+v", d.Variables)
	}
	if d.Summary.Same != 1 || d.Summary.Differs != 1 || d.Summary.OnlyTo != 1 {
		t.Fatalf("summary: %+v", d.Summary)
	}
}

func TestAnUnresolvedValueIsNeverEqual(t *testing.T) {
	g := func(v string) *Graph {
		return &Graph{Folders: []Folder{{Name: "n", Nodes: []Node{{ID: "n.module.a", Folder: "n", Kind: KindModule, Label: "a", Inputs: []byte(v)}}}}}
	}
	d := DiffGraphs(g(`{"x":{"$unknown":true}}`), g(`{"x":{"$unknown":true}}`), "dev", "prod", nil)
	if d.Resources[0].Status != DiffUnknown || d.Summary.Same != 0 || d.Summary.Unknown != 1 {
		t.Fatalf("unknown on both sides is not equal: %+v", d)
	}
	nested := DiffGraphs(g(`{"x":[{"y":{"$unknown":true}}]}`), g(`{"x":[1]}`), "dev", "prod", nil)
	if nested.Resources[0].Status != DiffUnknown {
		t.Fatalf("nested unknown: %+v", nested.Resources[0])
	}
	same := DiffGraphs(g(`{"a":1,"b":{"c":2,"d":3}}`), g(`{"b":{"d":3,"c":2},"a":1}`), "dev", "prod", nil)
	if same.Resources[0].Status != DiffSame {
		t.Fatalf("key order is not a difference: %+v", same.Resources[0])
	}
}
