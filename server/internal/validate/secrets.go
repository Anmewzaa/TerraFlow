package validate

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// secretChecks refuses a secret typed as text (8.12): an input marked x-tf-ui.secret takes a
// reference (var.db_password, a data source) and never the value itself, because the value would
// be written into the repository. Hard rule: no secrets in the repository.
func secretChecks(n store.CanvasNode, schema json.RawMessage, add func(Finding)) {
	var s struct {
		Properties map[string]struct {
			UI struct {
				Secret bool `json:"secret"`
			} `json:"x-tf-ui"`
		} `json:"properties"`
	}
	if json.Unmarshal(schema, &s) != nil {
		return
	}
	fields := make([]string, 0)
	for f, p := range s.Properties {
		if p.UI.Secret {
			fields = append(fields, f)
		}
	}
	sort.Strings(fields)
	for _, f := range fields {
		if str, ok := n.Values[f].(string); ok && str != "" && !placeholder.MatchString(str) {
			add(Finding{Node: n.ID, Field: f, Rule: "secret",
				Message: fmt.Sprintf("%s is a secret: give a reference such as var.%s, never the value (it would be written into the repository)", f, f)})
		}
	}
}
