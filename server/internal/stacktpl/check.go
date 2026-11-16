package stacktpl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/validate"
)

// Check is the gate for a template that did not come from this project's own canvas (an imported file,
// a gallery submission): untrusted input. It runs the structural rules of the validation engine on
// the nodes (names, paths, size, one expression per $expr) and the template's own rules (name,
// slots, parameters). Every problem is listed; nil when the template is fine to store.
func Check(t Template) []string {
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	if !nameRe.MatchString(t.Name) {
		add("the name must be 1 to %d letters, digits, spaces and . _ -", MaxName)
	}
	if len(t.Nodes) == 0 || len(t.Nodes) > MaxNodes {
		add("a template holds between 1 and %d resources", MaxNodes)
		return problems
	}
	if len(t.Params) > MaxParams {
		add("at most %d parameters", MaxParams)
	}
	if len(t.Slots) > MaxNodes {
		add("too many slots")
	}
	for _, f := range validate.Structural(store.CanvasState{Nodes: t.Nodes, Edges: t.Edges}) {
		add("%s", f.Message)
	}
	instances := map[string]store.CanvasNode{}
	ids := map[string]bool{}
	for _, n := range t.Nodes {
		if n.Existing != nil {
			add("%q edits a block already in a repository; a template holds new resources only", n.InstanceName)
		}
		if n.InstanceName == "" {
			add("every resource needs a name")
		}
		instances[n.InstanceName] = n
		ids[n.ID] = true
	}
	for _, n := range t.Nodes {
		if n.ParentID != "" && !ids[n.ParentID] {
			add("%q is placed inside a resource that is not in the template", n.InstanceName)
		}
	}
	for _, e := range t.Edges {
		if !ids[e.From] || !ids[e.To] {
			add("an arrow joins a resource that is not in the template")
		}
	}
	slotNames := map[string]bool{}
	for _, s := range t.Slots {
		if s.Kind != "parent" && s.Kind != "reference" {
			add("the slot %q has an unknown kind", s.Name)
		}
		if s.Name == "" || len(s.Name) > 120 || slotNames[s.Name] {
			add("a slot needs its own name of up to 120 characters")
		}
		slotNames[s.Name] = true
		if s.Kind == "parent" {
			if _, ok := instances[s.Node]; !ok {
				add("the slot %q names a resource that is not in the template", s.Name)
			}
		}
	}
	pn := map[string]bool{}
	for _, p := range t.Params {
		if !paramRe.MatchString(p.Name) || pn[p.Name] {
			add("%q is not a usable parameter name", p.Name)
		}
		pn[p.Name] = true
		if _, ok := instances[p.Node]; !ok {
			add("the parameter %q names a resource that is not in the template", p.Name)
		}
		if len(p.Default) > 200 || strings.ContainsAny(p.Default, "\n\r") {
			add("the default of %q must be one line of at most 200 characters", p.Name)
		}
	}
	return problems
}

