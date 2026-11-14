package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// good is a small schema that must validate with no errors.
const good = `{
  "title": "Thing",
  "description": "A thing.",
  "type": "object",
  "x-tf-module": {"source": "thing", "version": "v1", "target_folder": "things", "filename": "thing.tf", "icon": "Thing"},
  "x-tf-ui": {"groups": [{"id": "basics", "label": "Basics"}]},
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "title": "Name", "description": "The name.", "pattern": "^[a-z]+$", "minLength": 2, "maxLength": 10,
             "x-tf-ui": {"group": "basics", "order": 1}},
    "tier": {"type": "string", "title": "Tier", "description": "d", "enum": ["a", "b"], "default": "a"},
    "count": {"type": "integer", "title": "Count", "description": "d", "default": 3, "minimum": 1, "maximum": 9},
    "tags": {"type": "object", "title": "Tags", "description": "d", "default": {}, "x-tf-ui": {"widget": "key-value-map"}},
    "rg": {"type": "string", "title": "RG", "description": "d",
           "x-tf-ui": {"widget": "pick-existing", "pick": {"module_type": "resource-group", "output": "name"}}},
    "pe": {"type": ["object", "null"], "title": "PE", "description": "d", "default": null,
           "properties": {"subnet_id": {"type": "string"}}, "required": ["subnet_id"]},
    "zones": {"type": "array", "title": "Zones", "description": "d", "items": {"type": "string"}, "default": ["1"]}
  }
}`

// mutate replaces one piece of the good schema.
func mutate(t *testing.T, from, to string) string {
	t.Helper()
	if !strings.Contains(good, from) {
		t.Fatalf("test bug: %q not in the base schema", from)
	}
	return strings.Replace(good, from, to, 1)
}

func codes(issues []Issue) string {
	var out []string
	for _, i := range issues {
		out = append(out, i.Code+"@"+i.Path)
	}
	return strings.Join(out, " ")
}

func TestValidateAcceptsAGoodSchema(t *testing.T) {
	r := Validate("thing", []byte(good))
	if !r.Valid || len(r.Errors) != 0 || len(r.Warnings) != 0 {
		t.Fatalf("errors=%v warnings=%v", codes(r.Errors), codes(r.Warnings))
	}
	if r.Name != "thing" || r.Version != "v1" || r.Title != "Thing" || r.TargetFolder != "things" || r.Icon != "Thing" || r.Checksum == "" {
		t.Fatalf("module info not filled: %+v", r)
	}
}

