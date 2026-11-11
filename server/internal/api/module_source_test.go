package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
)

func TestModuleSourceComesFromTheCatalogOrTheBase(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r := &sourceResolver{base: "git::https://example.invalid/mods.git//modules", now: func() time.Time { return now }}
	mod := func(name, src string) catalog.Module {
		b, _ := json.Marshal(map[string]any{"x-tf-module": map[string]string{"source": src}})
		return catalog.Module{Name: name, Version: "v1", SchemaJSON: b}
	}
	calls := 0
	r.list = func(context.Context, string) ([]catalog.Module, error) {
		calls++
		return []catalog.Module{mod("a", "git::https://example.invalid/other.git//x/a"), mod("b", "b")}, nil
	}
	if got := r.build("", "a", "v1"); got != "git::https://example.invalid/other.git//x/a?ref=v1" {
		t.Errorf("a catalog source wins: %s", got)
	}
	if got := r.build("", "b", "v1"); got != "git::https://example.invalid/mods.git//modules/b?ref=v1" {
		t.Errorf("a bare name goes under the base: %s", got)
	}
	if got := r.build("", "c", "v2"); got != "git::https://example.invalid/mods.git//modules/c?ref=v2" {
		t.Errorf("unknown module: %s", got)
	}
	if calls != 1 {
		t.Errorf("the catalog is read once per minute, read %d times", calls)
	}
	now = now.Add(2 * time.Minute)
	r.build("", "a", "v1")
	if calls != 2 {
		t.Errorf("the catalog must be re-read after the TTL, calls=%d", calls)
	}
}

// D-060: the source of a module call comes from the catalog the project sees.
func TestModuleSourceIsLookedUpPerProject(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r := &sourceResolver{base: "git::https://example.invalid/mods.git//modules", now: func() time.Time { return now }}
	mod := func(src string) catalog.Module {
		b, _ := json.Marshal(map[string]any{"x-tf-module": map[string]string{"source": src}})
		return catalog.Module{Name: "a", Version: "v1", SchemaJSON: b}
	}
	r.list = func(_ context.Context, projectID string) ([]catalog.Module, error) {
		if projectID == "p1" {
			return []catalog.Module{mod("git::https://example.invalid/p1.git//a")}, nil
		}
		return []catalog.Module{mod("git::https://example.invalid/global.git//a")}, nil
	}
	if got := r.build("p1", "a", "v1"); got != "git::https://example.invalid/p1.git//a?ref=v1" {
		t.Errorf("project 1: %s", got)
	}
	if got := r.build("p2", "a", "v1"); got != "git::https://example.invalid/global.git//a?ref=v1" {
		t.Errorf("project 2 must not get project 1's source: %s", got)
	}
	if got := r.build("", "a", "v1"); got != "git::https://example.invalid/global.git//a?ref=v1" {
		t.Errorf("no project: %s", got)
	}
}
