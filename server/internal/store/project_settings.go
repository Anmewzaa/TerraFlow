package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// GetProjectSettings returns the project's settings document ("{}" when never set).
func (q *Queries) GetProjectSettings(ctx context.Context, projectID string) (json.RawMessage, error) {
	var raw []byte
	err := q.pool.QueryRow(ctx, `SELECT settings FROM projects WHERE id = $1`, projectID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// SetProjectSettings replaces the settings document. The caller has validated it.
func (q *Queries) SetProjectSettings(ctx context.Context, projectID string, raw json.RawMessage) error {
	tag, err := q.pool.Exec(ctx, `UPDATE projects SET settings = $2, updated_at = now() WHERE id = $1`, projectID, []byte(raw))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
