-- +goose Up
CREATE TABLE catalog_modules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    version         TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    icon            TEXT NOT NULL DEFAULT '',
    target_folder   TEXT NOT NULL DEFAULT '',
    schema_json     JSONB NOT NULL,
    checksum        TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, version)
);

CREATE TABLE catalog_drafts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id),
    module_id   UUID NOT NULL REFERENCES catalog_modules(id),
    user_oid    TEXT NOT NULL,
    values_json JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_catalog_drafts_unique ON catalog_drafts(project_id, module_id, user_oid);
CREATE INDEX idx_catalog_drafts_project ON catalog_drafts(project_id, user_oid);

-- +goose Down
DROP TABLE catalog_drafts;
DROP TABLE catalog_modules;
