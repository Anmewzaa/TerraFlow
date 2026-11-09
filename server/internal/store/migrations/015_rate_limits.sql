-- +goose Up
-- Phase 8.0: the sign-in rate limiter counts in the database so that several server
-- instances share one count. One row per client key (a network address, never a user
-- name or a secret); old rows are removed as new ones are made.
CREATE TABLE rate_limits (
    key        TEXT PRIMARY KEY,
    tokens     DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_rate_limits_updated ON rate_limits (updated_at);

-- +goose Down
DROP TABLE rate_limits;
