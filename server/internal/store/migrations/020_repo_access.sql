-- +goose Up
-- Phase 8 M2 (D-061): the owner's own access to the project's repository is checked
-- again each time they prove who they are to GitHub (sign-in or connect). When GitHub
-- says they can no longer reach the repository, the link needs attention and the
-- project stops reading and writing through the App until the owner verifies again.
-- Existing links stay 'ok': they were verified when they were made.
ALTER TABLE project_repos
    ADD COLUMN access_status TEXT NOT NULL DEFAULT 'ok' CHECK (access_status IN ('ok', 'needs_attention')),
    ADD COLUMN access_checked_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE project_repos
    DROP COLUMN access_checked_at,
    DROP COLUMN access_status;
