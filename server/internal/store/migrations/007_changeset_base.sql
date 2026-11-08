-- +goose Up
-- A revision starts as a copy of an earlier changeset; base_canvas_json keeps
-- that starting canvas so the HCL diff can show what the revision adds,
-- changes and removes. NULL = a brand-new changeset (everything is "added").
ALTER TABLE changesets ADD COLUMN base_canvas_json JSONB;

-- +goose Down
ALTER TABLE changesets DROP COLUMN base_canvas_json;
