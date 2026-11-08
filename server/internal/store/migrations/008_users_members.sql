-- +goose Up
-- Who can see which project is managed inside the app (D-032). Entra only
-- decides the global role (admin / user). A user row is created at first sign-in
-- so an admin can assign people to projects.
ALTER TABLE projects ADD COLUMN description TEXT NOT NULL DEFAULT '';

CREATE TABLE users (
    oid           TEXT PRIMARY KEY,
    email         TEXT NOT NULL DEFAULT '',
    name          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_oid   TEXT NOT NULL REFERENCES users(oid) ON DELETE CASCADE,
    added_by   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_oid)
);

CREATE INDEX idx_project_members_user ON project_members (user_oid);

-- +goose Down
DROP TABLE project_members;
DROP TABLE users;
ALTER TABLE projects DROP COLUMN description;
