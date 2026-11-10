package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// User is a person who has signed in at least once.
type User struct {
	OID         string    `json:"oid"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// UserSummary is a user with the number of projects they are assigned to.
type UserSummary struct {
	User
	ProjectCount int    `json:"project_count"`
	Provider     string `json:"provider"`
	Login        string `json:"login"`
	// Status is active, pending, rejected or suspended (migration 016).
	Status       string `json:"status"`
	StatusReason string `json:"status_reason"`
}

// Member is a user assigned to a project.
type Member struct {
	UserOID   string    `json:"user_oid"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AddedBy   string    `json:"added_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectPatch holds the fields an admin may change; nil means "leave as is".
type ProjectPatch struct {
	Name        *string
	Description *string
	IACPath     *string
}

// UnknownUserError is returned by SetMembers for a user that never signed in.
type UnknownUserError struct{ OID string }

func (e UnknownUserError) Error() string { return fmt.Sprintf("unknown user %q", e.OID) }

// UpsertUser records a sign-in. Existing users keep their created_at.
func (q *Queries) UpsertUser(ctx context.Context, u User) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO users (oid, email, name) VALUES ($1, $2, $3)
		 ON CONFLICT (oid) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name, last_login_at = now()`,
		u.OID, u.Email, u.Name)
	return err
}

// ListUsers returns every known user with their project count.
func (q *Queries) ListUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT u.oid, u.email, u.name, u.created_at, u.last_login_at, count(m.project_id)::int,
		        u.provider, u.login, u.status, u.status_reason
		 FROM users u LEFT JOIN project_members m ON m.user_oid = u.oid
		 GROUP BY u.oid ORDER BY lower(u.name), u.oid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserSummary{}
	for rows.Next() {
		var s UserSummary
		if err := rows.Scan(&s.OID, &s.Email, &s.Name, &s.CreatedAt, &s.LastLoginAt, &s.ProjectCount, &s.Provider, &s.Login, &s.Status, &s.StatusReason); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListProjectsForUser returns the projects the user is assigned to.
func (q *Queries) ListProjectsForUser(ctx context.Context, oid string) ([]Project, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT p.id, p.name, p.slug, p.entra_group_id, p.description, p.iac_path, p.source_type, p.created_at, p.updated_at
		 FROM projects p JOIN project_members m ON m.project_id = p.id
		 WHERE m.user_oid = $1 ORDER BY p.name`, oid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Project])
}

// IsMember reports whether the user is assigned to the project.
func (q *Queries) IsMember(ctx context.Context, projectID, oid string) (bool, error) {
	var ok bool
	err := q.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM project_members WHERE project_id = $1 AND user_oid = $2)`,
		projectID, oid).Scan(&ok)
	return ok, err
}

// ListMembers returns the project's members, by name.
func (q *Queries) ListMembers(ctx context.Context, projectID string) ([]Member, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT m.user_oid, u.email, u.name, m.added_by, m.created_at
		 FROM project_members m JOIN users u ON u.oid = m.user_oid
		 WHERE m.project_id = $1 ORDER BY lower(u.name), m.user_oid`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserOID, &m.Email, &m.Name, &m.AddedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetMembers makes oids the project's complete member list. It fails with
// pgx.ErrNoRows for an unknown project and UnknownUserError for a user that has
// never signed in; nothing changes in either case.
func (q *Queries) SetMembers(ctx context.Context, projectID string, oids []string, by string) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id = $1)`, projectID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	for _, oid := range oids {
		var known bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE oid = $1)`, oid).Scan(&known); err != nil {
			return err
		}
		if !known {
			return UnknownUserError{OID: oid}
		}
	}
	if _, err := tx.Exec(ctx,
		// The owner is never removed here (D-050): a project always keeps its owner.
		`DELETE FROM project_members WHERE project_id = $1 AND role <> 'owner' AND NOT (user_oid = ANY($2))`, projectID, oids); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO project_members (project_id, user_oid, added_by)
		 SELECT $1, o, $3 FROM unnest($2::text[]) AS o
		 ON CONFLICT (project_id, user_oid) DO NOTHING`, projectID, oids, by); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddMember assigns one user to a project (used by the dev seed).
func (q *Queries) AddMember(ctx context.Context, projectID, oid, by string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO project_members (project_id, user_oid, added_by) VALUES ($1, $2, $3)
		 ON CONFLICT (project_id, user_oid) DO NOTHING`, projectID, oid, by)
	return err
}

// UpdateProject applies the non-nil fields of patch. It returns pgx.ErrNoRows
// for an unknown project.
func (q *Queries) UpdateProject(ctx context.Context, id string, patch ProjectPatch) (Project, error) {
	rows, err := q.pool.Query(ctx,
		`UPDATE projects SET
		   name = COALESCE($2, name), description = COALESCE($3, description),
		   iac_path = COALESCE($4, iac_path), updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, slug, entra_group_id, description, iac_path, source_type, created_at, updated_at`,
		id, patch.Name, patch.Description, patch.IACPath)
	if err != nil {
		return Project{}, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Project])
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, pgx.ErrNoRows
	}
	return p, err
}

// CountPendingChangesets counts changesets waiting for review in a project.
func (q *Queries) CountPendingChangesets(ctx context.Context, projectID string) (int, error) {
	var n int
	err := q.pool.QueryRow(ctx,
		`SELECT count(*)::int FROM changesets WHERE project_id = $1 AND status = 'published'`, projectID).Scan(&n)
	return n, err
}
