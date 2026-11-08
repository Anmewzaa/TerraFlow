-- +goose Up
-- Raw provider resources an admin has prepared (D-040/D-041): an empty
-- `resource "azurerm_x" "name" {}` developers can place on the canvas.
-- Unlike a catalog module there is no schema; default_values are the starting
-- arguments.
CREATE TABLE raw_resource_templates (
    name           TEXT PRIMARY KEY CHECK (name ~ '^[a-z0-9][a-z0-9-]{1,62}$'),
    title          TEXT NOT NULL,
    resource_type  TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    default_values JSONB NOT NULL DEFAULT '{}'::jsonb,
    target_folder  TEXT NOT NULL DEFAULT '',
    updated_by     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE raw_resource_templates;
