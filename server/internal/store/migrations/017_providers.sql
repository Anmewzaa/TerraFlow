-- +goose Up
-- Phase 8, M4a (D-058): cloud providers as data. A provider is a name, its Terraform source address,
-- the version constraints a team picks from, a logo and the block written to providers.tf. Nothing in
-- the code branches on a provider id. azurerm is seeded so existing projects keep working.
CREATE TABLE providers (
    id         TEXT PRIMARY KEY CHECK (id ~ '^[a-z][a-z0-9]{1,30}$'),
    name       TEXT NOT NULL,
    source     TEXT NOT NULL,
    logo       TEXT NOT NULL DEFAULT '',
    versions   TEXT[] NOT NULL DEFAULT '{}',
    block      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO providers (id, name, source, logo, versions, block) VALUES (
    'azurerm', 'Microsoft Azure', 'hashicorp/azurerm',
    'data:image/svg+xml;base64,' || replace(encode(convert_to(
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect width="24" height="24" rx="5" fill="#0078d4"/><path fill="#fff" d="M5 18 11 5h3l-3.4 7.4L16 12l-3 6z"/></svg>',
        'UTF8'), 'base64'), chr(10), ''),
    ARRAY['~> 4.0', '~> 3.0'],
    'provider "azurerm" {' || chr(10) || '  features {}' || chr(10) || '}'
);

-- A raw resource belongs to a provider, has a category, and says from which provider version (major.minor)
-- it exists and until which. The provider of existing templates is the part of the resource type before
-- the first underscore.
ALTER TABLE raw_resource_templates
    ADD COLUMN provider    TEXT NOT NULL DEFAULT '',
    ADD COLUMN category    TEXT NOT NULL DEFAULT '',
    ADD COLUMN min_version TEXT NOT NULL DEFAULT '',
    ADD COLUMN max_version TEXT NOT NULL DEFAULT '';
UPDATE raw_resource_templates SET provider = split_part(resource_type, '_', 1) WHERE position('_' in resource_type) > 1;

-- +goose Down
ALTER TABLE raw_resource_templates DROP COLUMN max_version, DROP COLUMN min_version, DROP COLUMN category, DROP COLUMN provider;
DROP TABLE providers;
