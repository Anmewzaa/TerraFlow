package stacktpl

import (
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func node(id, module, inst string, values map[string]interface{}) store.CanvasNode {
	return store.CanvasNode{ID: id, ModuleName: module, ModuleVersion: "v1", InstanceName: inst, TargetFolder: "vnet", Values: values, PositionX: 10, PositionY: 20}
}

// A VNet's subnet group: a subnet inside a VNet that is not selected, a private endpoint inside the
// subnet pointing at a key vault that is not selected.
func canvas() store.CanvasState {
	vnet := node("v", "virtual-network", "vnet", map[string]interface{}{"name": "vnet-a"})
	sn := node("s", "subnet", "sn", map[string]interface{}{"name": "snet-app", "size": "24"})
	sn.ParentID = "v"
	pe := node("p", "private-endpoint", "pe", map[string]interface{}{
		"name": "pe-kv", "private_connection_resource_id": map[string]interface{}{"$expr": "module.kv.id"},
		"private_dns_zone_ids": []interface{}{map[string]interface{}{"$expr": "module.sn.id"}},
	})
	pe.ParentID = "s"
	kv := node("k", "key-vault", "kv", map[string]interface{}{"name": "kv-a"})
	return store.CanvasState{
		Nodes: []store.CanvasNode{vnet, sn, pe, kv},
		Edges: []store.CanvasEdge{{ID: "e1", From: "s", To: "p"}, {ID: "e2", From: "k", To: "p"}},
	}
}

func TestBuildLeavesSlotsForWhatPointedOutside(t *testing.T) {
	tpl, problems := Build("Private endpoint group", canvas(), []string{"s", "p"}, nil, nil)
	if len(problems) != 0 {
		t.Fatalf("%v", problems)
	}
	if len(tpl.Nodes) != 2 || len(tpl.Edges) != 1 || tpl.Edges[0].ID != "e1" {
		t.Fatalf("the nodes and the arrow between them: %+v", tpl)
	}
	slots := map[string]Slot{}
	for _, s := range tpl.Slots {
		slots[s.Name] = s
	}
	if s, ok := slots["parent of sn"]; !ok || s.Kind != "parent" || s.Accepts != "virtual-network" {
		t.Fatalf("the subnet's VNet is a slot: %+v", tpl.Slots)
	}
	if s, ok := slots["module kv"]; !ok || s.Kind != "reference" || s.Accepts != "key-vault" {
		t.Fatalf("the key vault the endpoint pointed at is a slot: %+v", tpl.Slots)
	}
	if _, ok := slots["module sn"]; ok {
		t.Fatal("a reference inside the selection is not a slot")
	}
	for _, n := range tpl.Nodes {
		if n.ID == "s" && n.ParentID != "" {
			t.Fatal("a parent outside the template is removed")
		}
	}
}

func TestBuildRefusesWhatCannotBeATemplate(t *testing.T) {
	c := canvas()
	c.Nodes[3].Existing = &store.ExistingRef{Folder: "vnet", Filename: "kv.tf", Label: "kv"}
	if _, p := Build("x", c, []string{"k"}, nil, nil); len(p) == 0 || !strings.Contains(p[0], "already in the repository") {
		t.Fatalf("a block of the repository: %v", p)
	}
	if _, p := Build("", canvas(), []string{"s"}, nil, nil); len(p) == 0 {
		t.Fatal("a name is needed")
	}
	if _, p := Build("x", canvas(), nil, nil, nil); len(p) == 0 {
		t.Fatal("something must be selected")
	}
}

func TestParametersBecomeProjectVariables(t *testing.T) {
	secret := func(m, f string) bool { return m == "key-vault" && f == "password" }
	tpl, p := Build("x", canvas(), []string{"s"}, []Param{{Name: "subnet_size", Node: "sn", Field: "size"}}, secret)
	if len(p) != 0 || tpl.Nodes[0].Values["size"] != "{subnet_size}" || tpl.Params[0].Default != "24" {
		t.Fatalf("%v %+v", p, tpl)
	}
	for name, params := range map[string][]Param{
		"a reference":    {{Name: "a", Node: "pe", Field: "private_connection_resource_id"}},
		"a list":         {{Name: "a", Node: "pe", Field: "private_dns_zone_ids"}},
		"a bad name":     {{Name: "A b", Node: "sn", Field: "size"}},
		"an unknown one": {{Name: "a", Node: "ghost", Field: "size"}},
		"twice":          {{Name: "a", Node: "sn", Field: "size"}, {Name: "a", Node: "sn", Field: "name"}},
		"a secret":       {{Name: "a", Node: "kv", Field: "password"}},
	} {
		sel := []string{"s", "p", "k"}
		c := canvas()
		c.Nodes[3].Values["password"] = "x"
		if _, p := Build("x", c, sel, params, secret); len(p) == 0 {
			t.Errorf("%s must be refused", name)
		}
	}
	vars := Variables(tpl, []string{"dev", "prod"}, map[string]map[string]string{"subnet_size": {"prod": "22"}})
	if vars[0].Values["dev"] != "24" || vars[0].Values["prod"] != "22" {
		t.Fatalf("a value per environment, the default where none: %+v", vars)
	}
}

func TestInstantiateRenamesFillsSlotsAndRefusesClashes(t *testing.T) {
	tpl, _ := Build("g", canvas(), []string{"s", "p"}, nil, nil)
	in := Input{Env: "dev", Prefix: "app-", Slots: map[string]string{"parent of sn": "vnet-existing-id", "module kv": "kv_main"}, OffsetX: 100}
	if _, p := Instantiate(tpl, Input{Slots: map[string]string{"module kv": "kv_main"}}); len(p) == 0 {
		t.Fatal("a slot left empty is a problem")
	}
	nodes, p := Instantiate(tpl, in)
	if len(p) != 0 || len(nodes) != 2 {
		t.Fatalf("%v", p)
	}
	byInst := map[string]store.CanvasNode{}
	for _, n := range nodes {
		byInst[n.InstanceName] = n
	}
	sn, pe := byInst["app-sn"], byInst["app-pe"]
	if sn.ParentID != "vnet-existing-id" || pe.ParentID != sn.ID || sn.ID == "s" || sn.PositionX != 110 {
		t.Fatalf("parent slot, inner parent remapped, new ids, offset: %+v %+v", sn, pe)
	}
	if got := pe.Values["private_connection_resource_id"].(map[string]interface{})["$expr"]; got != "module.kv_main.id" {
		t.Fatalf("the reference slot points at the chosen resource: %v", got)
	}
	if got := pe.Values["private_dns_zone_ids"].([]interface{})[0].(map[string]interface{})["$expr"]; got != "module.app-sn.id" {
		t.Fatalf("a reference inside the template follows the rename: %v", got)
	}
	if sn.Values["name"] != "app-snet-app" {
		t.Fatalf("the prefix goes on the name too: %v", sn.Values["name"])
	}
	// A clash with what exists is refused; the original template is untouched.
	in.ExistingLabels = map[string]bool{"vnet/app-sn": true}
	if _, p := Instantiate(tpl, in); len(p) == 0 || !strings.Contains(p[0], "already exists") {
		t.Fatalf("a clash: %v", p)
	}
	if tpl.Nodes[0].InstanceName != "sn" {
		t.Fatal("the template must not change")
	}
	if _, p := Instantiate(tpl, Input{Prefix: "../x", Slots: in.Slots}); len(p) == 0 {
		t.Fatal("a bad prefix is refused")
	}
}
