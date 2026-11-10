-- +goose Up
-- Phase 8, M2 (D-050): who is allowed in. A person who signs in for the first time is
-- "pending" until an admin approves them (the default sign-up mode); a rejected or
-- suspended person cannot sign in or use a session they already hold. Existing users
-- stay active, so nothing changes for them. People are identified by their provider id
-- (github:<numeric id>, or the Entra object id), never by a login name.
ALTER TABLE users
    ADD COLUMN provider      TEXT NOT NULL DEFAULT 'entra',
    ADD COLUMN login         TEXT NOT NULL DEFAULT '',
    ADD COLUMN status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('pending', 'active', 'rejected', 'suspended')),
    ADD COLUMN status_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN status_by     TEXT NOT NULL DEFAULT '',
    ADD COLUMN status_at     TIMESTAMPTZ;

CREATE INDEX idx_users_status ON users (status);

-- +goose Down
DROP INDEX idx_users_status;
ALTER TABLE users
    DROP COLUMN status_at,
    DROP COLUMN status_by,
    DROP COLUMN status_reason,
    DROP COLUMN status,
    DROP COLUMN login,
    DROP COLUMN provider;