// Modules lists what a template uses: the module names and the raw resource types, sorted, once each.
func Modules(t Template) []string {
	seen := map[string]bool{}
	for _, n := range t.Nodes {
		switch {
		case n.Kind == "resource" && n.ResourceType != "":
			seen[n.ResourceType] = true
		case n.ModuleName != "":
			seen[n.ModuleName] = true
		}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// Missing returns "name@version" for each module call of the template that the catalog does not have.
// Raw resources are not checked here: the draft's own checks decide on them. A nil catalog finds none.
func Missing(t Template, cat validate.Catalog) []string {
	if cat == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, n := range t.Nodes {
		if n.Kind == "resource" {
			continue
		}
		key := n.ModuleName + "@" + n.ModuleVersion
		if _, ok := cat.Module(n.ModuleName, n.ModuleVersion); !ok && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// A Leak is something in a template that looks like a secret or an identity that should not be
// shared. Blocking leaks refuse the request; the others are warnings.
type Leak struct {
	Node     string `json:"node"`
	Field    string `json:"field,omitempty"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

var (
	reKeyMaterial = regexp.MustCompile(`-----BEGIN [A-Z ]*(PRIVATE KEY|CERTIFICATE|PGP)`)
	reConnString  = regexp.MustCompile(`(?i)(AccountKey|SharedAccessKey|SharedAccessSignature|Password|Pwd)\s*=\s*[^;\s]{4,}`)
	reToken       = regexp.MustCompile(`\b(AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[abprs]-[A-Za-z0-9-]{10,}|eyJ[A-Za-z0-9_-]{15,}\.[A-Za-z0-9_-]{15,}\.[A-Za-z0-9_-]{10,})`)
	reGUID        = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	reSecretName  = regexp.MustCompile(`(?i)(password|passwd|secret|token|api_?key|access_?key|private_?key|sas_?token|connection_?string|credential)`)
	reResourceID  = regexp.MustCompile(`(?i)^/subscriptions/`)
)

// Scan looks through every value of a template for key material, tokens, connection strings, a
// literal value in a field named like a secret, and subscription or tenant ids. It never returns
// the value found, only where it is. Values written as an expression ($expr) or as a project
// variable ({name}) are references, not secrets, and are skipped for the name rule.
func Scan(t Template) []Leak {
	var out []Leak
	for _, n := range t.Nodes {
		id := n.InstanceName
		if id == "" {
			id = n.ID
		}
		scanValue(id, "", n.Values, false, &out)
	}
	for _, p := range t.Params {
		if reSecretName.MatchString(p.Name) || reSecretName.MatchString(p.Field) {
			if p.Default != "" {
				out = append(out, Leak{Node: p.Node, Field: p.Field, Rule: "secret-name", Blocking: true,
					Message: fmt.Sprintf("the default of the parameter %q is a value in a field named like a secret", p.Name)})
			}
		}
		scanString(p.Node, p.Field, p.Default, &out)
	}
	return out
}

func scanValue(node, path string, v interface{}, expr bool, out *[]Leak) {
	switch x := v.(type) {
	case map[string]interface{}:
		if e, ok := x["$expr"]; ok && len(x) == 1 {
			scanValue(node, path, e, true, out)
			return
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := k
			if path != "" {
				p = path + "." + k
			}
			scanValue(node, p, x[k], false, out)
		}
	case []interface{}:
		for i, e := range x {
			scanValue(node, fmt.Sprintf("%s[%d]", path, i), e, false, out)
		}
	case string:
		if x == "" {
			return
		}
		if !expr && !placeholder.MatchString(x) {
			last := path
			if i := strings.LastIndexAny(path, ".["); i >= 0 {
				last = path[i+1:]
			}
			if reSecretName.MatchString(last) && !strings.HasPrefix(x, "var.") {
				*out = append(*out, Leak{Node: node, Field: path, Rule: "secret-name", Blocking: true,
					Message: fmt.Sprintf("%s has a literal value in a field named like a secret; use a reference to a key vault instead", path)})
			}
		}
		scanString(node, path, x, out)
	}
}

func scanString(node, field, s string, out *[]Leak) {
	switch {
	case reKeyMaterial.MatchString(s):
		*out = append(*out, Leak{Node: node, Field: field, Rule: "key-material", Blocking: true, Message: "this looks like a private key or certificate"})
	case reToken.MatchString(s):
		*out = append(*out, Leak{Node: node, Field: field, Rule: "token", Blocking: true, Message: "this looks like an access token"})
	case reConnString.MatchString(s):
		*out = append(*out, Leak{Node: node, Field: field, Rule: "connection-string", Blocking: true, Message: "this looks like a connection string with a key or a password"})
	}
	if reGUID.MatchString(s) || reResourceID.MatchString(s) {
		*out = append(*out, Leak{Node: node, Field: field, Rule: "azure-id", Blocking: false,
			Message: "this holds a subscription, tenant or resource id; a shared template should use a slot or a parameter instead"})
	}
}

// Blocking reports whether any leak must refuse the request.
func Blocking(ls []Leak) bool {
	for _, l := range ls {
		if l.Blocking {
			return true
		}
	}
	return false
}
