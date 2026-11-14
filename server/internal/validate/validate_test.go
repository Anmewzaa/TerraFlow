package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// The real catalog seeds, so the rules are tried against the schemas people actually use.
func seedCatalog(t *testing.T) MapCatalog {
	t.Helper()
	dirs, err := filepath.Glob("../../../catalog/modules/*/v1/schema.json")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no seed schemas found: %v", err)
	}
	var mods []Module
	for _, p := range dirs {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		mods = append(mods, Module{Name: filepath.Base(filepath.Dir(filepath.Dir(p))), Version: "v1", Schema: raw})
	}
	return NewMapCatalog(mods)
}

func node(id, module string, values map[string]interface{}) store.CanvasNode {
	return store.CanvasNode{ID: id, ModuleName: module, ModuleVersion: "v1", InstanceName: strings.ReplaceAll(id, "-", "_"),
		TargetFolder: "common", Filename: "main.tf", Values: values}
}

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Node+":"+f.Rule+":"+f.Field)
	}
	return out
}

func has(fs []Finding, node, rule string) bool {
	for _, f := range fs {
		if f.Node == node && f.Rule == rule {
			return true
		}
	}
	return false
}

func TestAValidResourceGroupHasNoFindings(t *testing.T) {
	cat := seedCatalog(t)
	c := store.CanvasState{Nodes: []store.CanvasNode{node("rg", "resource-group", map[string]interface{}{
		"name": "rg-app", "location": "southeastasia", "lock_level": "CanNotDelete"})}}
	if fs := Canvas(c, cat); len(fs) != 0 {
		t.Fatalf("unexpected findings: %v", rules(fs))
	}
}

func TestRequiredEmptyAndUnknownModule(t *testing.T) {
	cat := seedCatalog(t)
	c := store.CanvasState{Nodes: []store.CanvasNode{
		node("rg", "resource-group", map[string]interface{}{"name": "  "}),
		node("x", "does-not-exist", nil),
	}}
	fs := Canvas(c, cat)
	if !has(fs, "rg", "required") || !has(fs, "x", "module") {
		t.Fatalf("got %v", rules(fs))
	}
	if HasStructural(fs) || !HasErrors(fs) {
		t.Fatal("incomplete work is an error but not a structural refusal")
	}
}

func TestValuesMustFitTheirSchemaButReferencesAreNotJudged(t *testing.T) {
	cat := seedCatalog(t)
	bad := node("rg", "resource-group", map[string]interface{}{"name": 5.0})
	fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{bad}}, cat)
	if !has(fs, "rg", "schema") {
		t.Fatalf("an enum violation must be found: %v", rules(fs))
	}
	// The same field as a reference or a project variable is unknown, never failed (rule 8).
	for _, v := range []interface{}{map[string]interface{}{"$expr": "var.rg_name"}, "{rg_name}"} {
		ok := node("rg", "resource-group", map[string]interface{}{"name": v, "location": "southeastasia"})
		if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{ok}}, cat); len(fs) != 0 {
			t.Errorf("%v must not be judged: %v", v, rules(fs))
		}
	}
}

func TestContainmentRules(t *testing.T) {
	cat := seedCatalog(t)
	vnet := node("vnet", "virtual-network", map[string]interface{}{"name": "vnet-a", "resource_group_name": "rg", "location": "southeastasia", "address_space": []interface{}{"10.0.0.0/16"}})
	pe := node("pe", "private-endpoint", map[string]interface{}{"name": "pe-a", "resource_group_name": "rg", "location": "southeastasia", "private_connection_resource_id": "x", "subresource_names": []interface{}{"vault"}})

	// A private endpoint must sit in a subnet.
	fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{pe}}, cat)
	if !has(fs, "pe", "parent") {
		t.Fatalf("missing parent not found: %v", rules(fs))
	}
	// In the wrong kind of parent.
	pe.ParentID = "vnet"
	fs = Canvas(store.CanvasState{Nodes: []store.CanvasNode{vnet, pe}}, cat)
	if !has(fs, "pe", "parent") {
		t.Fatalf("wrong parent not found: %v", rules(fs))
	}
	// A parent that is not on the canvas, and a circle.
	pe.ParentID = "ghost"
	if fs = Canvas(store.CanvasState{Nodes: []store.CanvasNode{pe}}, cat); !has(fs, "pe", "parent") {
		t.Fatalf("absent parent not found: %v", rules(fs))
	}
	a, b := node("a", "resource-group", nil), node("b", "resource-group", nil)
	a.ParentID, b.ParentID = "b", "a"
	fs = Canvas(store.CanvasState{Nodes: []store.CanvasNode{a, b}}, cat)
	if !has(fs, "a", "parent") || !has(fs, "b", "parent") {
		t.Fatalf("a circle must be found: %v", rules(fs))
	}
}

