package store

import (
	"context"
	"log/slog"
)

func Seed(ctx context.Context, q *Queries, logger *slog.Logger, sampleDir string) error {
	p := Project{
		ID:         "00000000-0000-0000-0000-000000000001",
		Name:       "Project 1 (Dev)",
		Slug:       "project-1-dev",
		IACPath:    sampleDir,
		SourceType: "local",
	}
	if err := q.UpsertProject(ctx, p); err != nil {
		return err
	}
	logger.Info("seeded project", "slug", p.Slug, "path", p.IACPath)
	return nil
}
