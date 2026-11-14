package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type schemaCheck struct {
	Rule    string   `json:"rule"`
	Field   string   `json:"field"`
	With    string   `json:"with"`
	Fields  []string `json:"fields"`
	Message string   `json:"message"`
}

// schemaChecks runs the rules a module carries itself (x-tf-checks, 8.13): `required_with` (field
// needs with) and `exclusive` (at most one of fields). An input that cannot be judged now (a
// reference, a project variable) counts as set, because it may well be, but it is never reported
// as missing.
func schemaChecks(n store.CanvasNode, schema json.RawMessage, add func(Finding)) {
	var s struct {
		Checks []schemaCheck `json:"x-tf-checks"`
	}
	if json.Unmarshal(schema, &s) != nil {
		return
	}
	set := func(f string) (isSet, unknown bool) {
		v, has := n.Values[f]
		if !has {
			return false, false
		}
		if m, ok := v.(map[string]interface{}); ok && isExprMap(m) {
			return true, true
		}
		if str, ok := v.(string); ok && placeholder.MatchString(str) {
			return true, true
		}
		return !isEmpty(v), false
	}
	for _, c := range s.Checks {
		switch c.Rule {
		case "required_with":
			a, _ := set(c.Field)
			b, unknown := set(c.With)
			if a && !b && !unknown {
				msg := c.Message
				if msg == "" {
					msg = fmt.Sprintf("%s needs %s", c.Field, c.With)
				}
				add(Finding{Node: n.ID, Field: c.With, Rule: "check", Message: msg})
			}
		case "exclusive":
			var on []string
			for _, f := range c.Fields {
				if isSet, _ := set(f); isSet {
					on = append(on, f)
				}
			}
			if len(on) > 1 {
				msg := c.Message
				if msg == "" {
					msg = "choose only one of " + strings.Join(c.Fields, ", ")
				}
				add(Finding{Node: n.ID, Field: on[1], Rule: "check", Message: msg})
			}
		}
	}
}