func TestDuplicateNamesAndDanglingArrows(t *testing.T) {
	cat := seedCatalog(t)
	one := node("one", "resource-group", map[string]interface{}{"name": "a", "location": "southeastasia"})
	two := node("two", "resource-group", map[string]interface{}{"name": "b", "location": "southeastasia"})
	two.InstanceName = one.InstanceName
	c := store.CanvasState{Nodes: []store.CanvasNode{one, two}, Edges: []store.CanvasEdge{{ID: "e", From: "one", To: "nowhere", Kind: "x"}}}
	fs := Canvas(c, cat)
	if !has(fs, "two", "duplicate") {
		t.Fatalf("duplicate not found: %v", rules(fs))
	}
	found := false
	for _, f := range fs {
		found = found || f.Rule == "edge"
	}
	if !found {
		t.Fatalf("dangling arrow not found: %v", rules(fs))
	}
}

func TestHostileInputIsRefusedStructurally(t *testing.T) {
	cases := map[string]store.CanvasNode{
		"path traversal":      {ID: "a", InstanceName: "x", TargetFolder: "../etc"},
		"windows path":        {ID: "a", InstanceName: "x", TargetFolder: `c:\x`},
		"bad file name":       {ID: "a", InstanceName: "x", TargetFolder: "f", Filename: "../x.tf"},
		"bad block name":      {ID: "a", InstanceName: `x" {}` + "\n" + `module "evil`},
		"expr closes block":   {ID: "a", InstanceName: "x", Values: map[string]interface{}{"name": map[string]interface{}{"$expr": "var.a }\nmodule \"evil\" {"}}},
		"two expressions":     {ID: "a", InstanceName: "x", Values: map[string]interface{}{"name": map[string]interface{}{"$expr": "var.a var.b"}}},
		"empty expression":    {ID: "a", InstanceName: "x", Values: map[string]interface{}{"name": map[string]interface{}{"$expr": " "}}},
		"expression not text": {ID: "a", InstanceName: "x", Values: map[string]interface{}{"name": map[string]interface{}{"$expr": 5}}},
		"huge string":         {ID: "a", InstanceName: "x", Values: map[string]interface{}{"name": strings.Repeat("a", MaxStringBytes+1)}},
		"no id":               {InstanceName: "x"},
		"bad resource type":   {ID: "a", InstanceName: "x", Kind: "resource", ResourceType: "Evil Type"},
	}
	for name, n := range cases {
		fs := Structural(store.CanvasState{Nodes: []store.CanvasNode{n}})
		if !HasStructural(fs) {
			t.Errorf("%s: not refused: %v", name, fs)
		}
	}
	// Too deep.
	deep := map[string]interface{}{}
	cur := deep
	for i := 0; i < MaxValueDepth+3; i++ {
		next := map[string]interface{}{}
		cur["k"] = next
		cur = next
	}
	if fs := Structural(store.CanvasState{Nodes: []store.CanvasNode{{ID: "a", InstanceName: "x", Values: deep}}}); !HasStructural(fs) {
		t.Error("a very deep value must be refused")
	}
}

func TestSizeLimitsStopEarly(t *testing.T) {
	var many store.CanvasState
	for i := 0; i < MaxNodes+1; i++ {
		many.Nodes = append(many.Nodes, store.CanvasNode{ID: fmt.Sprintf("n%d", i), InstanceName: "x"})
	}
	fs := Canvas(many, nil)
	if len(fs) != 1 || fs[0].Rule != "limit" || !fs[0].Structural {
		t.Fatalf("an oversized canvas must stop at one finding: %v", fs)
	}
	var edges store.CanvasState
	for i := 0; i < MaxEdges+1; i++ {
		edges.Edges = append(edges.Edges, store.CanvasEdge{ID: fmt.Sprint(i)})
	}
	if fs := Structural(edges); !HasStructural(fs) {
		t.Fatal("too many arrows must be refused")
	}
}

