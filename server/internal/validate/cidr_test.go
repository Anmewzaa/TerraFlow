package validate

import (
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func netCanvas(space interface{}, subnets ...[]interface{}) store.CanvasState {
	v := node("v", "virtual-network", map[string]interface{}{"name": "vnet-a", "resource_group_name": "rg", "address_space": space, "subnets": []interface{}{}})
	c := store.CanvasState{Nodes: []store.CanvasNode{v}}
	for i, p := range subnets {
		id := string(rune('a' + i))
		s := node("s"+id, "subnet", map[string]interface{}{"name": "snet-" + id, "address_prefixes": p})
		s.ParentID = "v"
		s.NestField = "subnets"
		c.Nodes = append(c.Nodes, s)
	}
	return c
}

func list(items ...interface{}) []interface{} { return items }

func cidrMessages(fs []Finding, node string) string {
	var out []string
	for _, f := range fs {
		if f.Node == node && f.Rule == "cidr" {
			out = append(out, f.Message)
		}
	}
	return strings.Join(out, " | ")
}

func TestSubnetMustLieInsideItsVirtualNetwork(t *testing.T) {
	cat := seedCatalog(t)
	c := netCanvas(list("10.0.0.0/16"), list("10.0.1.0/24"), list("10.1.0.0/24"))
	fs := Canvas(c, cat)
	if has(fs, "sa", "cidr") {
		t.Fatalf("a subnet inside the space is fine: %s", cidrMessages(fs, "sa"))
	}
	if m := cidrMessages(fs, "sb"); !strings.Contains(m, "10.1.0.0/24 is outside the address space of vnet-a (10.0.0.0/16)") {
		t.Fatalf("outside the space: %q", m)
	}
	for _, f := range fs {
		if f.Rule == "cidr" && f.Severity != Error {
			t.Fatalf("an address problem is an error: %+v", f)
		}
	}
}

func TestSubnetsMayNotOverlap(t *testing.T) {
	cat := seedCatalog(t)
	c := netCanvas(list("10.0.0.0/16"), list("10.0.1.0/24"), list("10.0.1.128/25"), list("10.0.2.0/24", "10.0.2.0/25"))
	fs := Canvas(c, cat)
	if has(fs, "sa", "cidr") {
		t.Fatal("the first of two overlapping subnets is not blamed")
	}
	if m := cidrMessages(fs, "sb"); !strings.Contains(m, "10.0.1.128/25 overlaps 10.0.1.0/24 of snet-a") {
		t.Fatalf("overlap with a sibling: %q", m)
	}
	if m := cidrMessages(fs, "sc"); !strings.Contains(m, "10.0.2.0/24 and 10.0.2.0/25 overlap") {
		t.Fatalf("overlap inside one subnet: %q", m)
	}
}

func TestACidrMustBeAValidBlock(t *testing.T) {
	cat := seedCatalog(t)
	c := netCanvas(list("10.0.0.0/16"), list("10.0.1.5/24"), list("not-a-cidr"), list("10.0.3.0/33"))
	fs := Canvas(c, cat)
	for _, id := range []string{"sa", "sb", "sc"} {
		if !has(fs, id, "cidr") {
			t.Fatalf("%s should be refused: %v", id, rules(fs))
		}
	}
	if !strings.Contains(cidrMessages(fs, "sa"), "first of its block") {
		t.Fatalf("host bits: %q", cidrMessages(fs, "sa"))
	}
	if m := cidrMessages(Canvas(netCanvas(list("10.0.0.5/16")), cat), "v"); m == "" {
		t.Fatal("a virtual network's own space is checked too")
	}
}

func TestWhatCannotBeJudgedIsNeverReported(t *testing.T) {
	cat := seedCatalog(t)
	ref := map[string]interface{}{"$expr": "var.space"}
	// The network's space is a reference: nothing to compare against.
	if fs := Canvas(netCanvas(ref, list("172.16.0.0/24")), cat); has(fs, "sa", "cidr") {
		t.Fatal("an unknown parent space is not a mismatch")
	}
	// A project variable among the items.
	if fs := Canvas(netCanvas(list("10.0.0.0/16", "{extra}"), list("172.16.0.0/24")), cat); has(fs, "sa", "cidr") {
		t.Fatal("a space with a variable in it cannot be judged")
	}
	// The subnet's own values are a reference or a variable.
	if fs := Canvas(netCanvas(list("10.0.0.0/16"), list(ref), list("{subnet_a}")), cat); has(fs, "sa", "cidr") || has(fs, "sb", "cidr") {
		t.Fatal("a reference is never reported")
	}
	// Nothing typed yet, and a subnet on no network.
	c := netCanvas(list("10.0.0.0/16"), nil)
	if fs := Canvas(c, cat); has(fs, "sa", "cidr") {
		t.Fatal("empty is the 'required' rule's business")
	}
	lone := node("l", "subnet", map[string]interface{}{"name": "snet-l", "address_prefixes": list("192.168.0.0/24")})
	if fs := Canvas(store.CanvasState{Nodes: []store.CanvasNode{lone}}, cat); has(fs, "l", "cidr") {
		t.Fatal("no parent, nothing to compare")
	}
}

func TestIPv6AndMixedFamilies(t *testing.T) {
	cat := seedCatalog(t)
	c := netCanvas(list("10.0.0.0/16", "fd00::/48"), list("fd00:0:0:1::/64"), list("fd01::/64"))
	fs := Canvas(c, cat)
	if has(fs, "sa", "cidr") {
		t.Fatalf("an IPv6 block inside an IPv6 space: %s", cidrMessages(fs, "sa"))
	}
	if !has(fs, "sb", "cidr") {
		t.Fatal("an IPv6 block outside the space")
	}
	if fs := Canvas(netCanvas(list("10.0.0.0/16"), list("fd00::/64")), cat); !has(fs, "sa", "cidr") {
		t.Fatal("an IPv6 block is not inside an IPv4 space")
	}
}
