-- +goose Up
-- Phase 8.1, layer 2 (D-049): a repo grant records that, a few minutes ago, GitHub
-- itself said this user can access this repository through this installation. The
-- user's token is used once while the grant is made and is never stored; creating a
-- project needs a live grant, so a repository the user cannot reach cannot be
-- connected. Grants are short lived and hold no secret.
CREATE TABLE repo_grants (
    user_oid         TEXT NOT NULL,
    connection_id    UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    repo_external_id TEXT NOT NULL,
    repo_full_name   TEXT NOT NULL,
    default_branch   TEXT NOT NULL,
    private          BOOLEAN NOT NULL DEFAULT true,
    -- The GitHub account that authorized (for the audit trail).
    github_user_id   TEXT NOT NULL,
    github_login     TEXT NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_oid, connection_id, repo_external_id)
);

CREATE INDEX idx_repo_grants_expiry ON repo_grants (expires_at);

-- +goose Down
DROP TABLE repo_grants;
