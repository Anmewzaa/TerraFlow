package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func SeedFromDir(ctx context.Context, store Store, logger *slog.Logger, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read catalog dir %s: %w", dir, err)
	}

	for _, modEntry := range entries {
		if !modEntry.IsDir() {
			continue
		}
		modName := modEntry.Name()
		modPath := filepath.Join(dir, modName)

		versions, err := os.ReadDir(modPath)
		if err != nil {
			return fmt.Errorf("read module dir %s: %w", modPath, err)
		}

		for _, verEntry := range versions {
			if !verEntry.IsDir() {
				continue
			}
			version := verEntry.Name()
			schemaPath := filepath.Join(modPath, version, "schema.json")

			data, err := os.ReadFile(schemaPath)
			if err != nil {
				logger.Warn("skip module version, no schema.json", "module", modName, "version", version)
				continue
			}

			checksum := fmt.Sprintf("%x", sha256.Sum256(data))

			existing, err := store.GetModule(ctx, modName, version)
			if err == nil && existing.Checksum == checksum {
				logger.Debug("module unchanged, skip", "module", modName, "version", version)
				continue
			}

			var schema struct {
				Title       string       `json:"title"`
				Description string       `json:"description"`
				Meta        TFModuleMeta `json:"x-tf-module"`
			}
			if err := json.Unmarshal(data, &schema); err != nil {
				return fmt.Errorf("parse schema %s: %w", schemaPath, err)
			}

			m := Module{
				Name:         modName,
				Version:      version,
				Title:        schema.Title,
				Description:  schema.Description,
				Icon:         schema.Meta.Icon,
				TargetFolder: schema.Meta.TargetFolder,
				SchemaJSON:   json.RawMessage(data),
				Checksum:     checksum,
			}

			if err := store.UpsertModule(ctx, m); err != nil {
				return fmt.Errorf("upsert module %s/%s: %w", modName, version, err)
			}
			logger.Info("seeded module", "module", modName, "version", version)
		}
	}
	return nil
}
