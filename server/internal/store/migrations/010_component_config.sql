-- +goose Up
-- Presentation settings for components that live in the repository (D-035), such
-- as the colour of a resource group. The address is "<folder>/<module label>".
CREATE TABLE project_component_config (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    address    TEXT NOT NULL,
    color      TEXT NOT NULL,
    updated_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, address)
);

-- +goose Down
DROP TABLE project_component_config;
