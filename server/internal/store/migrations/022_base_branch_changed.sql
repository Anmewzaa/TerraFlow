-- +goose Up
-- Phase 8.8: when the base branch was last changed, so the screen can say how many pending
-- changesets were made before (on the old base). NULL: never changed.
ALTER TABLE project_repos
    ADD COLUMN base_branch_changed_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE project_repos
    DROP COLUMN base_branch_changed_at;