func TestRealWorldValuesPass(t *testing.T) {
	// Expressions the generator and importer really produce must not be refused.
	for _, e := range []string{`var.location`, `local.rg_names.app`, `module.rg.name`, `var.environment == "prod" ? "CanNotDelete" : ""`,
		`merge(local.common_tags, { data_classification = "confidential" })`, `module.vnet.subnet_ids["app"]`, `"${var.org}-rg"`} {
		n := store.CanvasNode{ID: "a", InstanceName: "x", Values: map[string]interface{}{"v": map[string]interface{}{"$expr": e}}}
		if fs := Structural(store.CanvasState{Nodes: []store.CanvasNode{n}}); len(fs) != 0 {
			t.Errorf("%q refused: %v", e, fs)
		}
	}
}

func TestFindingsAreDeterministic(t *testing.T) {
	cat := seedCatalog(t)
	c := store.CanvasState{Nodes: []store.CanvasNode{
		node("b", "resource-group", nil), node("a", "key-vault", nil), node("c", "does-not-exist", nil),
	}}
	first, _ := json.Marshal(Canvas(c, cat))
	for i := 0; i < 20; i++ {
		again, _ := json.Marshal(Canvas(c, cat))
		if string(again) != string(first) {
			t.Fatal("the same canvas gave a different answer")
		}
	}
}

func TestParseRefusesBadShape(t *testing.T) {
	if _, err := Parse([]byte(`{"nodes": "no"}`)); err == nil {
		t.Error("a canvas whose nodes are not a list must be refused")
	}
	if _, err := Parse([]byte(`{"nodes":[],"edges":[]}`)); err != nil {
		t.Error(err)
	}
}

func FuzzValuesNeverPanic(f *testing.F) {
	f.Add(`{"a":{"$expr":"var.x"},"b":[1,"{x}",null],"c":{"d":{"e":true}}}`)
	f.Add(`{"$expr":"}"}`)
	cat := NewMapCatalog(nil)
	f.Fuzz(func(t *testing.T, raw string) {
		var v map[string]interface{}
		if json.Unmarshal([]byte(raw), &v) != nil {
			return
		}
		_ = Canvas(store.CanvasState{Nodes: []store.CanvasNode{{ID: "a", InstanceName: "x", ModuleName: "m", ModuleVersion: "v1", Values: v}}}, cat)
	})
}

func TestALinkOnlyReachesAResourceThatOffersIt(t *testing.T) {
	cat := seedCatalog(t)
	kv := node("kv", "key-vault", map[string]interface{}{"name": "kv-a"})
	rg := node("rg", "resource-group", map[string]interface{}{"name": "rg-a", "location": "southeastasia"})
	pe := func(target string) store.CanvasNode {
		n := node("pe", "private-endpoint", map[string]interface{}{
			"name": "pe-a", "resource_group_name": "rg", "location": "southeastasia",
			"private_connection_resource_id": map[string]interface{}{"$expr": "module." + target + ".id"},
			"subresource_names":              []interface{}{"vault"},
		})
		n.ParentID = "sn"
		return n
	}
	sn := node("sn", "subnet", map[string]interface{}{"name": "sn-a"})
	sn.ParentID = ""
	// A key vault offers a private link; a resource group does not.
	ok := Canvas(store.CanvasState{Nodes: []store.CanvasNode{kv, rg, sn, pe("kv")}}, cat)
	if has(ok, "pe", "link") {
		t.Fatalf("a key vault is a valid target: %v", rules(ok))
	}
	bad := Canvas(store.CanvasState{Nodes: []store.CanvasNode{kv, rg, sn, pe("rg")}}, cat)
	if !has(bad, "pe", "link") {
		t.Fatalf("a resource group must be refused as a target: %v", rules(bad))
	}
	// A target in the repository (not on the canvas) cannot be judged, so it is not refused.
	if unknown := Canvas(store.CanvasState{Nodes: []store.CanvasNode{sn, pe("elsewhere")}}, cat); has(unknown, "pe", "link") {
		t.Fatalf("an unknown target is not judged: %v", rules(unknown))
	}
}
