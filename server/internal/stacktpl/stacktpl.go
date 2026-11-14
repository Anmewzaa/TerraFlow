// Package stacktpl makes and applies stack templates (8.15): a group of canvas nodes saved with
// the arrows between them, slots for everything that pointed outside the group (the parent VNet,
// a DNS zone), and parameters (arguments that become project variables). It works on canvases
// only: no database, no repository, no network.
package stacktpl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// Limits keep a template and what it makes small enough to review.
const (
	MaxNodes  = 200
	MaxParams = 30
	MaxName   = 80
)

var (
	nameRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,79}$`)
	paramRe     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)
	moduleRef   = regexp.MustCompile(`\bmodule\.([A-Za-z_][A-Za-z0-9_-]*)\.`)
	affixRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,20}$`)
	placeholder = regexp.MustCompile(`\{[a-z_][a-z0-9_]*\}`)
)

// Slot is a place the template leaves open: the person fills it with a resource that already
// exists when they apply it.
type Slot struct {
	// Name is how the form names it ("parent of vnet", "module key_vault").
	Name string `json:"name"`
	// Kind is "parent" (the node was placed inside something outside the template) or "reference"
	// (a value pointed at another resource through module.<instance>.<output>).
	Kind string `json:"kind"`
	// Node is the template node that has the slot (parent) or the instance name that was pointed
	// at (reference).
	Node string `json:"node"`
	// Accepts is the module the slot expects (the parent's or the target's module), for the form to filter by.
	Accepts string `json:"accepts,omitempty"`
}

// Param is an argument that is a parameter: it becomes the project variable Name.
type Param struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Field   string `json:"field"`
	Default string `json:"default,omitempty"`
}

// Template is what is stored.
type Template struct {
	Name   string             `json:"name"`
	Nodes  []store.CanvasNode `json:"nodes"`
	Edges  []store.CanvasEdge `json:"edges"`
	Slots  []Slot             `json:"slots"`
	Params []Param            `json:"params"`
}

// SecretFn says whether an input of a module is a secret (never a parameter).
type SecretFn func(module, field string) bool

