// Package validate is the one place that decides whether a canvas is acceptable (D-065).
//
// It is pure: no database, no network, no Terraform binary, no Azure. The browser asks it
// for instant feedback; the server runs it again when a draft is saved, when a change is
// published and when it is approved, so a client that skips the screens cannot skip the
// rules. Two levels:
//
//   - Structural findings are about input that must never be stored: malformed, oversized,
//     or unsafe (a path with "..", an expression that would close the HCL block). They
//     refuse the request.
//   - Semantic findings are about work that is not finished or not valid yet (a required
//     argument empty, a subnet outside a VNet, a value that does not fit its schema). A
//     draft may carry them; publish and approve do not.
//
// A value that cannot be evaluated (a reference, a project variable) is never judged
// either way: it is skipped, not passed (hard rule 8).
package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	hcllib "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// Severity of a finding.
const (
	Error   = "error"
	Warning = "warning"
	Info    = "info"
)

// Finding is one thing wrong (or worth knowing) about one node.
type Finding struct {
	Severity string `json:"severity"`
	// Node is the canvas node id; empty for a finding about the canvas as a whole.
	Node  string `json:"node,omitempty"`
	Field string `json:"field,omitempty"`
	// Rule is a stable short name (required, parent, schema, limit...) tests and screens can rely on.
	Rule    string `json:"rule"`
	Message string `json:"message"`
	// Structural findings refuse the request that carries them (save, publish, approve).
	Structural bool `json:"structural,omitempty"`
}

// Limits keep one request from exhausting memory or time. They are generous for real work.
const (
	MaxNodes       = 500
	MaxEdges       = 2000
	MaxValueDepth  = 12
	MaxStringBytes = 64 << 10
	MaxNameLen     = 64
	MaxExprBytes   = 4 << 10
)

var (
	blockName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	resourceType = regexp.MustCompile(`^[a-z][a-z0-9]*_[a-z0-9_]+$`)
	placeholder  = regexp.MustCompile(`\{[a-z_][a-z0-9_]*\}`)
)

// Module is what the validator needs to know about a catalog entry.
type Module struct {
	Name, Version string
	Schema        json.RawMessage
}

// Catalog finds the modules a project may use (its own and the global ones, D-060).
type Catalog interface {
	Module(name, version string) (Module, bool)
}

// MapCatalog is a Catalog over a list.
type MapCatalog map[string]Module

// NewMapCatalog indexes modules by name and version.
func NewMapCatalog(mods []Module) MapCatalog {
	c := MapCatalog{}
	for _, m := range mods {
		c[m.Name+"@"+m.Version] = m
	}
	return c
}

// Module implements Catalog.
func (c MapCatalog) Module(name, version string) (Module, bool) {
	m, ok := c[name+"@"+version]
	return m, ok
}

// Parse reads a stored canvas. It refuses what is not JSON of the right shape.
func Parse(raw []byte) (store.CanvasState, error) {
	var c store.CanvasState
	if err := json.Unmarshal(raw, &c); err != nil {
		return store.CanvasState{}, fmt.Errorf("the canvas is not valid: %v", err)
	}
	return c, nil
}

// Structural returns only the findings that must refuse a request, without needing the
// catalog. It is what a draft save runs.
func Structural(c store.CanvasState) []Finding {
	var out []Finding
	add := func(f Finding) { f.Severity, f.Structural = Error, true; out = append(out, f) }

	if len(c.Nodes) > MaxNodes {
		add(Finding{Rule: "limit", Message: fmt.Sprintf("a canvas holds at most %d resources (this one has %d)", MaxNodes, len(c.Nodes))})
		return out // checking the rest of an oversized canvas is not worth the time
	}
	if len(c.Edges) > MaxEdges {
		add(Finding{Rule: "limit", Message: fmt.Sprintf("a canvas holds at most %d arrows (this one has %d)", MaxEdges, len(c.Edges))})
		return out
	}
	ids := map[string]bool{}
	for _, n := range c.Nodes {
		if n.ID == "" || len(n.ID) > 128 {
			add(Finding{Node: n.ID, Rule: "id", Message: "every resource needs an id of up to 128 characters"})
			continue
		}
		if ids[n.ID] {
			add(Finding{Node: n.ID, Rule: "id", Message: "two resources have the same id"})
		}
		ids[n.ID] = true

		if n.InstanceName != "" && (len(n.InstanceName) > MaxNameLen || !blockName.MatchString(n.InstanceName)) {
			add(Finding{Node: n.ID, Field: "instance_name", Rule: "name", Message: "the name may use letters, digits, _ and -, must not start with a digit, and has at most 64 characters"})
		}
		if n.TargetFolder != "" {
			if err := hcl.ValidFolder(n.TargetFolder); err != nil {
				add(Finding{Node: n.ID, Field: "target_folder", Rule: "path", Message: "the folder is not allowed: " + err.Error()})
			}
		}
		if n.Filename != "" {
			if err := hcl.ValidFilename(n.Filename); err != nil {
				add(Finding{Node: n.ID, Field: "filename", Rule: "path", Message: "the file name is not allowed: " + err.Error()})
			}
		}
		if n.Kind == "resource" && n.ResourceType != "" && !resourceType.MatchString(n.ResourceType) {
			add(Finding{Node: n.ID, Field: "resource_type", Rule: "name", Message: "the resource type must look like azurerm_resource_group"})
		}
		walkValues(n.ID, "", n.Values, 0, add)
	}
	for _, e := range c.Edges {
		if len(e.ID) > 256 || len(e.From) > 256 || len(e.To) > 256 || len(e.Kind) > 64 {
			add(Finding{Rule: "limit", Message: "an arrow has a field that is too long"})
		}
	}
	return out
}

// walkValues checks the shape of a value tree: depth, string size, and that an expression
// is exactly one HCL expression (so it cannot close the block it is written into).
func walkValues(node, path string, v interface{}, depth int, report func(Finding)) {
	if depth > MaxValueDepth {
		report(Finding{Node: node, Field: path, Rule: "limit", Message: "values are nested too deeply"})
		return
	}
	switch t := v.(type) {
	case string:
		if len(t) > MaxStringBytes {
			report(Finding{Node: node, Field: path, Rule: "limit", Message: "a value is too long"})
		}
	case []interface{}:
		for i, x := range t {
			walkValues(node, fmt.Sprintf("%s[%d]", path, i), x, depth+1, report)
		}
	case map[string]interface{}:
		if expr, ok := t["$expr"]; ok && len(t) == 1 {
			text, isStr := expr.(string)
			if !isStr {
				report(Finding{Node: node, Field: path, Rule: "expr", Message: "an expression must be text"})
				return
			}
			if err := checkExpr(text); err != nil {
				report(Finding{Node: node, Field: path, Rule: "expr", Message: err.Error()})
			}
			return
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(k) > 256 {
				report(Finding{Node: node, Field: path, Rule: "limit", Message: "a key is too long"})
				continue
			}
			p := k
			if path != "" {
				p = path + "." + k
			}
			walkValues(node, p, t[k], depth+1, report)
		}
	}
}

// checkExpr accepts exactly one HCL expression. Anything that does not parse, or that
// carries more than one expression, could break out of the block it is pasted into.
func checkExpr(text string) error {
	if len(text) > MaxExprBytes {
		return fmt.Errorf("an expression is too long")
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("an expression is empty")
	}
	if _, diags := hclsyntax.ParseExpression([]byte(text), "expr", hcllib.InitialPos); diags.HasErrors() {
		return fmt.Errorf("the expression %q is not valid HCL", clip(text))
	}
	return nil
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
