package catalog

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func catalogDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// schema_test.go is at server/internal/catalog/ → go up 3 to repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "catalog", "modules")
}

func TestSampleSchemasLoadAndValidate(t *testing.T) {
	dir := catalogDir(t)
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("catalog dir not found: %s", dir)
	}

	store := NewFSStore(dir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	if err := SeedFromDir(ctx, store, logger, dir); err != nil {
		t.Fatalf("SeedFromDir: %v", err)
	}

	modules, err := store.ListModules(ctx)
	if err != nil {
		t.Fatal(err)
	}

	expectedModules := map[string]bool{
		"resource-group":         false,
		"virtual-network":        false,
		"network-security-group": false,
		"key-vault":              false,
		"storage-account":        false,
	}

	for _, m := range modules {
		expectedModules[m.Name] = true

		t.Run(m.Name+"/"+m.Version, func(t *testing.T) {
			if m.Title == "" {
				t.Error("title is empty")
			}
			if m.Icon == "" {
				t.Error("icon is empty")
			}
			if m.TargetFolder == "" {
				t.Error("target_folder is empty")
			}

			var schema map[string]json.RawMessage
			if err := json.Unmarshal(m.SchemaJSON, &schema); err != nil {
				t.Fatalf("invalid JSON schema: %v", err)
			}
			if _, ok := schema["properties"]; !ok {
				t.Error("schema missing properties")
			}
			if _, ok := schema["x-tf-module"]; !ok {
				t.Error("schema missing x-tf-module")
			}
			if _, ok := schema["x-tf-ui"]; !ok {
				t.Error("schema missing x-tf-ui")
			}

			var meta TFModuleMeta
			json.Unmarshal(schema["x-tf-module"], &meta)
			// The source is the module's name or a full module source that ends in it.
			if meta.Source != m.Name && !strings.HasSuffix(meta.Source, "/"+m.Name) {
				t.Errorf("x-tf-module.source = %q, want %q or a source ending in /%s", meta.Source, m.Name, m.Name)
			}
			if meta.Version != m.Version {
				t.Errorf("x-tf-module.version = %q, want %q", meta.Version, m.Version)
			}
		})
	}

	for name, found := range expectedModules {
		if !found {
			t.Errorf("expected module %q not found in catalog", name)
		}
	}
}
