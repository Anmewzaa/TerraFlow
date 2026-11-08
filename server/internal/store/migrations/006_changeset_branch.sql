-- +goose Up
ALTER TABLE changesets ADD COLUMN branch_name TEXT;
ALTER TABLE changesets ADD COLUMN commit_sha TEXT;

CREATE INDEX idx_audit_log_project ON audit_log ((detail->>'project_id'), id DESC);

-- +goose Down
DROP INDEX idx_audit_log_project;
ALTER TABLE changesets DROP COLUMN commit_sha;
ALTER TABLE changesets DROP COLUMN branch_name;
