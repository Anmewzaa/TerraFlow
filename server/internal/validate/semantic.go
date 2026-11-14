package validate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// Canvas runs every rule: the structural ones and the semantic ones. The result is in a
// fixed order (structural first, then by node order, rule, field) so the same canvas always
// gives the same answer.
func Canvas(c store.CanvasState, cat Catalog) []Finding {
	out := Structural(c)
	for _, f := range out {
		if f.Rule == "limit" && f.Node == "" {
			return out // too big to judge further
		}
	}
	byID := make(map[string]store.CanvasNode, len(c.Nodes))
	for _, n := range c.Nodes {
		byID[n.ID] = n
	}
	order := make(map[string]int, len(c.Nodes))
	for i, n := range c.Nodes {
		order[n.ID] = i
	}

	var sem []Finding
	add := func(f Finding) {
		if f.Severity == "" {
			f.Severity = Error
		}
		sem = append(sem, f)
	}

	seen := map[string]string{} // folder/name -> node id
	for _, n := range c.Nodes {
		if n.Kind != "resource" && n.NestField == "" && n.InstanceName != "" {
			key := n.TargetFolder + "/" + n.InstanceName
			if other, dup := seen[key]; dup && n.Existing == nil {
				add(Finding{Node: n.ID, Field: "instance_name", Rule: "duplicate",
					Message: fmt.Sprintf("another resource in %q is already named %q (%s)", n.TargetFolder, n.InstanceName, other)})
			} else if !dup {
				seen[key] = n.ID
			}
		}
		if n.Kind == "resource" {
			if n.ResourceType == "" {
				add(Finding{Node: n.ID, Field: "resource_type", Rule: "required", Message: "resource type is required, for example azurerm_resource_group"})
			}
			if n.InstanceName == "" {
				add(Finding{Node: n.ID, Field: "instance_name", Rule: "required", Message: "a name is required"})
			}
			continue
		}
		addFor := add
		if n.Existing != nil {
			// A block that is already in the repository may predate the schema (an older module
			// version, a hand-written argument set). Editing it must not be blocked by what the
			// schema asks of a new block, so these findings are warnings for it.
			addFor = func(f Finding) {
				if f.Rule == "required" || f.Rule == "schema" || f.Rule == "module" {
					f.Severity = Warning
				}
				add(f)
			}
		}
		semanticModule(n, byID, c, cat, addFor)
	}
	for _, e := range c.Edges {
		if _, ok := byID[e.From]; !ok && !strings.Contains(e.From, ".module.") {
			add(Finding{Rule: "edge", Message: fmt.Sprintf("an arrow starts at %q, which is not on the canvas", e.From)})
		}
		if _, ok := byID[e.To]; !ok && !strings.Contains(e.To, ".module.") {
			add(Finding{Rule: "edge", Message: fmt.Sprintf("an arrow ends at %q, which is not on the canvas", e.To)})
		}
	}
	sort.SliceStable(sem, func(i, j int) bool {
		a, b := sem[i], sem[j]
		if order[a.Node] != order[b.Node] {
			return order[a.Node] < order[b.Node]
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Field < b.Field
	})
	return append(out, sem...)
}

// HasErrors reports whether any finding is an error.
func HasErrors(fs []Finding) bool {
	for _, f := range fs {
		if f.Severity == Error {
			return true
		}
	}
	return false
}

// HasStructural reports whether any finding must refuse the request outright.
func HasStructural(fs []Finding) bool {
	for _, f := range fs {
		if f.Structural {
			return true
		}
	}
	return false
}

type canvasMeta struct {
	Parents        []string `json:"parents"`
	ParentRequired bool     `json:"parent_required"`
	ParentRef      struct {
		Field string `json:"field"`
	} `json:"parent_ref"`
}

type schemaHead struct {
	Required []string   `json:"required"`
	Canvas   canvasMeta `json:"x-tf-canvas"`
}

func semanticModule(n store.CanvasNode, byID map[string]store.CanvasNode, c store.CanvasState, cat Catalog, add func(Finding)) {
	if cat == nil {
		return
	}
	mod, ok := cat.Module(n.ModuleName, n.ModuleVersion)
	if !ok {
		add(Finding{Node: n.ID, Field: "module_name", Rule: "module",
			Message: fmt.Sprintf("module %q version %q is not in the catalog this project can use", n.ModuleName, n.ModuleVersion)})
		return
	}
	var head schemaHead
	if err := json.Unmarshal(mod.Schema, &head); err != nil {
		add(Finding{Node: n.ID, Rule: "module", Message: "the module's schema cannot be read"})
		return
	}

	// Containment.
	if n.ParentID == "" {
		if head.Canvas.ParentRequired {
			add(Finding{Node: n.ID, Rule: "parent", Message: "it must be placed in " + orAParent(head.Canvas.Parents)})
		}
	} else if parent, ok := byID[n.ParentID]; !ok {
		add(Finding{Node: n.ID, Rule: "parent", Message: "its parent is not on the canvas"})
	} else {
		if len(head.Canvas.Parents) > 0 && !contains(head.Canvas.Parents, parent.ModuleName) {
			add(Finding{Node: n.ID, Rule: "parent", Message: fmt.Sprintf("it cannot be placed in %q; it goes in %s", parent.ModuleName, orAParent(head.Canvas.Parents))})
		}
		if cycle(n.ID, byID) {
			add(Finding{Node: n.ID, Rule: "parent", Message: "resources are placed inside each other in a circle"})
		}
	}

	linkChecks(n, mod, c, cat, add)
	schemaChecks(n, mod.Schema, add)
	secretChecks(n, mod.Schema, add)

	// Required arguments. The parent fills one of them in when the HCL is written.
	for _, field := range head.Required {
		if head.Canvas.ParentRef.Field == field && n.ParentID != "" {
			continue
		}
		if isEmpty(n.Values[field]) && !suppliedByChildren(n, field, c) {
			add(Finding{Node: n.ID, Field: field, Rule: "required", Message: strings.ReplaceAll(field, "_", " ") + " is required"})
		}
	}

	// Values against the schema. What cannot be judged (a reference, a project variable) is
	// left out, never passed: it is "unknown" and the plan decides.
	clean, skipped := judgeable(n.Values)
	sch, err := compiled(mod)
	if err != nil {
		add(Finding{Node: n.ID, Rule: "module", Message: "the module's schema cannot be used: " + err.Error()})
		return
	}
	verr := sch.Validate(map[string]interface{}(clean))
	var ve *jsonschema.ValidationError
	if !errors.As(verr, &ve) {
		return
	}
	printer := message.NewPrinter(language.English)
	for _, leaf := range leaves(ve) {
		loc := "/" + strings.Join(leaf.InstanceLocation, "/")
		if req, ok := leaf.ErrorKind.(*kind.Required); ok {
			// Required fields are handled above for the top level; below it, a missing field
			// that was only skipped (a reference) is not missing.
			var real []string
			for _, m := range req.Missing {
				if skipped[strings.TrimSuffix(loc, "/")+"/"+m] || len(leaf.InstanceLocation) == 0 {
					continue
				}
				real = append(real, m)
			}
			if len(real) == 0 {
				continue
			}
			add(Finding{Node: n.ID, Field: strings.Join(leaf.InstanceLocation, "."), Rule: "required",
				Message: strings.Join(real, ", ") + " is required in " + strings.Join(leaf.InstanceLocation, ".")})
			continue
		}
		add(Finding{Node: n.ID, Field: strings.Join(leaf.InstanceLocation, "."), Rule: "schema",
			Message: leaf.ErrorKind.LocalizedString(printer)})
	}
}

func leaves(ve *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(ve.Causes) == 0 {
		return []*jsonschema.ValidationError{ve}
	}
	var out []*jsonschema.ValidationError
	for _, c := range ve.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

func orAParent(parents []string) string {
	if len(parents) == 0 {
		return "a parent resource"
	}
	return strings.Join(parents, " or ")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func cycle(id string, byID map[string]store.CanvasNode) bool {
	cur := id
	for i := 0; i < 64; i++ {
		n, ok := byID[cur]
		if !ok || n.ParentID == "" {
			return false
		}
		if n.ParentID == id {
			return true
		}
		cur = n.ParentID
	}
	return true // a chain this long is treated as a loop
}

func isEmpty(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []interface{}:
		return len(t) == 0
	}
	return false
}

// judgeable returns a copy of values without anything that cannot be judged now, and the
// set of JSON pointers that were left out.
func judgeable(values map[string]interface{}) (map[string]interface{}, map[string]bool) {
	skipped := map[string]bool{}
	var walk func(path string, v interface{}) (interface{}, bool)
	walk = func(path string, v interface{}) (interface{}, bool) {
		switch t := v.(type) {
		case string:
			if placeholder.MatchString(t) {
				skipped[path] = true
				return nil, false
			}
			return t, true
		case []interface{}:
			out := make([]interface{}, 0, len(t))
			for i, x := range t {
				if y, ok := walk(fmt.Sprintf("%s/%d", path, i), x); ok {
					out = append(out, y)
				} else {
					out = append(out, nil) // keep positions; a null in a typed list is judged below
				}
			}
			return out, true
		case map[string]interface{}:
			if _, isExpr := t["$expr"]; isExpr && len(t) == 1 {
				skipped[path] = true
				return nil, false
			}
			out := make(map[string]interface{}, len(t))
			for k, x := range t {
				if y, ok := walk(path+"/"+k, x); ok {
					out[k] = y
				}
			}
			return out, true
		}
		return v, true
	}
	clean := map[string]interface{}{}
	for k, v := range values {
		if y, ok := walk("/"+k, v); ok {
			clean[k] = y
		}
	}
	return clean, skipped
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*jsonschema.Schema{}
)

// compiled compiles a module's schema once per content (names and versions can be
// republished, so the content hash is part of the key).
func compiled(m Module) (*jsonschema.Schema, error) {
	sum := sha256.Sum256(m.Schema)
	key := m.Name + "@" + m.Version + "#" + hex.EncodeToString(sum[:8])
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if s, ok := cache[key]; ok {
		return s, nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(m.Schema))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("module.json", doc); err != nil {
		return nil, err
	}
	s, err := c.Compile("module.json")
	if err != nil {
		return nil, err
	}
	if len(cache) > 512 {
		cache = map[string]*jsonschema.Schema{}
	}
	cache[key] = s
	return s, nil
}

// suppliedByChildren: resources placed inside n that are folded into its input `field` when the HCL is
// written (a subnet inside a virtual network supplies `subnets`), so the input is not missing.
func suppliedByChildren(n store.CanvasNode, field string, c store.CanvasState) bool {
	for _, ch := range c.Nodes {
		if ch.ParentID == n.ID && ch.NestField == field {
			return true
		}
	}
	return false
}