func TestValidateRejectsEachMistake(t *testing.T) {
	cases := []struct {
		name   string
		schema string
		want   string // "code@path" that must be among the errors
	}{
		{"not json", `{"title":`, "invalid_json@"},
		{"array root", `[]`, "invalid_json@"},
		{"trailing data", good + ` {}`, "invalid_json@"},
		{"wrong root type", mutate(t, `"type": "object",
  "x-tf-module"`, `"type": "array",
  "x-tf-module"`), "bad_type@/type"},
		{"no title", mutate(t, `"title": "Thing",`, ``), "missing@/title"},
		{"blank title", mutate(t, `"title": "Thing",`, `"title": "  ",`), "missing@/title"},
		{"no module meta", mutate(t, `"x-tf-module": {"source": "thing", "version": "v1", "target_folder": "things", "filename": "thing.tf", "icon": "Thing"},`, ``), "missing@/x-tf-module"},
		{"no source", mutate(t, `"source": "thing", `, ``), "missing@/x-tf-module/source"},
		{"source with space", mutate(t, `"source": "thing"`, `"source": "a b"`), "bad_value@/x-tf-module/source"},
		{"source with ..", mutate(t, `"source": "thing"`, `"source": "../x"`), "bad_value@/x-tf-module/source"},
		{"no version", mutate(t, `"version": "v1", `, ``), "missing@/x-tf-module/version"},
		{"bad version", mutate(t, `"version": "v1"`, `"version": "1.0"`), "bad_value@/x-tf-module/version"},
		{"no folder", mutate(t, `"target_folder": "things", `, ``), "missing@/x-tf-module/target_folder"},
		{"folder traversal", mutate(t, `"target_folder": "things"`, `"target_folder": "../x"`), "bad_value@/x-tf-module/target_folder"},
		{"nested folder", mutate(t, `"target_folder": "things"`, `"target_folder": "a/b"`), "bad_value@/x-tf-module/target_folder"},
		{"bad filename", mutate(t, `"filename": "thing.tf"`, `"filename": "thing.txt"`), "bad_value@/x-tf-module/filename"},
		{"filename traversal", mutate(t, `"filename": "thing.tf"`, `"filename": "../thing.tf"`), "bad_value@/x-tf-module/filename"},
		{"no properties", `{"title":"T","type":"object","x-tf-module":{"source":"s","version":"v1","target_folder":"f"}}`, "missing@/properties"},
		{"empty properties", `{"title":"T","type":"object","x-tf-module":{"source":"s","version":"v1","target_folder":"f"},"properties":{}}`, "missing@/properties"},
		{"input without type", mutate(t, `"tier": {"type": "string",`, `"tier": {`), "missing@/properties/tier/type"},
		{"unknown type", mutate(t, `"count": {"type": "integer"`, `"count": {"type": "int"`), "bad_type@/properties/count/type"},
		{"bad input name", mutate(t, `"tier":`, `"1tier":`), "bad_name@/properties/1tier"},
		{"input not object", mutate(t, `"tier": {"type": "string", "title": "Tier", "description": "d", "enum": ["a", "b"], "default": "a"}`, `"tier": "string"`), "bad_type@/properties/tier"},
		{"default wrong type", mutate(t, `"default": 3`, `"default": "three"`), "bad_default@/properties/count/default"},
		{"default fractional integer", mutate(t, `"default": 3`, `"default": 3.5`), "bad_default@/properties/count/default"},
		{"null default without null type", mutate(t, `"default": "a"`, `"default": null`), "bad_default@/properties/tier/default"},
		{"default not in enum", mutate(t, `"default": "a"`, `"default": "z"`), "bad_default@/properties/tier/default"},
		{"empty enum", mutate(t, `"enum": ["a", "b"]`, `"enum": []`), "bad_enum@/properties/tier/enum"},
		{"enum wrong type", mutate(t, `"enum": ["a", "b"]`, `"enum": ["a", 2]`), "bad_enum@/properties/tier/enum/1"},
		{"enum duplicate", mutate(t, `"enum": ["a", "b"]`, `"enum": ["a", "a"]`), "duplicate@/properties/tier/enum/1"},
		{"pattern not regex", mutate(t, `"^[a-z]+$"`, `"[a-"`), "bad_pattern@/properties/name/pattern"},
		{"pattern on number", mutate(t, `"count": {"type": "integer",`, `"count": {"type": "integer", "pattern": "^1$",`), "bad_type@/properties/count/pattern"},
		{"min greater than max", mutate(t, `"minimum": 1, "maximum": 9`, `"minimum": 9, "maximum": 1`), "bad_range@/properties/count/minimum"},
		{"minLength negative", mutate(t, `"minLength": 2`, `"minLength": -1`), "bad_value@/properties/name/minLength"},
		{"maxLength fractional", mutate(t, `"maxLength": 10`, `"maxLength": 1.5`), "bad_value@/properties/name/maxLength"},
		{"required unknown input", mutate(t, `"required": ["name"],
  "properties"`, `"required": ["name", "ghost"],
  "properties"`), "unknown_ref@/required/1"},
		{"required twice", mutate(t, `"required": ["name"],
  "properties"`, `"required": ["name", "name"],
  "properties"`), "duplicate@/required/1"},
		{"required not a list", mutate(t, `"required": ["name"],
  "properties"`, `"required": "name",
  "properties"`), "bad_type@/required"},
		{"nested required unknown", mutate(t, `"required": ["subnet_id"]`, `"required": ["nope"]`), "unknown_ref@/properties/pe/required/0"},
		{"properties on a string", mutate(t, `"tier": {"type": "string",`, `"tier": {"type": "string", "properties": {"x": {"type": "string"}},`), "bad_type@/properties/tier/properties"},
		{"$ref", mutate(t, `"tier": {"type": "string",`, `"tier": {"$ref": "#/x", "type": "string",`), "unsupported@/properties/tier/$ref"},
		{"oneOf", mutate(t, `"tier": {"type": "string",`, `"tier": {"oneOf": [], "type": "string",`), "unsupported@/properties/tier/oneOf"},
		{"unknown group", mutate(t, `"group": "basics"`, `"group": "nope"`), "unknown_ref@/properties/name/x-tf-ui/group"},
		{"duplicate group", mutate(t, `{"id": "basics", "label": "Basics"}`, `{"id": "basics", "label": "A"}, {"id": "basics", "label": "B"}`), "duplicate@/x-tf-ui/groups/1/id"},
		{"group without id", mutate(t, `{"id": "basics", "label": "Basics"}`, `{"label": "Basics"}`), "missing@/x-tf-ui/groups/0"},
		{"order not a number", mutate(t, `"order": 1`, `"order": "first"`), "bad_type@/properties/name/x-tf-ui/order"},
		{"pick without module_type", mutate(t, `"module_type": "resource-group", `, ``), "missing@/properties/rg/x-tf-ui/pick/module_type"},
		{"pick without output", mutate(t, `, "output": "name"`, ``), "missing@/properties/rg/x-tf-ui/pick/output"},
		{"x-tf-ui not an object", mutate(t, `"x-tf-ui": {"widget": "key-value-map"}`, `"x-tf-ui": "map"`), "bad_type@/properties/tags/x-tf-ui"},
		{"pick with both a module type and a link", mutate(t, `"module_type": "resource-group"`, `"module_type": "resource-group", "link": "private_link"`), "bad_value@/properties/rg/x-tf-ui/pick"},
		{"pick fills without a link", mutate(t, `"output": "name"`, `"output": "name", "fills": "other"`), "bad_value@/properties/rg/x-tf-ui/pick/fills"},
		{"an offer with a bad kind", mutate(t, `"x-tf-module"`, `"x-tf-link": {"offers": [{"kind": "Bad Kind", "output": "id"}]}, "x-tf-module"`), "bad_value@/x-tf-link/offers/0/kind"},
		{"bad module name", good, "bad_name@/name"}, // checked with a bad name below
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := "thing"
			if tc.name == "bad module name" {
				name = "Bad Name"
			}
			r := Validate(name, []byte(tc.schema))
			if r.Valid {
				t.Fatalf("schema was accepted; want an error %s", tc.want)
			}
			if !strings.Contains(codes(r.Errors)+" ", tc.want+" ") {
				t.Fatalf("want error %s, got %s", tc.want, codes(r.Errors))
			}
		})
	}
}

