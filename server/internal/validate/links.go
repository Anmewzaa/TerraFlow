package validate

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

var moduleRef = regexp.MustCompile(`^module\.([A-Za-z_][A-Za-z0-9_-]*)\.`)

type linkSchema struct {
	Link struct {
		Offers []struct {
			Kind string `json:"kind"`
		} `json:"offers"`
	} `json:"x-tf-link"`
	Properties map[string]struct {
		UI struct {
			Pick struct {
				Link string `json:"link"`
			} `json:"pick"`
		} `json:"x-tf-ui"`
	} `json:"properties"`
}

func offersKind(schema json.RawMessage, kind string) bool {
	var s linkSchema
	if json.Unmarshal(schema, &s) != nil {
		return false
	}
	for _, o := range s.Link.Offers {
		if o.Kind == kind {
			return true
		}
	}
	return false
}

// exprsOf lists the expression texts of a value that is one expression or a list of them.
func exprsOf(v interface{}) []string {
	switch t := v.(type) {
	case map[string]interface{}:
		if e, ok := t["$expr"].(string); ok && len(t) == 1 {
			return []string{e}
		}
	case []interface{}:
		var out []string
		for _, x := range t {
			out = append(out, exprsOf(x)...)
		}
		return out
	}
	return nil
}

// linkChecks refuses a link to a resource that does not offer what the field asks for (8.11):
// a field with x-tf-ui.pick.link set to a kind may only point at a module that offers that kind.
// A target that is not on the canvas (a block already in the repository) cannot be judged here.
func linkChecks(n store.CanvasNode, mod Module, c store.CanvasState, cat Catalog, add func(Finding)) {
	var s linkSchema
	if json.Unmarshal(mod.Schema, &s) != nil {
		return
	}
	for field, p := range s.Properties {
		kind := p.UI.Pick.Link
		if kind == "" {
			continue
		}
		for _, e := range exprsOf(n.Values[field]) {
			m := moduleRef.FindStringSubmatch(e)
			if m == nil {
				continue
			}
			for _, t := range c.Nodes {
				if t.ID == n.ID || t.TargetFolder != n.TargetFolder || t.InstanceName != m[1] || t.Kind == "resource" {
					continue
				}
				tm, ok := cat.Module(t.ModuleName, t.ModuleVersion)
				if ok && !offersKind(tm.Schema, kind) && !childOffers(t, kind, c, cat) {
					add(Finding{Node: n.ID, Field: field, Rule: "link",
						Message: fmt.Sprintf("%s cannot be linked to %q: it does not offer a %s", field, t.ModuleName, kind)})
				}
			}
		}
	}
}

// childOffers: some resource placed in t offers the kind through its parent (a subnet is reached
// as module.<vnet>.subnet_ids["name"], so the reference names the virtual network).
func childOffers(t store.CanvasNode, kind string, c store.CanvasState, cat Catalog) bool {
	for _, ch := range c.Nodes {
		if ch.ParentID != t.ID {
			continue
		}
		if cm, ok := cat.Module(ch.ModuleName, ch.ModuleVersion); ok && offersKind(cm.Schema, kind) {
			return true
		}
	}
	return false
}
