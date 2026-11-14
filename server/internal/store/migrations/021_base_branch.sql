-- +goose Up
-- Phase 8.8: a project may read from, and start its review branches from, a branch other
-- than the repository default. Empty means the repository default, so nothing changes for
-- existing projects. It is only read; Approve still creates a new branch (rules 1 and 7).
ALTER TABLE project_repos
    ADD COLUMN base_branch TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE project_repos
    DROP COLUMN base_branch;
