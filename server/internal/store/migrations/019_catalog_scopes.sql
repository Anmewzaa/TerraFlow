-- +goose Up
-- Phase 8, M3c (D-060): catalog scopes. A module schema or a raw resource template is global
-- (project_id IS NULL, managed by an admin) or belongs to one project (managed by its owner and
-- co-owners, seen only in that project). Names are unique per scope, so two projects may each
-- have their own "storage-account"; existing rows stay global.
ALTER TABLE catalog_modules ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE catalog_modules DROP CONSTRAINT catalog_modules_name_version_key;
CREATE UNIQUE INDEX catalog_modules_global_key ON catalog_modules (name, version) WHERE project_id IS NULL;
CREATE UNIQUE INDEX catalog_modules_project_key ON catalog_modules (project_id, name, version) WHERE project_id IS NOT NULL;

ALTER TABLE raw_resource_templates ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE raw_resource_templates DROP CONSTRAINT raw_resource_templates_pkey;
ALTER TABLE raw_resource_templates ADD COLUMN id UUID PRIMARY KEY DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX raw_resource_templates_global_key ON raw_resource_templates (name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX raw_resource_templates_project_key ON raw_resource_templates (project_id, name) WHERE project_id IS NOT NULL;

-- +goose Down
-- Project catalogs do not exist before this migration, so their rows go (and the drafts that point at them).
DELETE FROM catalog_drafts WHERE module_id IN (SELECT id FROM catalog_modules WHERE project_id IS NOT NULL);
DELETE FROM catalog_modules WHERE project_id IS NOT NULL;
DROP INDEX catalog_modules_project_key;
DROP INDEX catalog_modules_global_key;
ALTER TABLE catalog_modules ADD CONSTRAINT catalog_modules_name_version_key UNIQUE (name, version);
ALTER TABLE catalog_modules DROP COLUMN project_id;

DELETE FROM raw_resource_templates WHERE project_id IS NOT NULL;
DROP INDEX raw_resource_templates_project_key;
DROP INDEX raw_resource_templates_global_key;
ALTER TABLE raw_resource_templates DROP COLUMN id;
ALTER TABLE raw_resource_templates ADD PRIMARY KEY (name);
ALTER TABLE raw_resource_templates DROP COLUMN project_id;
