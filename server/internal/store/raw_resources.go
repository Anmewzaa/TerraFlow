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
	// ProjectID is the project the template belongs to; "" is the global catalog (D-060).
	ProjectID     string                 `json:"project_id,omitempty"`
	Name          string                 `json:"name"`
	Title         string                 `json:"title"`
	ResourceType  string                 `json:"resource_type"`
	Description   string                 `json:"description"`
	DefaultValues map[string]interface{} `json:"default_values"`
	TargetFolder  string                 `json:"target_folder"`
	// Provider is the cloud provider the resource type belongs to (azurerm); Category groups it in
	// the palette; MinVersion and MaxVersion (major.minor, either may be empty) are the provider
	// versions it exists in (D-058).
	Provider   string    `json:"provider"`
	Category   string    `json:"category"`
	MinVersion string    `json:"min_version"`
	MaxVersion string    `json:"max_version"`
	UpdatedBy  string    `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ErrRawResourceExists means a template with that name is already there.
var ErrRawResourceExists = errors.New("a raw resource with this name already exists")

// ListRawResources returns the global templates by title.
func (q *Queries) ListRawResources(ctx context.Context) ([]RawResource, error) {
	return q.ListRawResourcesIn(ctx, "")
}

// ListRawResourcesIn returns the templates of one scope by title: a project's own, or the global ones for "".
func (q *Queries) ListRawResourcesIn(ctx context.Context, projectID string) ([]RawResource, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT COALESCE(project_id::text, ''), name, title, resource_type, description, default_values, target_folder,
		        provider, category, min_version, max_version, updated_by, updated_at
		 FROM raw_resource_templates WHERE project_id IS NOT DISTINCT FROM $1::uuid ORDER BY lower(title), name`, scopeArg(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RawResource{}
	for rows.Next() {
		var r RawResource
		var vals []byte
		if err := rows.Scan(&r.ProjectID, &r.Name, &r.Title, &r.ResourceType, &r.Description, &vals, &r.TargetFolder,
			&r.Provider, &r.Category, &r.MinVersion, &r.MaxVersion, &r.UpdatedBy, &r.UpdatedAt); err != nil {
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

// CreateRawResource inserts a template into the scope in r.ProjectID; ErrRawResourceExists when the name is taken.
func (q *Queries) CreateRawResource(ctx context.Context, r RawResource) error {
	vals, err := json.Marshal(r.DefaultValues)
	if err != nil {
		return err
	}
	_, err = q.pool.Exec(ctx,
		`INSERT INTO raw_resource_templates (name, title, resource_type, description, default_values, target_folder,
		   provider, category, min_version, max_version, updated_by, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		r.Name, r.Title, r.ResourceType, r.Description, vals, r.TargetFolder, r.Provider, r.Category, r.MinVersion, r.MaxVersion, r.UpdatedBy, scopeArg(r.ProjectID))
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrRawResourceExists
	}
	return err
}

// UpdateRawResource replaces the fields of a template of the scope in r.ProjectID; pgx.ErrNoRows when it does not exist.
func (q *Queries) UpdateRawResource(ctx context.Context, r RawResource) error {
	vals, err := json.Marshal(r.DefaultValues)
	if err != nil {
		return err
	}
	tag, err := q.pool.Exec(ctx,
		`UPDATE raw_resource_templates SET title = $2, resource_type = $3, description = $4,
		   default_values = $5, target_folder = $6, provider = $7, category = $8, min_version = $9, max_version = $10,
		   updated_by = $11, updated_at = now()
		 WHERE name = $1 AND project_id IS NOT DISTINCT FROM $12::uuid`,
		r.Name, r.Title, r.ResourceType, r.Description, vals, r.TargetFolder, r.Provider, r.Category, r.MinVersion, r.MaxVersion, r.UpdatedBy,
		scopeArg(r.ProjectID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteRawResource removes a global template; pgx.ErrNoRows when it does not exist.
// Nodes already on a canvas keep working: they carry their own type and values.
func (q *Queries) DeleteRawResource(ctx context.Context, name string) error {
	return q.DeleteRawResourceIn(ctx, "", name)
}

// DeleteRawResourceIn removes a template of one scope; pgx.ErrNoRows when it does not exist.
func (q *Queries) DeleteRawResourceIn(ctx context.Context, projectID, name string) error {
	tag, err := q.pool.Exec(ctx,
		`DELETE FROM raw_resource_templates WHERE name = $1 AND project_id IS NOT DISTINCT FROM $2::uuid`, name, scopeArg(projectID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
