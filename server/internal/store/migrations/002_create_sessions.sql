-- +goose Up
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_oid    TEXT NOT NULL,
    email       TEXT NOT NULL,
    name        TEXT NOT NULL,
    roles       TEXT[] NOT NULL DEFAULT '{}',
    group_ids   TEXT[] NOT NULL DEFAULT '{}',
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_expires ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
