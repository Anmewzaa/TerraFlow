package catalog

import "testing"

func TestVersionFromSource(t *testing.T) {
	for src, want := range map[string]string{
		"git::https://h/x.git//modules/rg?ref=v1.1.0":     "v1.1.0",
		"git::https://h/x.git//modules/rg?ref=v2&depth=1": "v2",
		"git::https://h/x.git//modules/rg":                "",
		"":                                                "",
		"../modules/rg":                                   "",
	} {
		if got := VersionFromSource(src); got != want {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
}

func TestResolveVersion(t *testing.T) {
	versions := []string{"v1", "v1.1.0", "v2"}
	for name, c := range map[string]struct{ wanted, want string }{
		"the same version":           {"v1.1.0", "v1.1.0"},
		"newest of the same major":   {"v1.2.0", "v1.1.0"},
		"a bare major":               {"v2", "v2"},
		"unknown major uses latest":  {"v9.0.0", "v2"},
		"nothing pinned uses latest": {"", "v2"},
	} {
		if got := ResolveVersion(c.wanted, versions, "v2"); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
	if got := ResolveVersion("v1.2.0", nil, ""); got != "v1.2.0" {
		t.Errorf("without a catalog the wanted version stays: %q", got)
	}
}
