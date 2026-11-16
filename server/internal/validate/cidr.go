package validate

import (
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// cidrChecks runs the address rules a module carries in x-tf-checks (the network modules, D-076):
//   - cidr: every literal in field is a CIDR block with no host bits (10.0.1.0/24, not 10.0.1.5/24);
//   - cidr_within_parent: every block lies inside one of the blocks of parent_field of the resource it is
//     placed in (a subnet inside its virtual network's address space);
//   - cidr_no_overlap: the blocks do not overlap each other or the blocks of the resources placed in
//     the same parent with the same module (two subnets of one network).
//
// A value that cannot be judged now (a reference, a project variable, a text that is not a block) is
// skipped, never reported as outside: honest state (rule 8). Nothing is read from Azure.
func cidrChecks(n store.CanvasNode, schema json.RawMessage, c store.CanvasState, byID map[string]store.CanvasNode, add func(Finding)) {
	var s struct {
		Checks []struct {
			Rule        string `json:"rule"`
			Field       string `json:"field"`
			ParentField string `json:"parent_field"`
			Message     string `json:"message"`
		} `json:"x-tf-checks"`
	}
	if json.Unmarshal(schema, &s) != nil {
		return
	}
	for _, ck := range s.Checks {
		switch ck.Rule {
		case "cidr":
			_, bad, _ := blocksOf(n.Values[ck.Field])
			for _, b := range bad {
				add(Finding{Node: n.ID, Field: ck.Field, Rule: "cidr", Message: orMsg(ck.Message, fmt.Sprintf("%q is not a CIDR block such as 10.0.1.0/24 (the address must be the first of its block)", b))})
			}
		case "cidr_within_parent":
			parent, ok := byID[n.ParentID]
			if !ok || n.ParentID == "" {
				continue
			}
			mine, _, _ := blocksOf(n.Values[ck.Field])
			theirs, badSpace, unknown := blocksOf(parent.Values[ck.ParentField])
			if len(mine) == 0 || len(theirs) == 0 || unknown || len(badSpace) > 0 {
				continue // nothing to compare yet, or the parent's space is not known
			}
			for _, m := range mine {
				if !within(m, theirs) {
					add(Finding{Node: n.ID, Field: ck.Field, Rule: "cidr", Message: orMsg(ck.Message,
						fmt.Sprintf("%s is outside the address space of %s (%s)", m, parentLabel(parent), joinBlocks(theirs)))})
				}
			}
		case "cidr_no_overlap":
			mine, _, _ := blocksOf(n.Values[ck.Field])
			for i, a := range mine {
				for _, b := range mine[i+1:] {
					if a.Overlaps(b) {
						add(Finding{Node: n.ID, Field: ck.Field, Rule: "cidr", Message: fmt.Sprintf("%s and %s overlap", a, b)})
					}
				}
			}
			if n.ParentID == "" {
				continue
			}
			for _, o := range c.Nodes {
				if o.ID == n.ID {
					break // only the nodes before this one: each pair is reported once, on the later node
				}
				if o.ParentID != n.ParentID || o.ModuleName != n.ModuleName {
					continue
				}
				theirs, _, _ := blocksOf(o.Values[ck.Field])
				for _, a := range mine {
					for _, b := range theirs {
						if a.Overlaps(b) {
							add(Finding{Node: n.ID, Field: ck.Field, Rule: "cidr", Message: fmt.Sprintf("%s overlaps %s of %s", a, b, labelOf(o))})
						}
					}
				}
			}
		}
	}
}

func orMsg(custom, fallback string) string {
	if custom != "" {
		return custom
	}
	return fallback
}

func labelOf(n store.CanvasNode) string {
	if s, ok := plainString(n.Values["name"]); ok && s != "" {
		return s
	}
	return n.InstanceName
}

func parentLabel(n store.CanvasNode) string { return labelOf(n) }

func joinBlocks(bs []netip.Prefix) string {
	out := ""
	for i, b := range bs {
		if i > 0 {
			out += ", "
		}
		out += b.String()
	}
	return out
}

// within says whether block lies entirely inside one of the outer blocks.
func within(block netip.Prefix, outer []netip.Prefix) bool {
	for _, o := range outer {
		if o.Addr().Is4() == block.Addr().Is4() && o.Bits() <= block.Bits() && o.Contains(block.Addr()) {
			return true
		}
	}
	return false
}

// blocksOf reads the CIDR blocks of an input that is a list of text (or one text). bad holds the
// literals that are not a block; partial is true when something could not be judged (a reference or a
// project variable, in the value or among its items).
func blocksOf(v interface{}) (blocks []netip.Prefix, bad []string, partial bool) {
	var items []interface{}
	switch x := v.(type) {
	case []interface{}:
		items = x
	case string:
		items = []interface{}{x}
	case nil:
		return nil, nil, false
	default:
		return nil, nil, true // a reference ({"$expr": ...}) or something this check does not read
	}
	for _, it := range items {
		str, isStr := it.(string)
		if !isStr {
			partial = true
			continue
		}
		if str == "" {
			continue
		}
		if placeholder.MatchString(str) {
			partial = true
			continue
		}
		p, err := netip.ParsePrefix(str)
		if err != nil || p != p.Masked() {
			bad = append(bad, str)
			continue
		}
		blocks = append(blocks, p)
	}
	return blocks, bad, partial
}
