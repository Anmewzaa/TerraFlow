-- +goose Up
-- Phase 8.15: stack templates of a project. The template (nodes, arrows, slots, parameters) is one
-- JSON document; a name is unique within the project.
CREATE TABLE stack_templates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    template   JSONB NOT NULL,
    UNIQUE (project_id, name)
);

-- +goose Down
DROP TABLE stack_templates;
