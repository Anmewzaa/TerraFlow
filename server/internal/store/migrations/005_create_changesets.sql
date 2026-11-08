-- +goose Up
CREATE TYPE changeset_status AS ENUM ('draft', 'published', 'approved', 'rejected');

CREATE TABLE changesets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id),
    env         TEXT NOT NULL DEFAULT 'dev',
    user_oid    TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    status      changeset_status NOT NULL DEFAULT 'draft',
    canvas_json JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
    reject_reason TEXT,
    reviewer_oid  TEXT,
    reviewed_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_changesets_project ON changesets(project_id, user_oid, status);

-- +goose Down
DROP TABLE changesets;
DROP TYPE changeset_status;
