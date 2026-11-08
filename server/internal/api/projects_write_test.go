package api

import (
	"path/filepath"
	"testing"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"My Project 1":  "my-project-1",
		"  Dev/Prod!! ": "dev-prod",
		"!!!":           "",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q)=%q want %q", in, got, want)
		}
	}
}

func TestWithin(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "a", "ws")
	if !within(root, filepath.Join(root, "p")) || !within(root, root) {
		t.Error("expected inside")
	}
	if within(root, filepath.Join(string(filepath.Separator), "a", "other")) ||
		within(root, filepath.Join(root, "..", "x")) {
		t.Error("expected outside")
	}
}