// Build makes a template from the selected nodes of a canvas. params name the arguments that become
// parameters; every problem is listed (nil when fine).
func Build(name string, c store.CanvasState, selected []string, params []Param, secret SecretFn) (Template, []string) {
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	if !nameRe.MatchString(name) {
		add("the name must be 1 to %d letters, digits, spaces and . _ -", MaxName)
	}
	if len(selected) == 0 || len(selected) > MaxNodes {
		add("select between 1 and %d resources", MaxNodes)
	}
	in := map[string]bool{}
	for _, id := range selected {
		in[id] = true
	}
	byID := map[string]store.CanvasNode{}
	for _, n := range c.Nodes {
		byID[n.ID] = n
	}
	instances := map[string]store.CanvasNode{} // instance name -> node, inside the selection
	var nodes []store.CanvasNode
	for _, n := range c.Nodes { // canvas order keeps the template stable
		if !in[n.ID] {
			continue
		}
		if n.Existing != nil {
			add("%q edits a block already in the repository; only new resources can be in a template", n.InstanceName)
			continue
		}
		nodes = append(nodes, deepCopy(n))
		instances[n.InstanceName] = n
	}
	for id := range in {
		if _, ok := byID[id]; !ok {
			add("a selected resource is not on the canvas")
		}
	}
	t := Template{Name: name, Nodes: nodes, Edges: []store.CanvasEdge{}, Slots: []Slot{}, Params: []Param{}}
	for _, e := range c.Edges {
		if in[e.From] && in[e.To] {
			t.Edges = append(t.Edges, e)
		}
	}

	// Slots: whatever pointed outside the selection.
	seen := map[string]bool{}
	for i := range t.Nodes {
		n := &t.Nodes[i]
		if n.ParentID != "" && !in[n.ParentID] {
			s := Slot{Name: "parent of " + n.InstanceName, Kind: "parent", Node: n.InstanceName}
			if p, ok := byID[n.ParentID]; ok {
				s.Accepts = p.ModuleName
			}
			t.Slots = append(t.Slots, s)
			n.ParentID = ""
		}
		for _, ref := range referencesIn(n.Values) {
			if _, inside := instances[ref]; inside || seen[ref] {
				continue
			}
			seen[ref] = true
			s := Slot{Name: "module " + ref, Kind: "reference", Node: ref}
			for _, other := range c.Nodes {
				if other.InstanceName == ref && other.TargetFolder == n.TargetFolder {
					s.Accepts = other.ModuleName
				}
			}
			t.Slots = append(t.Slots, s)
		}
	}
	sort.SliceStable(t.Slots, func(i, j int) bool { return t.Slots[i].Name < t.Slots[j].Name })

	// Parameters: a plain argument becomes {name}, which the generator writes as var.<name>.
	if len(params) > MaxParams {
		add("at most %d parameters", MaxParams)
		params = params[:MaxParams]
	}
	names := map[string]bool{}
	for _, p := range params {
		if !paramRe.MatchString(p.Name) {
			add("%q is not a parameter name (lower case letters, digits and _)", p.Name)
			continue
		}
		if names[p.Name] {
			add("the parameter %q is used twice", p.Name)
			continue
		}
		names[p.Name] = true
		idx := -1
		for i, n := range t.Nodes {
			if n.InstanceName == p.Node {
				idx = i
			}
		}
		if idx < 0 {
			add("the parameter %q names a resource that is not selected", p.Name)
			continue
		}
		n := &t.Nodes[idx]
		if secret != nil && secret(n.ModuleName, p.Field) {
			add("%s of %q is a secret: a secret is never a parameter", p.Field, p.Node)
			continue
		}
		switch v := n.Values[p.Field].(type) {
		case string:
			if placeholder.MatchString(v) {
				add("%s of %q already uses a project variable", p.Field, p.Node)
				continue
			}
			if p.Default == "" {
				p.Default = v
			}
		case float64, bool:
			if p.Default == "" {
				p.Default = fmt.Sprint(v)
			}
		default:
			add("%s of %q is not a single value, so it cannot be a parameter", p.Field, p.Node)
			continue
		}
		if len(p.Default) > 200 || strings.ContainsAny(p.Default, "\n\r") {
			add("the default of %q must be one line of at most 200 characters", p.Name)
			continue
		}
		n.Values[p.Field] = "{" + p.Name + "}"
		t.Params = append(t.Params, p)
	}
	if len(problems) > 0 {
		return Template{}, problems
	}
	return t, nil
}

// Input is what the person chose when applying a template.
type Input struct {
	Env    string
	Prefix string
	Suffix string
	// Slots maps a slot name to the node it is filled with: for a parent slot the id of the node to
	// place inside; for a reference slot the instance name of the resource to point at.
	Slots map[string]string
	// ExistingLabels are the "folder/instance" names already taken in this environment (the draft and
	// the repository); a clash is refused (D-036).
	ExistingLabels map[string]bool
	// OffsetX, OffsetY place the new nodes beside what is already there.
	OffsetX, OffsetY float64
	// NewID makes a node id; nil uses a counter.
	NewID func(i int) string
}

