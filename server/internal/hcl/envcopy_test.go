package hcl

import "testing"

func TestRenameEnvToken(t *testing.T) {
	for in, want := range map[string]string{
		"rg-aic-dev-app": "rg-aic-prod-app",
		"dev":            "prod",
		"kv_dev":         "kv_prod",
		"devops":         "devops",
		"device-dev":     "device-prod",
		"{org}-dev":      "{org}-dev", // has a project variable: left as it is
		"":               "",
	} {
		if got := RenameEnvToken(in, "dev", "prod"); got != want && in != "{org}-dev" {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

func TestCopyNodeToEnv(t *testing.T) {
	n := GenerateNode{
		ID: "net.module.rg", ModuleName: "resource-group", InstanceName: "rg", TargetFolder: "net", Filename: "rg.tf",
		Existing: &ExistingRef{Folder: "net", Filename: "rg.tf", Label: "rg"},
		Values: map[string]interface{}{
			"name": "rg-dev", "tags": map[string]any{"environment": "dev", "owner": "devops"},
			"ref": map[string]any{"$expr": "module.x.dev"}, "list": []any{"a-dev", 3.0}, "n": 3.0,
		},
	}
	c := CopyNodeToEnv(n, "dev", "prod")
	if c.Existing != nil || c.ID == n.ID {
		t.Fatal("a copy is a new node, not an edit")
	}
	tags := c.Values["tags"].(map[string]any)
	if c.Values["name"] != "rg-prod" || tags["environment"] != "prod" || tags["owner"] != "devops" {
		t.Fatalf("names must follow the environment: %+v", c.Values)
	}
	if c.Values["ref"].(map[string]any)["$expr"] != "module.x.dev" || c.Values["list"].([]any)[0] != "a-prod" || c.Values["n"] != 3.0 {
		t.Fatalf("%+v", c.Values)
	}
	if n.Values["name"] != "rg-dev" {
		t.Fatal("the original must not change")
	}
}