func TestValidateModuleNames(t *testing.T) {
	for name, ok := range map[string]bool{
		"thing": true, "key-vault": true, "a1": true,
		"": false, "a": false, "Thing": false, "1thing": false, "my_thing": false, "a b": false, strings.Repeat("a", 64): false,
	} {
		if got := Validate(name, []byte(good)).Valid; got != ok {
			t.Errorf("name %q: valid=%v, want %v", name, got, ok)
		}
	}
}

func TestValidateWarnsButStillAccepts(t *testing.T) {
	schema := strings.NewReplacer(
		`"description": "A thing.",`, ``,
		`"x-tf-ui": {"widget": "key-value-map"}`, `"x-tf-ui": {"widget": "rainbow"}`,
		`"zones": {"type": "array", "title": "Zones", "description": "d", "items": {"type": "string"}, "default": ["1"]}`,
		`"zones": {"type": "array", "title": "Zones", "description": "d", "default": ["1"]}`,
	).Replace(good)
	schema = strings.Replace(schema, `"name": {"type": "string", "title": "Name", "description": "The name.",`, `"name": {"type": "string",`, 1)
	r := Validate("thing", []byte(schema))
	if !r.Valid {
		t.Fatalf("warnings must not block an import: %v", codes(r.Errors))
	}
	got := codes(r.Warnings)
	for _, want := range []string{"missing@/description", "unknown_widget@/properties/tags/x-tf-ui/widget", "missing@/properties/zones/items", "missing@/properties/name/title"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing warning %s in %s", want, got)
		}
	}
}

