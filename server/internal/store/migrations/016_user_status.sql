-- +goose Up
-- Phase 8.5 (D-050): who a user is (provider) and whether they may use TerraFlow
-- (status). Everyone who signed in before this migration stays active. New people
-- start as pending unless an admin rule says otherwise.
ALTER TABLE users
    ADD COLUMN provider      TEXT NOT NULL DEFAULT 'entra',
    ADD COLUMN status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('pending', 'active', 'rejected', 'suspended')),
    ADD COLUMN status_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN status_by     TEXT NOT NULL DEFAULT '',
    ADD COLUMN status_at     TIMESTAMPTZ;

CREATE INDEX idx_users_status ON users (status);

-- +goose Down
DROP INDEX idx_users_status;
ALTER TABLE users
    DROP COLUMN status_at, DROP COLUMN status_by, DROP COLUMN status_reason,
    DROP COLUMN status, DROP COLUMN provider;
