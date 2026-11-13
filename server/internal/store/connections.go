package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Connections (D-050) ---

// Connection statuses.
const (
	ConnActive         = "active"
	ConnNeedsAttention = "needs_attention"
	ConnDisconnected   = "disconnected"
)

// Connection is one link between TerraFlow and a git account. It holds no secret.
type Connection struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	ExternalID   string    `json:"external_id"`
	AccountLogin string    `json:"account_login"`
	AccountType  string    `json:"account_type"`
	CreatedBy    string    `json:"created_by"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const connColumns = `id, provider, external_id, account_login, account_type, created_by, status, created_at, updated_at`

// UpsertConnection records an installation. The same provider and external id is
// one connection; reconnecting reactivates it and refreshes the account name but
// never changes who created it.
func (q *Queries) UpsertConnection(ctx context.Context, c Connection) (Connection, error) {
	if c.Provider == "" || c.ExternalID == "" || c.AccountLogin == "" || c.CreatedBy == "" {
		return Connection{}, errors.New("connection needs a provider, an external id, an account and a creator")
	}
	if c.AccountType == "" {
		c.AccountType = "User"
	}
	rows, err := q.pool.Query(ctx,
		`INSERT INTO connections (provider, external_id, account_login, account_type, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (provider, external_id) DO UPDATE SET
		   account_login = EXCLUDED.account_login, account_type = EXCLUDED.account_type,
		   status = 'active', updated_at = now()
		 RETURNING `+connColumns,
		c.Provider, c.ExternalID, c.AccountLogin, c.AccountType, c.CreatedBy)
	if err != nil {
		return Connection{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Connection])
}

// GetConnection returns pgx.ErrNoRows for an unknown id.
func (q *Queries) GetConnection(ctx context.Context, id string) (Connection, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+connColumns+` FROM connections WHERE id = $1`, id)
	if err != nil {
		return Connection{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Connection])
}

// ListConnections returns every connection (admin) newest first.
func (q *Queries) ListConnections(ctx context.Context) ([]Connection, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+connColumns+` FROM connections ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Connection])
}

