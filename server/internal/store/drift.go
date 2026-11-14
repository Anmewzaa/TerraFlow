package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// DriftBaseline is the files (path -> short hash) of the base branch at a moment, and when.
type DriftBaseline struct {
	Files map[string]string
	At    time.Time
}

// SetDriftBaseline records the files the base branch has now as the state the project accepts.
func (q *Queries) SetDriftBaseline(ctx context.Context, projectID string, files map[string]string) error {
	b, err := json.Marshal(files)
	if err != nil {
		return err
	}
	ct, err := q.pool.Exec(ctx, `UPDATE project_repos SET drift_baseline = $2, drift_baseline_at = now(), updated_at = now() WHERE project_id = $1`, projectID, b)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetDriftBaseline returns (nil, nil) when none was recorded yet.
func (q *Queries) GetDriftBaseline(ctx context.Context, projectID string) (*DriftBaseline, error) {
	var raw []byte
	var at *time.Time
	err := q.pool.QueryRow(ctx, `SELECT drift_baseline, drift_baseline_at FROM project_repos WHERE project_id = $1`, projectID).Scan(&raw, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil || raw == nil || at == nil {
		return nil, err
	}
	var files map[string]string
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil, err
	}
	return &DriftBaseline{Files: files, At: *at}, nil
}

// Dismissal is one drift item a person chose to hide, with their note.
type Dismissal struct {
	Key  string    `json:"key"`
	Note string    `json:"note"`
	By   string    `json:"by"`
	At   time.Time `json:"at"`
}

// DismissDrift hides an item (again with a new note when it already is).
func (q *Queries) DismissDrift(ctx context.Context, projectID, key, note, by string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO drift_dismissals (project_id, item_key, note, by_oid) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (project_id, item_key) DO UPDATE SET note = EXCLUDED.note, by_oid = EXCLUDED.by_oid, at = now()`,
		projectID, key, note, by)
	return err
}

// RestoreDrift shows a dismissed item again.
func (q *Queries) RestoreDrift(ctx context.Context, projectID, key string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM drift_dismissals WHERE project_id = $1 AND item_key = $2`, projectID, key)
	return err
}

// ListDriftDismissals returns the project's dismissed items.
func (q *Queries) ListDriftDismissals(ctx context.Context, projectID string) ([]Dismissal, error) {
	rows, err := q.pool.Query(ctx, `SELECT item_key AS key, note, by_oid AS by, at FROM drift_dismissals WHERE project_id = $1 ORDER BY item_key`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Dismissal])
}
