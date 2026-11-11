package catalog

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedFromDir(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "test-mod", "v1")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}

	schema := map[string]any{
		"title":       "Test Module",
		"description": "A test module.",
		"type":        "object",
		"x-tf-module": map[string]any{
			"source":        "test-mod",
			"version":       "v1",
			"target_folder": "test",
			"icon":          "Test",
		},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	if err := os.WriteFile(filepath.Join(modDir, "schema.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewFSStore(dir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := SeedFromDir(context.Background(), store, logger, dir); err != nil {
		t.Fatalf("SeedFromDir: %v", err)
	}

	modules, err := store.ListModules(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}

	m := modules[0]
	if m.Name != "test-mod" {
		t.Errorf("name = %q, want %q", m.Name, "test-mod")
	}
	if m.Version != "v1" {
		t.Errorf("version = %q, want %q", m.Version, "v1")
	}
	if m.Title != "Test Module" {
		t.Errorf("title = %q, want %q", m.Title, "Test Module")
	}
	if m.Icon != "Test" {
		t.Errorf("icon = %q, want %q", m.Icon, "Test")
	}
	if m.TargetFolder != "test" {
		t.Errorf("target_folder = %q, want %q", m.TargetFolder, "test")
	}
}

func TestSeedIdempotent(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "mod-a", "v1")
	os.MkdirAll(modDir, 0o755)

	schema := map[string]any{
		"title": "Mod A", "description": "", "type": "object",
		"x-tf-module": map[string]any{"source": "mod-a", "version": "v1", "target_folder": "a", "icon": "A"},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	os.WriteFile(filepath.Join(modDir, "schema.json"), data, 0o644)

	store := NewFSStore(dir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	SeedFromDir(ctx, store, logger, dir)
	SeedFromDir(ctx, store, logger, dir)

	modules, _ := store.ListModules(ctx, "")
	if len(modules) != 1 {
		t.Fatalf("expected 1 module after double seed, got %d", len(modules))
	}
}
