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

	got := policyFindings(CanvasWithPolicy(c, cat, checks))
	if len(got) != 3 {
		t.Fatalf("naming, region and tags: %+v", got)
	}
	for _, f := range got {
		if f.Severity != Warning {
			t.Errorf("%s must be a warning: %+v", f.Field, f)
		}
	}
	if HasErrors(CanvasWithPolicy(c, cat, checks)) {
		t.Fatal("warnings do not stop publish")
	}
	checks.Strict = true
	if !HasErrors(CanvasWithPolicy(c, cat, checks)) {
		t.Fatal("strict findings are errors")
	}
	// The same canvas always gives the same list.
	a, b := CanvasWithPolicy(c, cat, checks), CanvasWithPolicy(c, cat, checks)
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
	if got := policyFindings(CanvasWithPolicy(store.CanvasState{Nodes: []store.CanvasNode{rg}}, cat, checks)); len(got) != 0 {
		t.Fatalf("references and project variables are unknown, not failed: %+v", got)
	}
	ok := node("rg", "resource-group", map[string]interface{}{"name": "rg-app", "location": "SoutheastAsia", "tags": map[string]interface{}{"owner": "a"}})
	if got := policyFindings(CanvasWithPolicy(store.CanvasState{Nodes: []store.CanvasNode{ok}}, cat, checks)); len(got) != 0 {
		t.Fatalf("a compliant resource has no findings (region compared without case): %+v", got)
	}
}

func TestAnExistingBlockIsOnlyEverWarned(t *testing.T) {
	cat := seedCatalog(t)
	rg := node("rg", "resource-group", map[string]interface{}{"name": "legacy", "location": "southeastasia"})
	rg.Existing = &store.ExistingRef{Folder: "common", Filename: "main.tf", Label: "rg"}
	checks := &hcl.Checks{Naming: map[string]string{"resource-group": `^rg-`}, Strict: true}
	for _, f := range policyFindings(CanvasWithPolicy(store.CanvasState{Nodes: []store.CanvasNode{rg}}, cat, checks)) {
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
