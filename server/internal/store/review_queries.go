package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReviewChangeset records a reviewer's decision on a published changeset.
// It only succeeds while the changeset is still 'published' (no double review).
func (q *Queries) ReviewChangeset(ctx context.Context, id string, status ChangesetStatus,
	reviewerOID, reason, branch, commit string) error {
	ct, err := q.pool.Exec(ctx,
		`UPDATE changesets
		 SET status = $2::changeset_status, reviewer_oid = $3,
		     reject_reason = NULLIF($4, ''), branch_name = NULLIF($5, ''), commit_sha = NULLIF($6, ''),
		     reviewed_at = now(), updated_at = now()
		 WHERE id = $1 AND status = 'published'`,
		id, string(status), reviewerOID, reason, branch, commit)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// AuditEntry is one row of the audit log.
type AuditEntry struct {
	ID        int64           `json:"id"         db:"id"`
	UserOID   string          `json:"user_oid"   db:"user_oid"`
	Action    string          `json:"action"     db:"action"`
	Resource  string          `json:"resource"   db:"resource"`
	Detail    json.RawMessage `json:"detail"     db:"detail"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// ListAudit returns the newest audit rows whose detail.project_id matches.
func (q *Queries) ListAudit(ctx context.Context, projectID string, limit int) ([]AuditEntry, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_oid, action, resource, detail, created_at
		 FROM audit_log WHERE detail->>'project_id' = $1
		 ORDER BY id DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[AuditEntry])
}

// ListAuditAll returns the newest audit rows of every kind (publish, approve,
// project and module management), for the admin audit page.
func (q *Queries) ListAuditAll(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_oid, action, resource, detail, created_at
		 FROM audit_log ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[AuditEntry])
}