func TestValidateLockedWithoutReasonWarns(t *testing.T) {
	s := mutate(t, `"tier": {"type": "string",`, `"tier": {"x-tf-ui": {"widget": "locked"}, "type": "string",`)
	r := Validate("thing", []byte(s))
	if !r.Valid || !strings.Contains(codes(r.Warnings), "missing@/properties/tier/x-tf-ui/locked_reason") {
		t.Fatalf("valid=%v warnings=%s", r.Valid, codes(r.Warnings))
	}
}

func TestValidateStopsAtDeepNesting(t *testing.T) {
	inner := `{"type":"string"}`
	for i := 0; i < 12; i++ {
		inner = `{"type":"object","properties":{"x":` + inner + `}}`
	}
	s := `{"title":"T","type":"object","x-tf-module":{"source":"s","version":"v1","target_folder":"f"},"properties":{"x":` + inner + `}}`
	if r := Validate("thing", []byte(s)); r.Valid || !strings.Contains(codes(r.Errors), "too_deep") {
		t.Fatalf("deep nesting must be refused: %s", codes(r.Errors))
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	s := mutate(t, `"default": 3`, `"default": "x"`)
	s = strings.Replace(s, `"version": "v1"`, `"version": "x"`, 1)
	r := Validate("Bad", []byte(s))
	if len(r.Errors) < 3 {
		t.Fatalf("an admin should see all problems in one go, got %s", codes(r.Errors))
	}
	// Output is ordered by path so it is stable between runs.
	for i := 1; i < len(r.Errors); i++ {
		if r.Errors[i-1].Path > r.Errors[i].Path {
			t.Fatalf("errors not sorted: %s", codes(r.Errors))
		}
	}
}

func TestChecksumIsOfTheExactBytes(t *testing.T) {
	a := Validate("thing", []byte(good))
	b := Validate("thing", []byte(good+"\n"))
	if a.Checksum == b.Checksum || a.Checksum != Validate("thing", []byte(good)).Checksum {
		t.Fatal("the checksum identifies the exact uploaded bytes")
	}
}

// Every schema that ships with the app must pass the same check an admin's
// import has to pass; otherwise the validator is stricter than the product.
func TestShippedSchemasAreValid(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "catalog", "modules")
	files, err := filepath.Glob(filepath.Join(dir, "*", "*", "schema.json"))
	if err != nil || len(files) < 10 {
		t.Fatalf("found %d schemas in %s: %v", len(files), dir, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(filepath.Dir(filepath.Dir(f)))
		r := Validate(name, raw)
		if !r.Valid {
			t.Errorf("%s: %s", f, codes(r.Errors))
		}
		if want := filepath.Base(filepath.Dir(f)); r.Version != want {
			t.Errorf("%s: x-tf-module.version %q does not match its folder %q", f, r.Version, want)
		}
	}
}

func TestCategoryOfDomainGroupsOlderModules(t *testing.T) {
	for domain, want := range map[string]string{"network": "network", "data": "storage", "compute": "compute", "security": "security", "neutral": "other", "": "other", "unheard-of": "other"} {
		if got := CategoryOfDomain(domain); got != want {
			t.Errorf("%q -> %q, want %q", domain, got, want)
		}
	}
}

func TestAnOfferAndALinkPickAreAccepted(t *testing.T) {
	schema := mutate(t, `"widget": "pick-existing", "pick": {"module_type": "resource-group", "output": "name"}`,
		`"widget": "pick-existing", "pick": {"link": "private_link", "output": "id", "fills": "subnets"}`)
	r := Validate("thing", []byte(schema))
	if !r.Valid {
		t.Fatalf("a link pick must be valid: %s", codes(r.Errors))
	}
}
