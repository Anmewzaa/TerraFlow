-- +goose Up
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_oid    TEXT NOT NULL,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    detail      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_user ON audit_log (user_oid, created_at DESC);

-- +goose Down
DROP TABLE audit_log;
