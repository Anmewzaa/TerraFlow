package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// maxGrantsPerUser caps one authorization, so a huge account cannot fill the table.
const maxGrantsPerUser = 500

// RepoGrant says that GitHub itself reported, recently, that this user can access
// this repository through this connection (D-049 layer 2). It holds no secret.
type RepoGrant struct {
	UserOID        string    `json:"-"`
	ConnectionID   string    `json:"connection_id"`
	RepoExternalID string    `json:"repo_external_id"`
	RepoFullName   string    `json:"repo_full_name"`
	DefaultBranch  string    `json:"default_branch"`
	Private        bool      `json:"private"`
	GitHubUserID   string    `json:"-"`
	GitHubLogin    string    `json:"github_login"`
	ExpiresAt      time.Time `json:"expires_at"`
}

const grantColumns = `user_oid, connection_id, repo_external_id, repo_full_name, default_branch, private, github_user_id, github_login, expires_at`

// ReplaceGrants swaps every grant of the user for one connection with the given
// list, in one transaction: a repository that is no longer reachable loses its grant
// at once. At most maxGrantsPerUser are kept.
func (q *Queries) ReplaceGrants(ctx context.Context, userOID, connectionID string, grants []RepoGrant) error {
	if len(grants) > maxGrantsPerUser {
		grants = grants[:maxGrantsPerUser]
	}
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM repo_grants WHERE user_oid = $1 AND connection_id = $2`, userOID, connectionID); err != nil {
		return err
	}
	for _, g := range grants {
		if _, err := tx.Exec(ctx,
			`INSERT INTO repo_grants (`+grantColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			userOID, connectionID, g.RepoExternalID, g.RepoFullName, g.DefaultBranch, g.Private, g.GitHubUserID, g.GitHubLogin, g.ExpiresAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListGrants returns the user's grants that have not expired, by repository name.
func (q *Queries) ListGrants(ctx context.Context, userOID string) ([]RepoGrant, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+grantColumns+` FROM repo_grants WHERE user_oid = $1 AND expires_at > now() ORDER BY lower(repo_full_name)`, userOID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[RepoGrant])
}

// GetGrant returns the user's live grant for one repository, or pgx.ErrNoRows.
func (q *Queries) GetGrant(ctx context.Context, userOID, connectionID, repoExternalID string) (RepoGrant, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+grantColumns+` FROM repo_grants
		 WHERE user_oid = $1 AND connection_id = $2 AND repo_external_id = $3 AND expires_at > now()`,
		userOID, connectionID, repoExternalID)
	if err != nil {
		return RepoGrant{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[RepoGrant])
}

// DeleteGrant removes one grant (it is used up when the project is created).
func (q *Queries) DeleteGrant(ctx context.Context, userOID, connectionID, repoExternalID string) error {
	_, err := q.pool.Exec(ctx,
		`DELETE FROM repo_grants WHERE user_oid = $1 AND connection_id = $2 AND repo_external_id = $3`,
		userOID, connectionID, repoExternalID)
	return err
}

// PurgeExpiredGrants deletes grants past their time and returns how many.
func (q *Queries) PurgeExpiredGrants(ctx context.Context) (int64, error) {
	ct, err := q.pool.Exec(ctx, `DELETE FROM repo_grants WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}
