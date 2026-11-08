-- +goose NO TRANSACTION
-- +goose Up
-- A "saved" changeset is a named snapshot of a draft. The draft stays the
-- working copy; a saved version is published later from the Changesets page.
ALTER TYPE changeset_status ADD VALUE IF NOT EXISTS 'saved' AFTER 'draft';

-- +goose Down
-- Postgres cannot drop an enum value; removing the rows is the most a down can do.
DELETE FROM changesets WHERE status = 'saved';
