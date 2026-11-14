package validate

import (
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func policyFindings(fs []Finding) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Rule == "policy" || f.Rule == "check" {
			out = append(out, f)
		}
	}
	return out
}

func TestPolicyRulesAreWarningsUnlessStrict(t *testing.T) {
	cat := seedCatalog(t)
	rg := node("rg", "resource-group", map[string]interface{}{"name": "badname", "location": "japaneast", "tags": map[string]interface{}{"owner": "x"}})
	c := store.CanvasState{Nodes: []store.CanvasNode{rg}}
	checks := &hcl.Checks{Naming: map[string]string{"resource-group": `^rg-[a-z0-9-]+$`}, RequiredTags: []string{"owner", "project"}, AllowedRegions: []string{"southeastasia"}}

	got := policyFindings(CanvasWithPolicy(c, cat, &hcl.ProjectSettings{Checks: checks}, ""))
	if len(got) != 3 {
		t.Fatalf("naming, region and tags: %+v", got)
	}
	for _, f := range got {
		if f.Severity != Warning {
			t.Errorf("%s must be a warning: %+v", f.Field, f)
		}
	}
	if HasErrors(CanvasWithPolicy(c, cat, &hcl.ProjectSettings{Checks: checks}, "")) {
		t.Fatal("warnings do not stop publish")
	}
	checks.Strict = true
	if !HasErrors(CanvasWithPolicy(c, cat, &hcl.ProjectSettings{Checks: checks}, "")) {
		t.Fatal("strict findings are errors")
	}
	// The same canvas always gives the same list.
	a, b := CanvasWithPolicy(c, cat, &hcl.ProjectSettings{Checks: checks}, ""), CanvasWithPolicy(c, cat, &hcl.ProjectSettings{Checks: checks}, "")
	if strings.Join(rules(a), ",") != strings.Join(rules(b), ",") {
		t.Fatal("findings must come in a fixed order")
	}
}

func TestPolicyDoesNotJudgeWhatItCannotSee(t *testing.T) {
	cat := seedCatalog(t)
	rg := node("rg", "resource-group", map[string]interface{}{
		"name": "{org}-app", "location": map[string]interface{}{"$expr": "var.region"}, "tags": map[string]interface{}{"$expr": "local.tags"},
	})
	checks := &hcl.Checks{Naming: map[string]string{"resource-group": `^rg-`}, RequiredTags: []string{"owner"}, AllowedRegions: []string{"southeastasia"}, Strict: true}
	if got := policyFindings(CanvasWithPolicy(store.CanvasState{Nodes: []store.CanvasNode{rg}}, cat, &hcl.ProjectSettings{Checks: checks}, "")); len(got) != 0 {
		t.Fatalf("references and project variables are unknown, not failed: %+v", got)
	}
	ok := node("rg", "resource-group", map[string]interface{}{"name": "rg-app", "location": "SoutheastAsia", "tags": map[string]interface{}{"owner": "a"}})
	if got := policyFindings(CanvasWithPolicy(store.CanvasState{Nodes: []store.CanvasNode{ok}}, cat, &hcl.ProjectSettings{Checks: checks}, "")); len(got) != 0 {
		t.Fatalf("a compliant resource has no findings (region compared without case): %+v", got)
	}
}

func TestAnExistingBlockIsOnlyEverWarned(t *testing.T) {
	cat := seedCatalog(t)
	rg := node("rg", "resource-group", map[string]interface{}{"name": "legacy", "location": "southeastasia"})
	rg.Existing = &store.ExistingRef{Folder: "common", Filename: "main.tf", Label: "rg"}
	checks := &hcl.Checks{Naming: map[string]string{"resource-group": `^rg-`}, Strict: true}
	for _, f := range policyFindings(CanvasWithPolicy(store.CanvasState{Nodes: []store.CanvasNode{rg}}, cat, &hcl.ProjectSettings{Checks: checks}, "")) {
		if f.Severity != Warning {
			t.Fatalf("an existing block must not be blocked: %+v", f)
		}
	}
}

func TestSchemaChecksCarriedByAModule(t *testing.T) {
	schema := `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"},"c":{"type":"string"}},
	  "x-tf-checks":[{"rule":"required_with","field":"a","with":"b","message":"a needs b"},{"rule":"exclusive","fields":["b","c"]}]}`
	n := store.CanvasNode{ID: "n", Values: map[string]interface{}{"a": "x"}}
	var got []Finding
	schemaChecks(n, []byte(schema), func(f Finding) { got = append(got, f) })
	if len(got) != 1 || got[0].Message != "a needs b" {
		t.Fatalf("required_with: %+v", got)
	}
	got = nil
	n.Values = map[string]interface{}{"a": "x", "b": map[string]interface{}{"$expr": "var.b"}, "c": "y"}
	schemaChecks(n, []byte(schema), func(f Finding) { got = append(got, f) })
	if len(got) != 1 || got[0].Field != "c" {
		t.Fatalf("a reference counts as set, never as missing; both set is exclusive: %+v", got)
	}
}

