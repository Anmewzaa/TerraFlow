-- +goose Up
-- Phase 8, M3b (docs/11-project-settings.md): per-project settings for generated HCL: where new
-- resources go by default, project variables (org = aic) and starter files. A JSON document, so a
-- new option needs no migration; the server validates it before it is stored. An empty object means
-- "behave as before", so existing projects are unchanged.
ALTER TABLE projects ADD COLUMN settings JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE projects DROP COLUMN settings;
