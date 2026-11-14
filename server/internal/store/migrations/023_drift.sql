-- +goose Up
-- Phase 8.14: repository drift. The baseline is the files (path -> hash) the base branch had when
-- the last changeset was approved, or when someone accepted the current state; a dismissal hides
-- one drift item with a note kept for the project.
ALTER TABLE project_repos
    ADD COLUMN drift_baseline JSONB,
    ADD COLUMN drift_baseline_at TIMESTAMPTZ;

CREATE TABLE drift_dismissals (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    item_key   TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    by_oid     TEXT NOT NULL,
    at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, item_key)
);

-- +goose Down
DROP TABLE drift_dismissals;
ALTER TABLE project_repos
    DROP COLUMN drift_baseline_at,
    DROP COLUMN drift_baseline;