// Instantiate makes new canvas nodes from a template. Names are made unique with the prefix and
// suffix; a slot that is not filled, or a name that clashes, is a problem.
func Instantiate(t Template, in Input) ([]store.CanvasNode, []string) {
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	if in.Prefix != "" && !affixRe.MatchString(in.Prefix) || in.Suffix != "" && !affixRe.MatchString(in.Suffix) {
		add("a prefix or suffix uses letters, digits, - and _ (up to 21 characters)")
	}
	for _, s := range t.Slots {
		if strings.TrimSpace(in.Slots[s.Name]) == "" {
			add("fill %q", s.Name)
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	newID := in.NewID
	if newID == nil {
		newID = func(i int) string { return fmt.Sprintf("draft-stack-%d", i) }
	}
	idMap := map[string]string{}
	nameMap := map[string]string{}
	for i, n := range t.Nodes {
		idMap[n.ID] = newID(i)
		nameMap[n.InstanceName] = in.Prefix + n.InstanceName + in.Suffix
	}
	// A reference slot renames the target: module.<old>. becomes module.<chosen>.
	refMap := map[string]string{}
	for _, s := range t.Slots {
		if s.Kind == "reference" {
			refMap[s.Node] = in.Slots[s.Name]
		}
	}
	out := make([]store.CanvasNode, 0, len(t.Nodes))
	taken := map[string]bool{}
	for _, n := range t.Nodes {
		c := deepCopy(n)
		c.ID = idMap[n.ID]
		c.InstanceName = nameMap[n.InstanceName]
		c.PositionX += in.OffsetX
		c.PositionY += in.OffsetY
		if n.ParentID != "" {
			c.ParentID = idMap[n.ParentID]
		}
		for _, s := range t.Slots {
			if s.Kind == "parent" && s.Node == n.InstanceName {
				c.ParentID = in.Slots[s.Name]
			}
		}
		c.Values = rename(c.Values, nameMap, refMap, in.Prefix, in.Suffix).(map[string]interface{})
		key := c.TargetFolder + "/" + c.InstanceName
		if in.ExistingLabels[key] || taken[key] {
			add("%q already exists in %s; choose another prefix or suffix", c.InstanceName, c.TargetFolder)
		}
		taken[key] = true
		out = append(out, c)
	}
	if len(problems) > 0 {
		return nil, problems
	}
	ApplyAffix(out, in.Prefix, in.Suffix)
	return out, nil
}

// rename updates references to renamed resources inside the template, points reference slots at
// the chosen resource, and puts the prefix and suffix on a resource's own name.
func rename(v interface{}, names, refs map[string]string, prefix, suffix string) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if e, ok := t["$expr"].(string); ok && len(t) == 1 {
			return map[string]interface{}{"$expr": moduleRef.ReplaceAllStringFunc(e, func(m string) string {
				inst := moduleRef.FindStringSubmatch(m)[1]
				if to, ok := refs[inst]; ok {
					return "module." + to + "."
				}
				if to, ok := names[inst]; ok {
					return "module." + to + "."
				}
				return m
			})}
		}
		out := make(map[string]interface{}, len(t))
		for k, x := range t {
			out[k] = rename(x, names, refs, prefix, suffix)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, x := range t {
			out[i] = rename(x, names, refs, prefix, suffix)
		}
		return out
	}
	return v
}

// ApplyAffix puts the prefix and suffix on a node's own `name` value when it is plain text.
func ApplyAffix(nodes []store.CanvasNode, prefix, suffix string) {
	for i := range nodes {
		if s, ok := nodes[i].Values["name"].(string); ok && s != "" && !placeholder.MatchString(s) {
			nodes[i].Values["name"] = prefix + s + suffix
		}
	}
}

// referencesIn lists the instance names a value points at through module.<instance>.<output>.
func referencesIn(v interface{}) []string {
	var out []string
	var walk func(interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case string:
			for _, m := range moduleRef.FindAllStringSubmatch(t, -1) {
				out = append(out, m[1])
			}
		case map[string]interface{}:
			for _, x := range t {
				walk(x)
			}
		case []interface{}:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(v)
	sort.Strings(out)
	return out
}

func deepCopy(n store.CanvasNode) store.CanvasNode {
	b, _ := json.Marshal(n)
	var c store.CanvasNode
	_ = json.Unmarshal(b, &c)
	if c.Values == nil {
		c.Values = map[string]interface{}{}
	}
	return c
}

// VariableValues says the project variables a template's parameters need, with the value for each
// environment the template is applied to. values maps a parameter name to env -> value; a missing
// one falls back to the default.
type VariableValues struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

// Variables lists one variable per parameter, with a value for each chosen environment.
func Variables(t Template, envs []string, values map[string]map[string]string) []VariableValues {
	out := make([]VariableValues, 0, len(t.Params))
	for _, p := range t.Params {
		v := VariableValues{Name: p.Name, Values: map[string]string{}}
		for _, env := range envs {
			val := values[p.Name][env]
			if val == "" {
				val = p.Default
			}
			if val != "" {
				v.Values[env] = val
			}
		}
		out = append(out, v)
	}
	return out
}