// SetConnectionStatus changes the status; an unknown id is pgx.ErrNoRows.
func (q *Queries) SetConnectionStatus(ctx context.Context, id, status string) error {
	switch status {
	case ConnActive, ConnNeedsAttention, ConnDisconnected:
	default:
		return fmt.Errorf("unknown connection status %q", status)
	}
	ct, err := q.pool.Exec(ctx, `UPDATE connections SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// --- The repository of a project ---

// ProjectRepo says where a project's code lives (not set for a local project).
type ProjectRepo struct {
	ProjectID      string `json:"project_id"`
	ConnectionID   string `json:"connection_id"`
	RepoExternalID string `json:"repo_external_id"`
	RepoFullName   string `json:"repo_full_name"`
	DefaultBranch  string `json:"default_branch"`
	RepoPath       string `json:"repo_path"`
	ConnectedBy    string `json:"connected_by"`
	SecondReviewer bool   `json:"second_reviewer"`
	// AccessStatus is RepoAccessOK, or RepoAccessNeedsAttention once GitHub said the
	// project's owner can no longer reach the repository (D-061).
	AccessStatus    string     `json:"access_status"`
	AccessCheckedAt *time.Time `json:"access_checked_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// The owner's access to a project's repository, as GitHub last reported it.
const (
	RepoAccessOK             = "ok"
	RepoAccessNeedsAttention = "needs_attention"
)

const repoColumns = `project_id, connection_id, repo_external_id, repo_full_name, default_branch, repo_path, connected_by, second_reviewer, access_status, access_checked_at, created_at, updated_at`

// SetProjectRepo links a project to a repository, replacing an earlier link.
func (q *Queries) SetProjectRepo(ctx context.Context, r ProjectRepo) error {
	if r.ProjectID == "" || r.ConnectionID == "" || r.RepoExternalID == "" || r.RepoFullName == "" || r.DefaultBranch == "" || r.ConnectedBy == "" {
		return errors.New("project repository needs a project, a connection, a repository, a branch and who connected it")
	}
	_, err := q.pool.Exec(ctx,
		`INSERT INTO project_repos (project_id, connection_id, repo_external_id, repo_full_name, default_branch, repo_path, connected_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (project_id) DO UPDATE SET
		   connection_id = EXCLUDED.connection_id, repo_external_id = EXCLUDED.repo_external_id,
		   repo_full_name = EXCLUDED.repo_full_name, default_branch = EXCLUDED.default_branch,
		   repo_path = EXCLUDED.repo_path, connected_by = EXCLUDED.connected_by,
		   access_status = 'ok', access_checked_at = now(), updated_at = now()`,
		r.ProjectID, r.ConnectionID, r.RepoExternalID, r.RepoFullName, r.DefaultBranch, r.RepoPath, r.ConnectedBy)
	return err
}

// GetProjectRepo returns pgx.ErrNoRows for a project without a repository link.
func (q *Queries) GetProjectRepo(ctx context.Context, projectID string) (ProjectRepo, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+repoColumns+` FROM project_repos WHERE project_id = $1`, projectID)
	if err != nil {
		return ProjectRepo{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ProjectRepo])
}

// OwnedRepo is a repository link of a project this person owns, with what is needed to
// ask GitHub whether they can still reach it.
type OwnedRepo struct {
	ProjectID      string `json:"project_id"`
	InstallationID string `json:"installation_id"`
	RepoExternalID string `json:"repo_external_id"`
	RepoFullName   string `json:"repo_full_name"`
	AccessStatus   string `json:"access_status"`
}

// OwnedRepos lists the repository links of the projects whose owner is oid.
func (q *Queries) OwnedRepos(ctx context.Context, oid string) ([]OwnedRepo, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT pr.project_id, c.external_id AS installation_id, pr.repo_external_id, pr.repo_full_name, pr.access_status
		   FROM project_repos pr
		   JOIN project_members m ON m.project_id = pr.project_id AND m.role = 'owner'
		   JOIN connections c ON c.id = pr.connection_id
		  WHERE m.user_oid = $1 AND c.provider = 'github'
		  ORDER BY pr.project_id`, oid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[OwnedRepo])
}

// SetRepoAccess records what GitHub said about the owner's access and when.
func (q *Queries) SetRepoAccess(ctx context.Context, projectID, status string) error {
	if status != RepoAccessOK && status != RepoAccessNeedsAttention {
		return errors.New("unknown repository access status")
	}
	ct, err := q.pool.Exec(ctx,
		`UPDATE project_repos SET access_status = $2, access_checked_at = now(), updated_at = now() WHERE project_id = $1`,
		projectID, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SetSecondReviewer turns the "another reviewer is required" switch on or off.
func (q *Queries) SetSecondReviewer(ctx context.Context, projectID string, on bool) error {
	ct, err := q.pool.Exec(ctx, `UPDATE project_repos SET second_reviewer = $2, updated_at = now() WHERE project_id = $1`, projectID, on)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteProjectRepo removes the link (disconnecting the repository).
func (q *Queries) DeleteProjectRepo(ctx context.Context, projectID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM project_repos WHERE project_id = $1`, projectID)
	return err
}

// ProjectsOfConnection counts the projects that use a connection.
func (q *Queries) ProjectsOfConnection(ctx context.Context, connectionID string) (int, error) {
	var n int
	err := q.pool.QueryRow(ctx, `SELECT count(*) FROM project_repos WHERE connection_id = $1`, connectionID).Scan(&n)
	return n, err
}

// --- Project roles ---

// Project roles (D-050).
const (
	RoleOwner       = "owner"
	RoleCoOwner     = "co_owner"
	RoleContributor = "contributor"
)

// ValidRole reports whether s is one of the three roles.
func ValidRole(s string) bool { return s == RoleOwner || s == RoleCoOwner || s == RoleContributor }

// RoleRank orders the roles: a higher number can do more.
func RoleRank(role string) int {
	switch role {
	case RoleOwner:
		return 3
	case RoleCoOwner:
		return 2
	case RoleContributor:
		return 1
	}
	return 0
}

// CanApprove reports whether the role may approve a changeset.
func CanApprove(role string) bool { return role == RoleOwner || role == RoleCoOwner }

