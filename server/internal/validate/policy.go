package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// CanvasWithPolicy is Canvas plus the project's own rules (8.13). The policy findings follow the
// schema findings, in canvas order, so the same canvas always gives the same list. A nil policy
// is Canvas itself.
func CanvasWithPolicy(c store.CanvasState, cat Catalog, checks *hcl.Checks) []Finding {
	out := Canvas(c, cat)
	if checks == nil || len(c.Nodes) > MaxNodes {
		return out
	}
	sev := Warning
	if checks.Strict {
		sev = Error
	}
	for _, n := range c.Nodes {
		if n.Kind == "resource" || n.NestField != "" {
			continue
		}
		s := sev
		if n.Existing != nil {
			s = Warning // a block already in the repository is never blocked by a new rule
		}
		add := func(field, msg string) {
			out = append(out, Finding{Severity: s, Node: n.ID, Field: field, Rule: "policy", Message: msg})
		}
		if pat, ok := checks.Naming[n.ModuleName]; ok {
			if name, ok := plainString(n.Values["name"]); ok && name != "" {
				if re, err := regexp.Compile(pat); err == nil && !re.MatchString(name) {
					add("name", fmt.Sprintf("the name %q does not follow this project's naming rule for %s (%s)", name, n.ModuleName, pat))
				}
			}
		}
		if len(checks.AllowedRegions) > 0 {
			if loc, ok := plainString(n.Values["location"]); ok && loc != "" && !contains(checks.AllowedRegions, strings.ToLower(loc)) {
				add("location", fmt.Sprintf("%s is not one of this project's regions (%s)", loc, strings.Join(checks.AllowedRegions, ", ")))
			}
		}
		if len(checks.RequiredTags) > 0 && hasTagsInput(cat, n) {
			if tags, ok := n.Values["tags"].(map[string]interface{}); ok && !isExprMap(tags) {
				var missing []string
				for _, t := range checks.RequiredTags {
					if v, has := tags[t]; !has || isEmpty(v) {
						missing = append(missing, t)
					}
				}
				if len(missing) > 0 {
					sort.Strings(missing)
					add("tags", "missing the required tags: "+strings.Join(missing, ", "))
				}
			} else if !ok && n.Values["tags"] == nil {
				add("tags", "missing the required tags: "+strings.Join(sortedCopy(checks.RequiredTags), ", "))
			}
		}
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// plainString is a value that is a plain string: not a reference and not a project variable.
func plainString(v interface{}) (string, bool) {
	s, ok := v.(string)
	if !ok || placeholder.MatchString(s) {
		return "", false
	}
	return s, true
}

func isExprMap(m map[string]interface{}) bool {
	_, ok := m["$expr"]
	return ok && len(m) == 1
}

// hasTagsInput says whether the node's module takes tags at all.
func hasTagsInput(cat Catalog, n store.CanvasNode) bool {
	if cat == nil {
		return false
	}
	mod, ok := cat.Module(n.ModuleName, n.ModuleVersion)
	if !ok {
		return false
	}
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(mod.Schema, &s) != nil {
		return false
	}
	_, has := s.Properties["tags"]
	return has
}
