-- +goose Up
-- Phase 8.16: the shared template gallery. A gallery template is one stack template document (8.15) that
-- its author keeps private or offers to every signed-in person (public, after an admin approves it).
-- A person's own copy in a project stays in stack_templates; this table is the shared shelf.
CREATE TABLE gallery_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL,
    author_name TEXT NOT NULL DEFAULT '',
    scope       TEXT NOT NULL CHECK (scope IN ('private', 'public')),
    -- approved: visible to the scope; pending: a public template waiting for an admin;
    -- rejected, hidden (by an admin) and withdrawn (by the author) are not shown to others.
    status      TEXT NOT NULL CHECK (status IN ('approved', 'pending', 'rejected', 'hidden', 'withdrawn')),
    review_note TEXT NOT NULL DEFAULT '',
    reviewed_by TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,
    template    JSONB NOT NULL,
    modules     TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (author, name)
);
CREATE INDEX gallery_templates_shelf ON gallery_templates (scope, status);

-- Every import of a gallery template into a project, for "imported N times, used in N projects".
CREATE TABLE gallery_imports (
    id          BIGSERIAL PRIMARY KEY,
    template_id UUID NOT NULL REFERENCES gallery_templates(id) ON DELETE CASCADE,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    imported_by TEXT NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX gallery_imports_template ON gallery_imports (template_id);

-- +goose Down
DROP TABLE gallery_imports;
DROP TABLE gallery_templates;