// MemberRole returns the user's role in the project, or "" if they are not a member.
func (q *Queries) MemberRole(ctx context.Context, projectID, oid string) (string, error) {
	var role string
	err := q.pool.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id = $1 AND user_oid = $2`, projectID, oid).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// ErrNoOwner and ErrOwnerChange keep the ownership rules in one place.
var (
	// ErrOwnerChange: the owner's role cannot be changed or removed this way; use
	// TransferOwnership.
	ErrOwnerChange = errors.New("the owner's role cannot be changed here; transfer ownership instead")
	// ErrNotMember: the user is not a member of the project.
	ErrNotMember = errors.New("the user is not a member of the project")
)

// AddMemberRole adds a user with a role, or changes the role of an existing
// member. It never creates or changes an owner (only CreateOwner and
// TransferOwnership do), so no call can leave a project with two owners or none.
func (q *Queries) AddMemberRole(ctx context.Context, projectID, oid, role, by string) error {
	if !ValidRole(role) || role == RoleOwner {
		return fmt.Errorf("role must be %q or %q", RoleCoOwner, RoleContributor)
	}
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var current string
	switch err := tx.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id = $1 AND user_oid = $2 FOR UPDATE`, projectID, oid).Scan(&current); {
	case err == nil:
		if current == RoleOwner {
			return ErrOwnerChange
		}
		if _, err := tx.Exec(ctx, `UPDATE project_members SET role = $3 WHERE project_id = $1 AND user_oid = $2`, projectID, oid, role); err != nil {
			return err
		}
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx, `INSERT INTO project_members (project_id, user_oid, role, added_by) VALUES ($1, $2, $3, $4)`, projectID, oid, role, by); err != nil {
			return err
		}
	default:
		return err
	}
	return tx.Commit(ctx)
}

// RemoveMember removes a member; the owner cannot be removed.
func (q *Queries) RemoveMember(ctx context.Context, projectID, oid string) error {
	ct, err := q.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_oid = $2 AND role <> 'owner'`, projectID, oid)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		role, rerr := q.MemberRole(ctx, projectID, oid)
		if rerr != nil {
			return rerr
		}
		if role == RoleOwner {
			return ErrOwnerChange
		}
		return ErrNotMember
	}
	return nil
}

// CreateOwner makes the user the owner of a project that has none.
func (q *Queries) CreateOwner(ctx context.Context, projectID, oid string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO project_members (project_id, user_oid, role, added_by) VALUES ($1, $2, 'owner', $2)
		 ON CONFLICT (project_id, user_oid) DO UPDATE SET role = 'owner'`, projectID, oid)
	return err
}

// TransferOwnership makes `to` the owner and `from` a co-owner, in one step. `from`
// must be the current owner and `to` must already be a member.
func (q *Queries) TransferOwnership(ctx context.Context, projectID, from, to string) error {
	if from == to {
		return errors.New("the new owner is already the owner")
	}
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var role string
	if err := tx.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id = $1 AND user_oid = $2 FOR UPDATE`, projectID, from).Scan(&role); err != nil || role != RoleOwner {
		return errors.New("only the owner can transfer ownership")
	}
	if err := tx.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id = $1 AND user_oid = $2 FOR UPDATE`, projectID, to).Scan(&role); err != nil {
		return ErrNotMember
	}
	// The unique index allows one owner, so the old one steps down first.
	if _, err := tx.Exec(ctx, `UPDATE project_members SET role = 'co_owner' WHERE project_id = $1 AND user_oid = $2`, projectID, from); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE project_members SET role = 'owner' WHERE project_id = $1 AND user_oid = $2`, projectID, to); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RoleMember is a member with a role, for the members screen.
type RoleMember struct {
	UserOID   string    `json:"user_oid"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	AddedBy   string    `json:"added_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ListMembersWithRoles returns the members, owner first, then by name.
func (q *Queries) ListMembersWithRoles(ctx context.Context, projectID string) ([]RoleMember, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT m.user_oid, u.email, u.name, m.role, m.added_by, m.created_at
		 FROM project_members m JOIN users u ON u.oid = m.user_oid
		 WHERE m.project_id = $1
		 ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'co_owner' THEN 1 ELSE 2 END, lower(u.name), m.user_oid`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[RoleMember])
}
