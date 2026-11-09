-- +goose Up
-- Phase 8 (D-049, D-050): provider-neutral connections, the repository a project is
-- linked to, and project roles. Additive: existing projects keep working with
-- source_type = 'local'; existing members become contributors.

-- A connection is the link between TerraFlow and a git account. For GitHub it is one
-- installation of the GitHub App; only its id and the account are stored, never a
-- token or a key (tokens last one hour and are made per call).
CREATE TABLE connections (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider      TEXT NOT NULL,
    external_id   TEXT NOT NULL,
    account_login TEXT NOT NULL,
    account_type  TEXT NOT NULL DEFAULT 'User',
    created_by    TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'needs_attention', 'disconnected')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

-- Where a project's code lives. All columns are NULL for a local project.
CREATE TABLE project_repos (
    project_id       UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    connection_id    UUID NOT NULL REFERENCES connections(id),
    repo_external_id TEXT NOT NULL,
    repo_full_name   TEXT NOT NULL,
    default_branch   TEXT NOT NULL,
    repo_path        TEXT NOT NULL DEFAULT '',
    -- The person who connected the repository; the one whose access was checked.
    connected_by     TEXT NOT NULL,
    -- A per-project switch: require a reviewer other than the requester.
    second_reviewer  BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_project_repos_connection ON project_repos (connection_id);

ALTER TABLE project_members
    ADD COLUMN role TEXT NOT NULL DEFAULT 'contributor' CHECK (role IN ('owner', 'co_owner', 'contributor'));

-- At most one owner per project.
CREATE UNIQUE INDEX idx_project_one_owner ON project_members (project_id) WHERE role = 'owner';

-- +goose Down
DROP INDEX idx_project_one_owner;
ALTER TABLE project_members DROP COLUMN role;
DROP TABLE project_repos;
DROP TABLE connections;
