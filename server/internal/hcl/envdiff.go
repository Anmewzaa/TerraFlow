package hcl

import (
	"encoding/json"
	"sort"
)

// Statuses of a compared resource or argument.
const (
	DiffSame     = "same"
	DiffDiffers  = "differs"
	DiffUnknown  = "unknown" // a value on one side could not be evaluated: never reported as equal
	DiffOnlyFrom = "only_from"
	DiffOnlyTo   = "only_to"
)

// ArgDiff is one argument that is not the same on both sides.
type ArgDiff struct {
	Name   string          `json:"name"`
	From   json.RawMessage `json:"from,omitempty"`
	To     json.RawMessage `json:"to,omitempty"`
	Status string          `json:"status"` // differs | unknown | only_from | only_to
}

// ResourceDiff is one resource paired across two environments by its id (stack, kind, label).
type ResourceDiff struct {
	ID     string    `json:"id"`
	Stack  string    `json:"stack"`
	Kind   NodeKind  `json:"kind"`
	Type   string    `json:"type"`
	Label  string    `json:"label,omitempty"`
	Status string    `json:"status"`
	Args   []ArgDiff `json:"args,omitempty"`
}

// VariableDiff is a project variable with a value in one environment and none in the other.
type VariableDiff struct {
	Name string `json:"name"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Has  string `json:"has"` // from | to
}

// DiffSummary counts the resources by status.
type DiffSummary struct {
	Same     int `json:"same"`
	Differs  int `json:"differs"`
	Unknown  int `json:"unknown"`
	OnlyFrom int `json:"only_from"`
	OnlyTo   int `json:"only_to"`
}

// DiffViolation is a difference the project marked as forbidden (checks.must_match).
type DiffViolation struct {
	Resource string `json:"resource"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// EnvDiff is the read-only comparison of two environments of one project.
type EnvDiff struct {
	From      string         `json:"from"`
	To        string         `json:"to"`
	Resources []ResourceDiff `json:"resources"`
	Variables []VariableDiff `json:"variables"`
	Summary   DiffSummary    `json:"summary"`
	// Violations are the differences the project's checks forbid between these two environments.
	Violations []DiffViolation `json:"violations"`
}

func graphNodes(g *Graph) map[string]Node {
	m := map[string]Node{}
	if g == nil {
		return m
	}
	for _, f := range g.Folders {
		for _, n := range f.Nodes {
			m[n.ID] = n
		}
	}
	return m
}

func decodeInputs(raw json.RawMessage) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// hasUnknown reports whether a value is, or holds, an unresolved value ({"$unknown": true}).
func hasUnknown(raw json.RawMessage) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return true
	}
	var scan func(any) bool
	scan = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["$unknown"]; ok {
				return true
			}
			for _, e := range x {
				if scan(e) {
					return true
				}
			}
		case []any:
			for _, e := range x {
				if scan(e) {
					return true
				}
			}
		}
		return false
	}
	return scan(v)
}

// canonical re-encodes a value so that equal values have equal bytes (object keys sorted).
func canonical(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// DiffGraphs compares the graphs of two environments. Resources are paired by id, which holds
// the stack and the block's label and so does not depend on the repository's layout.
func DiffGraphs(from, to *Graph, fromEnv, toEnv string, st *ProjectSettings) EnvDiff {
	out := EnvDiff{From: fromEnv, To: toEnv, Resources: []ResourceDiff{}, Variables: []VariableDiff{}, Violations: []DiffViolation{}}
	a, b := graphNodes(from), graphNodes(to)
	ids := map[string]bool{}
	for id := range a {
		ids[id] = true
	}
	for id := range b {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	for _, id := range sorted {
		na, inA := a[id]
		nb, inB := b[id]
		n := na
		if !inA {
			n = nb
		}
		r := ResourceDiff{ID: id, Stack: n.Folder, Kind: n.Kind, Type: n.Type, Label: n.Label}
		switch {
		case !inB:
			r.Status = DiffOnlyFrom
			out.Summary.OnlyFrom++
		case !inA:
			r.Status = DiffOnlyTo
			out.Summary.OnlyTo++
		default:
			r.Args = diffArgs(decodeInputs(na.Inputs), decodeInputs(nb.Inputs))
			r.Status = DiffSame
			for _, d := range r.Args {
				if d.Status == DiffUnknown {
					r.Status = DiffUnknown
					continue
				}
				r.Status = DiffDiffers
				break
			}
			switch r.Status {
			case DiffSame:
				out.Summary.Same++
			case DiffDiffers:
				out.Summary.Differs++
			default:
				out.Summary.Unknown++
			}
		}
		out.Resources = append(out.Resources, r)
	}
	if st != nil && st.Checks != nil {
		for _, m := range st.Checks.MustMatch {
			if !(m.From == fromEnv && m.To == toEnv) && !(m.From == toEnv && m.To == fromEnv) {
				continue
			}
			for _, r := range out.Resources {
				if r.Type != m.Module || r.Kind != KindModule {
					continue
				}
				name := r.Stack + " / " + r.Label
				switch r.Status {
				case DiffOnlyFrom:
					out.Violations = append(out.Violations, DiffViolation{Resource: name, Message: name + " exists only in " + fromEnv})
				case DiffOnlyTo:
					out.Violations = append(out.Violations, DiffViolation{Resource: name, Message: name + " exists only in " + toEnv})
				default:
					for _, a := range r.Args {
						for _, f := range m.Fields {
							if a.Name != f {
								continue
							}
							msg := f + " differs between " + fromEnv + " and " + toEnv
							if a.Status == DiffUnknown {
								msg = f + " cannot be compared between " + fromEnv + " and " + toEnv + " (a value is unknown)"
							}
							out.Violations = append(out.Violations, DiffViolation{Resource: name, Field: f, Message: name + ": " + msg})
						}
					}
				}
			}
		}
	}
	if st != nil {
		for _, v := range st.Variables {
			fv, tv := v.Values[fromEnv], v.Values[toEnv]
			hasF, hasT := fv != "", tv != ""
			if hasF == hasT {
				continue
			}
			d := VariableDiff{Name: v.Name, Has: "from", From: fv}
			if hasT {
				d = VariableDiff{Name: v.Name, Has: "to", To: tv}
			}
			out.Variables = append(out.Variables, d)
		}
	}
	return out
}

func diffArgs(a, b map[string]json.RawMessage) []ArgDiff {
	names := map[string]bool{}
	for k := range a {
		names[k] = true
	}
	for k := range b {
		names[k] = true
	}
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []ArgDiff
	for _, k := range keys {
		va, inA := a[k]
		vb, inB := b[k]
		switch {
		case !inB:
			out = append(out, ArgDiff{Name: k, From: va, Status: DiffOnlyFrom})
		case !inA:
			out = append(out, ArgDiff{Name: k, To: vb, Status: DiffOnlyTo})
		case hasUnknown(va) || hasUnknown(vb):
			out = append(out, ArgDiff{Name: k, From: va, To: vb, Status: DiffUnknown})
		case canonical(va) != canonical(vb):
			out = append(out, ArgDiff{Name: k, From: va, To: vb, Status: DiffDiffers})
		}
	}
	return out
}