func TestASecretIsNeverTypedAsText(t *testing.T) {
	cat := seedCatalog(t)
	pg := node("pg", "postgresql-flexible-server", map[string]interface{}{"name": "pg-a", "resource_group_name": "rg", "administrator_password": "hunter2"})
	if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{pg}}, cat); !has(fs, "pg", "secret") {
		t.Fatalf("a typed password must be refused: %v", rules(fs))
	}
	pg.Values["administrator_password"] = map[string]interface{}{"$expr": "var.pg_password"}
	if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{pg}}, cat); has(fs, "pg", "secret") {
		t.Fatalf("a reference is the way to give a secret: %v", rules(fs))
	}
}

func TestASubnetIsLinkedThroughItsVirtualNetwork(t *testing.T) {
	cat := seedCatalog(t)
	vnet := node("vnet", "virtual-network", map[string]interface{}{"name": "vnet-a", "resource_group_name": "rg", "address_space": []interface{}{"10.0.0.0/16"}})
	sn := node("sn", "subnet", map[string]interface{}{"name": "snet-app", "address_prefixes": []interface{}{"10.0.1.0/24"}})
	sn.ParentID = "vnet"
	assoc := func(target string) store.CanvasNode {
		return node("assoc", "subnet-network-security-group-association", map[string]interface{}{
			"subnet_id":                 map[string]interface{}{"$expr": "module." + target + `.subnet_ids["snet-app"]`},
			"network_security_group_id": map[string]interface{}{"$expr": "module.nsg.id"},
		})
	}
	if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{vnet, sn, assoc("vnet")}}, cat); has(fs, "assoc", "link") {
		t.Fatalf("the virtual network that holds a subnet is a valid target: %v", rules(fs))
	}
	rg := node("rg2", "resource-group", map[string]interface{}{"name": "rg-b", "location": "southeastasia"})
	rg.InstanceName = "rgx"
	if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{vnet, sn, rg, assoc("rgx")}}, cat); !has(fs, "assoc", "link") {
		t.Fatalf("a resource group holds no subnet: %v", rules(fs))
	}
}

func TestEnvironmentRulesAndVariablesWithoutAValue(t *testing.T) {
	cat := seedCatalog(t)
	st := &hcl.ProjectSettings{
		Variables: []hcl.ProjectVariable{{Name: "org", Values: map[string]string{"dev": "aic"}}},
		Checks:    &hcl.Checks{RequiredInEnv: []hcl.EnvRequirement{{Env: "prod", Module: "resource-group", Field: "tags"}}},
	}
	rg := node("rg", "resource-group", map[string]interface{}{"name": "rg-{org}-app", "location": "southeastasia"})
	c := store.CanvasState{Nodes: []store.CanvasNode{rg}}
	if got := policyFindings(CanvasWithPolicy(c, cat, st, "dev")); len(got) != 0 {
		t.Fatalf("dev has the value and no requirement: %+v", got)
	}
	got := policyFindings(CanvasWithPolicy(c, cat, st, "prod"))
	if len(got) != 2 {
		t.Fatalf("prod: the variable has no value there, and tags are required: %+v", got)
	}
	// Without an environment (an older client) nothing about environments is judged.
	if got := policyFindings(CanvasWithPolicy(c, cat, st, "")); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestARequiredInputSuppliedByNestedResourcesIsNotMissing(t *testing.T) {
	cat := seedCatalog(t)
	vnet := node("vnet", "virtual-network", map[string]interface{}{"name": "vnet-a", "resource_group_name": "rg", "address_space": []interface{}{"10.0.0.0/16"}})
	if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{vnet}}, cat); !has(fs, "vnet", "required") {
		t.Fatalf("a virtual network with no subnets is incomplete: %v", rules(fs))
	}
	sn := node("sn", "subnet", map[string]interface{}{"name": "snet-app", "address_prefixes": []interface{}{"10.0.1.0/24"}})
	sn.ParentID, sn.NestField = "vnet", "subnets"
	for _, f := range Canvas(store.CanvasState{Nodes: []store.CanvasNode{vnet, sn}}, cat) {
		if f.Node == "vnet" && f.Rule == "required" {
			t.Fatalf("subnets are supplied by the subnet placed inside: %+v", f)
		}
	}
}
