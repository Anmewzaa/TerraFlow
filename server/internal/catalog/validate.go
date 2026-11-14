package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Issue is one problem (or warning) found in a module schema. Path is a JSON
// pointer such as /properties/name/default so the Admin UI can point at it.
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Report is the result of validating a schema before it is imported.
type Report struct {
	// Valid is true when there are no errors; warnings do not block an import.
	Valid    bool    `json:"valid"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`

	// Filled when the schema is readable enough to identify the module.
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	Icon         string `json:"icon,omitempty"`
	TargetFolder string `json:"target_folder,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
}

var (
	moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)
	versionRe    = regexp.MustCompile(`^v\d+(\.\d+){0,2}$`)
	propNameRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	filenameRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}\.tf$`)
	colorKeyRe   = regexp.MustCompile(`^[a-z]{2,16}$`)
	providerRe   = regexp.MustCompile(`^[a-z][a-z0-9]{1,30}$`)
	categoryRe   = regexp.MustCompile(`^[a-z][a-z-]{1,30}$`)
	linkKindRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)
	outputNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,60}$`)

	jsonTypes = map[string]bool{"string": true, "number": true, "integer": true, "boolean": true, "object": true, "array": true, "null": true}

	// Widgets the form engine knows. An unknown widget is a warning, not an
	// error: the form falls back to the plain input for the type.
	knownWidgets = map[string]bool{
		"pick-existing": true, "locked": true, "nullable-object": true, "key-value-map": true,
		"list-of-strings": true, "list-of-objects": true, "nested-map-of-objects": true, "from-parent": true,
	}

	// Keywords the form engine cannot render. Rejecting them at import is better
	// than showing a broken form later.
	unsupportedKeywords = []string{"$ref", "oneOf", "anyOf", "allOf", "not", "if", "patternProperties", "dependentSchemas"}
)

const maxDepth = 8

type validator struct {
	r      *Report
	groups map[string]bool
}

func (v *validator) err(path, code, format string, a ...any) {
	v.r.Errors = append(v.r.Errors, Issue{Path: path, Code: code, Message: fmt.Sprintf(format, a...)})
}

func (v *validator) warn(path, code, format string, a ...any) {
	v.r.Warnings = append(v.r.Warnings, Issue{Path: path, Code: code, Message: fmt.Sprintf(format, a...)})
}

// ValidateName reports whether name can identify a module.
func ValidateName(name string) bool { return moduleNameRe.MatchString(name) }

// ValidColorKey reports whether s looks like a colour key chosen in the UI.
func ValidColorKey(s string) bool { return colorKeyRe.MatchString(s) }

// Validate checks a module schema the way the form engine and the generator
// will use it. name is the catalog name the schema would be imported under.
func Validate(name string, raw []byte) (r Report) {
	r = Report{Errors: []Issue{}, Warnings: []Issue{}, Name: name}
	v := &validator{r: &r, groups: map[string]bool{}}
	defer func() {
		r.Valid = len(r.Errors) == 0
		sort.SliceStable(r.Errors, func(i, j int) bool { return r.Errors[i].Path < r.Errors[j].Path })
		sort.SliceStable(r.Warnings, func(i, j int) bool { return r.Warnings[i].Path < r.Warnings[j].Path })
	}()

	if !moduleNameRe.MatchString(name) {
		v.err("/name", "bad_name", "module name must be 2-63 characters: lowercase letters, digits and '-', starting with a letter")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		v.err("", "invalid_json", "schema is not a valid JSON object: %v", err)
		return r
	}
	if dec.More() {
		v.err("", "invalid_json", "schema has extra content after the JSON object")
		return r
	}
	r.Checksum = fmt.Sprintf("%x", sha256.Sum256(raw))

	if t, ok := root["type"].(string); !ok || t != "object" {
		v.err("/type", "bad_type", `the top level must have "type": "object"`)
	}
	r.Title, _ = root["title"].(string)
	r.Description, _ = root["description"].(string)
	if strings.TrimSpace(r.Title) == "" {
		v.err("/title", "missing", "title is required: it is the name shown in the catalog")
	}
	if r.Description == "" {
		v.warn("/description", "missing", "a description helps people pick the right module")
	}
	v.checkUnsupported("", root)
	v.checkMeta(root)
	v.checkUIGroups(root)
	v.checkLinks(root)

	props, ok := root["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		v.err("/properties", "missing", "properties is required and must list at least one input")
		return r
	}
	v.checkProperties("/properties", props, 0, true)
	v.checkRequired("", root, props)
	return r
}

func (v *validator) checkMeta(root map[string]any) {
	meta, ok := root["x-tf-module"].(map[string]any)
	if !ok {
		v.err("/x-tf-module", "missing", "x-tf-module is required: it tells the generator where the module comes from")
		return
	}
	str := func(key string) string {
		s, _ := meta[key].(string)
		return strings.TrimSpace(s)
	}
	if src := str("source"); src == "" {
		v.err("/x-tf-module/source", "missing", "source is required (the module path or address used in the generated source line)")
	} else if strings.ContainsAny(src, " \t\r\n\"\\") || strings.Contains(src, "..") {
		v.err("/x-tf-module/source", "bad_value", "source must not contain spaces, quotes, backslashes or '..'")
	}
	ver := str("version")
	switch {
	case ver == "":
		v.err("/x-tf-module/version", "missing", "version is required")
	case !versionRe.MatchString(ver):
		v.err("/x-tf-module/version", "bad_value", "version %q must look like v1, v1.2 or v1.2.3", ver)
	}
	v.r.Version = ver
	folder := str("target_folder")
	switch {
	case folder == "":
		v.err("/x-tf-module/target_folder", "missing", "target_folder is required: the folder new resources are written to")
	case strings.Contains(folder, "..") || strings.ContainsAny(folder, `\:/`):
		v.err("/x-tf-module/target_folder", "bad_value", "target_folder must be one plain folder name")
	}
	v.r.TargetFolder = folder
	if fn := str("filename"); fn != "" && !filenameRe.MatchString(fn) {
		v.err("/x-tf-module/filename", "bad_value", "filename %q must end in .tf and use letters, digits, - _ .", fn)
	}
	v.r.Icon = str("icon")
	// provider is optional: a module may be written for one provider (azurerm), or for none. It
	// only groups and filters the catalog; the generator does not read it (D-058).
	if p := str("provider"); p != "" && !providerRe.MatchString(p) {
		v.err("/x-tf-module/provider", "bad_value", "provider %q must be a provider id such as azurerm (lowercase letters and digits)", p)
	}
	if canvas, ok := root["x-tf-canvas"].(map[string]any); ok {
		if c, _ := canvas["category"].(string); c != "" && !categoryRe.MatchString(c) {
			v.err("/x-tf-canvas/category", "bad_value", "category %q must be a lowercase word such as compute, network or storage", c)
		}
	}
}

func (v *validator) checkUIGroups(root map[string]any) {
	ui, ok := root["x-tf-ui"].(map[string]any)
	if !ok {
		return
	}
	list, ok := ui["groups"].([]any)
	if !ok {
		return
	}
	for i, g := range list {
		path := fmt.Sprintf("/x-tf-ui/groups/%d", i)
		gm, ok := g.(map[string]any)
		id, _ := gm["id"].(string)
		if !ok || id == "" {
			v.err(path, "missing", "each group needs an id")
			continue
		}
		if v.groups[id] {
			v.err(path+"/id", "duplicate", "group id %q is used twice", id)
		}
		v.groups[id] = true
		if label, _ := gm["label"].(string); label == "" {
			v.warn(path+"/label", "missing", "group %q has no label", id)
		}
	}
}

func (v *validator) checkUnsupported(path string, m map[string]any) {
	for _, k := range unsupportedKeywords {
		if _, ok := m[k]; ok {
			v.err(path+"/"+k, "unsupported", "%q is not supported by the form engine", k)
		}
	}
}

func (v *validator) checkRequired(path string, m map[string]any, props map[string]any) {
	raw, ok := m["required"]
	if !ok {
		return
	}
	list, ok := raw.([]any)
	if !ok {
		v.err(path+"/required", "bad_type", "required must be a list of input names")
		return
	}
	seen := map[string]bool{}
	for i, item := range list {
		name, ok := item.(string)
		p := fmt.Sprintf("%s/required/%d", path, i)
		switch {
		case !ok:
			v.err(p, "bad_type", "required entries must be strings")
		case seen[name]:
			v.err(p, "duplicate", "%q is listed as required twice", name)
		case props[name] == nil:
			v.err(p, "unknown_ref", "%q is required but is not an input", name)
		}
		seen[name] = true
	}
}

func (v *validator) checkProperties(path string, props map[string]any, depth int, top bool) {
	if depth > maxDepth {
		v.err(path, "too_deep", "inputs are nested more than %d levels deep", maxDepth)
		return
	}
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := path + "/" + name
		if !propNameRe.MatchString(name) {
			v.err(p, "bad_name", "input name %q must start with a letter or _ and use letters, digits, _ and -", name)
		}
		sch, ok := props[name].(map[string]any)
		if !ok {
			v.err(p, "bad_type", "input %q must be an object", name)
			continue
		}
		v.checkSchema(p, sch, depth, top)
	}
}

func typesOf(sch map[string]any) ([]string, bool) {
	switch t := sch["type"].(type) {
	case string:
		return []string{t}, true
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s, ok := x.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, len(out) > 0
	}
	return nil, false
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// valueFits reports whether a decoded JSON value is allowed by the types.
func valueFits(types []string, val any) bool {
	switch x := val.(type) {
	case nil:
		return has(types, "null")
	case bool:
		return has(types, "boolean")
	case string:
		return has(types, "string")
	case json.Number:
		if has(types, "number") {
			return true
		}
		if has(types, "integer") {
			_, err := x.Int64()
			return err == nil
		}
		return false
	case []any:
		return has(types, "array")
	case map[string]any:
		return has(types, "object")
	}
	return false
}

func (v *validator) checkSchema(path string, sch map[string]any, depth int, top bool) {
	v.checkUnsupported(path, sch)

	types, ok := typesOf(sch)
	if !ok {
		v.err(path+"/type", "missing", `every input needs a "type"`)
	}
	for _, t := range types {
		if !jsonTypes[t] {
			v.err(path+"/type", "bad_type", "unknown type %q", t)
		}
	}
	if top {
		if s, _ := sch["title"].(string); s == "" {
			v.warn(path+"/title", "missing", "input has no title, so the form shows its raw name")
		}
		if s, _ := sch["description"].(string); s == "" {
			v.warn(path+"/description", "missing", "input has no description")
		}
	}

	if def, has := sch["default"]; has && ok && !valueFits(types, def) {
		v.err(path+"/default", "bad_default", "default %s does not match type %s", compact(def), strings.Join(types, "|"))
	}
	if raw, has := sch["enum"]; has {
		v.checkEnum(path, sch, raw, types, ok)
	}
	if p, has := sch["pattern"]; has {
		ps, isStr := p.(string)
		if !isStr || !ok || !hasAny(types, "string") {
			v.err(path+"/pattern", "bad_type", "pattern only applies to string inputs")
		} else if _, err := regexp.Compile(ps); err != nil {
			v.err(path+"/pattern", "bad_pattern", "pattern is not a valid expression: %v", err)
		}
	}
	v.checkRange(path, sch, "minimum", "maximum", false)
	v.checkRange(path, sch, "minLength", "maxLength", true)
	v.checkRange(path, sch, "minItems", "maxItems", true)

	if ui, has := sch["x-tf-ui"]; has {
		v.checkFieldUI(path+"/x-tf-ui", ui)
	}

	if ok && hasAny(types, "array") {
		switch items := sch["items"].(type) {
		case nil:
			v.warn(path+"/items", "missing", "array input has no items schema")
		case map[string]any:
			v.checkSchema(path+"/items", items, depth+1, false)
		default:
			v.err(path+"/items", "bad_type", "items must be a schema object")
		}
	}
	if sub, has := sch["properties"]; has {
		pm, isMap := sub.(map[string]any)
		switch {
		case !isMap:
			v.err(path+"/properties", "bad_type", "properties must be an object")
		case ok && !hasAny(types, "object"):
			v.err(path+"/properties", "bad_type", "properties only applies to object inputs")
		default:
			v.checkProperties(path+"/properties", pm, depth+1, false)
			v.checkRequired(path, sch, pm)
		}
	}
	if ap, has := sch["additionalProperties"]; has {
		if m, isMap := ap.(map[string]any); isMap {
			v.checkSchema(path+"/additionalProperties", m, depth+1, false)
		}
	}
}

func hasAny(types []string, want string) bool { return has(types, want) }

func compact(val any) string {
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprint(val)
	}
	return string(b)
}

func (v *validator) checkEnum(path string, sch map[string]any, raw any, types []string, typed bool) {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		v.err(path+"/enum", "bad_enum", "enum must be a non-empty list")
		return
	}
	seen := map[string]bool{}
	for i, item := range list {
		p := fmt.Sprintf("%s/enum/%d", path, i)
		if typed && !valueFits(types, item) {
			v.err(p, "bad_enum", "enum value %s does not match type %s", compact(item), strings.Join(types, "|"))
		}
		k := compact(item)
		if seen[k] {
			v.err(p, "duplicate", "enum value %s is listed twice", k)
		}
		seen[k] = true
	}
	if def, has := sch["default"]; has && def != nil && !seen[compact(def)] {
		v.err(path+"/default", "bad_default", "default %s is not one of the enum values", compact(def))
	}
}

func (v *validator) checkRange(path string, sch map[string]any, lo, hi string, integer bool) {
	get := func(key string) (float64, bool) {
		raw, has := sch[key]
		if !has {
			return 0, false
		}
		n, ok := raw.(json.Number)
		if !ok {
			v.err(path+"/"+key, "bad_type", "%s must be a number", key)
			return 0, false
		}
		f, err := n.Float64()
		if err != nil {
			v.err(path+"/"+key, "bad_type", "%s must be a number", key)
			return 0, false
		}
		if integer && (f < 0 || f != float64(int64(f))) {
			v.err(path+"/"+key, "bad_value", "%s must be a whole number, 0 or more", key)
			return 0, false
		}
		return f, true
	}
	l, lok := get(lo)
	h, hok := get(hi)
	if lok && hok && l > h {
		v.err(path+"/"+lo, "bad_range", "%s (%v) is greater than %s (%v)", lo, l, hi, h)
	}
}

func (v *validator) checkFieldUI(path string, raw any) {
	ui, ok := raw.(map[string]any)
	if !ok {
		v.err(path, "bad_type", "x-tf-ui must be an object")
		return
	}
	if g, has := ui["group"]; has {
		gs, isStr := g.(string)
		switch {
		case !isStr:
			v.err(path+"/group", "bad_type", "group must be a string")
		case len(v.groups) > 0 && !v.groups[gs]:
			v.err(path+"/group", "unknown_ref", "group %q is not defined in x-tf-ui.groups", gs)
		}
	}
	if o, has := ui["order"]; has {
		if _, isNum := o.(json.Number); !isNum {
			v.err(path+"/order", "bad_type", "order must be a number")
		}
	}
	w, has := ui["widget"]
	if !has {
		return
	}
	ws, isStr := w.(string)
	if !isStr {
		v.err(path+"/widget", "bad_type", "widget must be a string")
		return
	}
	if !knownWidgets[ws] {
		v.warn(path+"/widget", "unknown_widget", "widget %q is not known to the form engine; the plain input is used", ws)
	}
	switch ws {
	case "pick-existing":
		pick, _ := ui["pick"].(map[string]any)
		mt, _ := pick["module_type"].(string)
		lk, _ := pick["link"].(string)
		switch {
		case mt == "" && lk == "":
			v.err(path+"/pick/module_type", "missing", "a pick-existing input needs pick.module_type or pick.link")
		case mt != "" && lk != "":
			v.err(path+"/pick", "bad_value", "a pick-existing input takes pick.module_type or pick.link, not both")
		}
		if lk != "" && !linkKindRe.MatchString(lk) {
			v.err(path+"/pick/link", "bad_value", "link %q must be a lowercase word such as private_link", lk)
		}
		if s, _ := pick["output"].(string); s == "" {
			v.err(path+"/pick/output", "missing", "a pick-existing input needs pick.output")
		}
		if f, has := pick["fills"]; has {
			if s, _ := f.(string); !outputNameRe.MatchString(s) || lk == "" {
				v.err(path+"/pick/fills", "bad_value", "pick.fills names one other input, and is only used with pick.link")
			}
		}
	case "locked":
		if s, _ := ui["locked_reason"].(string); s == "" {
			v.warn(path+"/locked_reason", "missing", "a locked input should say why it is locked")
		}
	}
}

// checkLinks validates x-tf-link: what the module offers to the inputs that ask for a link (8.11).
func (v *validator) checkLinks(root map[string]any) {
	link, ok := root["x-tf-link"].(map[string]any)
	if !ok {
		return
	}
	offers, _ := link["offers"].([]any)
	if len(offers) > 8 {
		v.err("/x-tf-link/offers", "too_many", "a module offers at most 8 links")
	}
	for i, o := range offers {
		path := fmt.Sprintf("/x-tf-link/offers/%d", i)
		m, ok := o.(map[string]any)
		if !ok {
			v.err(path, "bad_type", "an offer is an object with kind and output")
			continue
		}
		if k, _ := m["kind"].(string); !linkKindRe.MatchString(k) {
			v.err(path+"/kind", "bad_value", "kind must be a lowercase word such as private_link")
		}
		if s, _ := m["output"].(string); !outputNameRe.MatchString(s) {
			v.err(path+"/output", "bad_value", "output must name an output of the module, such as id")
		}
		if s, has := m["subresource"]; has {
			if str, _ := s.(string); str == "" || len(str) > 60 {
				v.err(path+"/subresource", "bad_value", "subresource must be a short name such as vault or blob")
			}
		}
	}
}
