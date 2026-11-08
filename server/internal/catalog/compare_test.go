package catalog

import (
	"strings"
	"testing"
)

func change(list []Change, path, kind string) *Change {
	for i := range list {
		if list[i].Path == path && list[i].Kind == kind {
			return &list[i]
		}
	}
	return nil
}

func TestCompareIdenticalSchemasHaveNoChanges(t *testing.T) {
	if c := Compare([]byte(good), []byte(good)); len(c) != 0 {
		t.Fatalf("got %+v", c)
	}
}

func TestCompareClassifiesBreakingChanges(t *testing.T) {
	old := []byte(good)
	type tc struct {
		name           string
		from, to       string
		path, kind     string
		breaking       bool
		detailContains string
	}
	for _, c := range []tc{
		{"input removed", `"count": {"type": "integer", "title": "Count", "description": "d", "default": 3, "minimum": 1, "maximum": 9},`, ``, "/properties/count", "removed", true, "removed"},
		{"optional input added", `"tier":`, `"extra": {"type": "string"}, "tier":`, "/properties/extra", "added", false, "added"},
		{"required input added", `"required": ["name"],
  "properties": {`, `"required": ["name", "extra"],
  "properties": {"extra": {"type": "string"},`, "/properties/extra", "added", true, "required"},
		{"became required", `"required": ["name"],
  "properties"`, `"required": ["name", "tier"],
  "properties"`, "/properties/tier", "changed", true, "required"},
		{"stopped being required", `"required": ["name"],
  "properties"`, `"required": [],
  "properties"`, "/properties/name", "changed", false, "required"},
		{"type changed", `"count": {"type": "integer"`, `"count": {"type": "string"`, "/properties/count/type", "changed", true, "type"},
		{"default changed", `"default": 3`, `"default": 4`, "/properties/count/default", "changed", false, "default"},
		{"enum value dropped", `"enum": ["a", "b"]`, `"enum": ["a"]`, "/properties/tier/enum", "changed", true, "removed"},
		{"enum value added", `"enum": ["a", "b"]`, `"enum": ["a", "b", "c"]`, "/properties/tier/enum", "changed", false, "added"},
		{"nested input removed", `"properties": {"subnet_id": {"type": "string"}}, "required": ["subnet_id"]`, `"properties": {}`, "/properties/pe/properties/subnet_id", "removed", true, "removed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(good, c.from) {
				t.Fatalf("test bug: %q not in base", c.from)
			}
			got := Compare(old, []byte(strings.Replace(good, c.from, c.to, 1)))
			ch := change(got, c.path, c.kind)
			if ch == nil {
				t.Fatalf("want %s %s, got %+v", c.kind, c.path, got)
			}
			if ch.Breaking != c.breaking || !strings.Contains(ch.Detail, c.detailContains) {
				t.Fatalf("%+v: want breaking=%v detail~%q", *ch, c.breaking, c.detailContains)
			}
		})
	}
}

func TestCompareToleratesBrokenInput(t *testing.T) {
	if c := Compare([]byte(`{`), []byte(good)); c == nil || len(c) != 0 {
		t.Fatalf("unreadable schemas give an empty (non-nil) list, got %#v", c)
	}
}
