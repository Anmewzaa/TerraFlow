-- +goose Up
-- Modules can be imported from the Admin area and retired later. A published
-- version is immutable; deprecating hides it from the catalog developers use
-- but keeps it readable for drafts and changesets that already reference it.
ALTER TABLE catalog_modules
    ADD COLUMN status      TEXT NOT NULL DEFAULT 'published' CHECK (status IN ('published', 'deprecated')),
    ADD COLUMN imported_by TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE catalog_modules DROP COLUMN imported_by;
ALTER TABLE catalog_modules DROP COLUMN status;
