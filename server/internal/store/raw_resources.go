package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RawResource is a raw provider resource an admin prepared for developers.
type RawResource struct {
	Name          string                 `json:"name"`
	Title         string                 `json:"title"`
	ResourceType  string                 `json:"resource_type"`
	Description   string                 `json:"description"`
	DefaultValues map[string]interface{} `json:"default_values"`
	TargetFolder  string                 `json:"target_folder"`
	UpdatedBy     string                 `json:"updated_by"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ErrRawResourceExists means a template with that name is already there.
var ErrRawResourceExists = errors.New("a raw resource with this name already exists")

// ListRawResources returns every template by title.
func (q *Queries) ListRawResources(ctx context.Context) ([]RawResource, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT name, title, resource_type, description, default_values, target_folder, updated_by, updated_at
		 FROM raw_resource_templates ORDER BY lower(title), name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RawResource{}
	for rows.Next() {
		var r RawResource
		var vals []byte
		if err := rows.Scan(&r.Name, &r.Title, &r.ResourceType, &r.Description, &vals, &r.TargetFolder, &r.UpdatedBy, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.DefaultValues = map[string]interface{}{}
		if err := json.Unmarshal(vals, &r.DefaultValues); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateRawResource inserts a template; ErrRawResourceExists when the name is taken.
func (q *Queries) CreateRawResource(ctx context.Context, r RawResource) error {
	vals, err := json.Marshal(r.DefaultValues)
	if err != nil {
		return err
	}
	_, err = q.pool.Exec(ctx,
		`INSERT INTO raw_resource_templates (name, title, resource_type, description, default_values, target_folder, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		r.Name, r.Title, r.ResourceType, r.Description, vals, r.TargetFolder, r.UpdatedBy)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrRawResourceExists
	}
	return err
}

// UpdateRawResource replaces a template's fields; pgx.ErrNoRows when it does not exist.
func (q *Queries) UpdateRawResource(ctx context.Context, r RawResource) error {
	vals, err := json.Marshal(r.DefaultValues)
	if err != nil {
		return err
	}
	tag, err := q.pool.Exec(ctx,
		`UPDATE raw_resource_templates SET title = $2, resource_type = $3, description = $4,
		   default_values = $5, target_folder = $6, updated_by = $7, updated_at = now()
		 WHERE name = $1`,
		r.Name, r.Title, r.ResourceType, r.Description, vals, r.TargetFolder, r.UpdatedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteRawResource removes a template; pgx.ErrNoRows when it does not exist.
// Nodes already on a canvas keep working: they carry their own type and values.
func (q *Queries) DeleteRawResource(ctx context.Context, name string) error {
	tag, err := q.pool.Exec(ctx, `DELETE FROM raw_resource_templates WHERE name = $1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
