package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

type CatalogModule struct {
	ID           string          `json:"id"            db:"id"`
	Name         string          `json:"name"          db:"name"`
	Version      string          `json:"version"       db:"version"`
	Title        string          `json:"title"         db:"title"`
	Description  string          `json:"description"   db:"description"`
	Icon         string          `json:"icon"          db:"icon"`
	TargetFolder string          `json:"target_folder" db:"target_folder"`
	SchemaJSON   json.RawMessage `json:"schema_json"   db:"schema_json"`
	Checksum     string          `json:"checksum"      db:"checksum"`
	CreatedAt    time.Time       `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"    db:"updated_at"`
}

type CatalogDraft struct {
	ID         string          `json:"id"          db:"id"`
	ProjectID  string          `json:"project_id"  db:"project_id"`
	ModuleID   string          `json:"module_id"   db:"module_id"`
	UserOID    string          `json:"user_oid"    db:"user_oid"`
	ValuesJSON json.RawMessage `json:"values_json" db:"values_json"`
	CreatedAt  time.Time       `json:"created_at"  db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"  db:"updated_at"`
}

func (q *Queries) ListCatalogModules(ctx context.Context) ([]CatalogModule, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, name, version, title, description, icon, target_folder, schema_json, checksum, created_at, updated_at
		 FROM catalog_modules ORDER BY name, version`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[CatalogModule])
}

func (q *Queries) GetCatalogModule(ctx context.Context, name, version string) (CatalogModule, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, name, version, title, description, icon, target_folder, schema_json, checksum, created_at, updated_at
		 FROM catalog_modules WHERE name = $1 AND version = $2`, name, version)
	if err != nil {
		return CatalogModule{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CatalogModule])
}

func (q *Queries) GetCatalogModuleByID(ctx context.Context, id string) (CatalogModule, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, name, version, title, description, icon, target_folder, schema_json, checksum, created_at, updated_at
		 FROM catalog_modules WHERE id = $1`, id)
	if err != nil {
		return CatalogModule{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CatalogModule])
}

func (q *Queries) UpsertCatalogModule(ctx context.Context, m CatalogModule) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO catalog_modules (name, version, title, description, icon, target_folder, schema_json, checksum)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (name, version) DO UPDATE SET
		   title = EXCLUDED.title, description = EXCLUDED.description,
		   icon = EXCLUDED.icon, target_folder = EXCLUDED.target_folder,
		   schema_json = EXCLUDED.schema_json, checksum = EXCLUDED.checksum,
		   updated_at = now()`,
		m.Name, m.Version, m.Title, m.Description, m.Icon, m.TargetFolder, m.SchemaJSON, m.Checksum)
	return err
}

func (q *Queries) ListCatalogDrafts(ctx context.Context, projectID, userOID string) ([]CatalogDraft, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, project_id, module_id, user_oid, values_json, created_at, updated_at
		 FROM catalog_drafts WHERE project_id = $1 AND user_oid = $2
		 ORDER BY updated_at DESC`, projectID, userOID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[CatalogDraft])
}

func (q *Queries) GetCatalogDraft(ctx context.Context, projectID, moduleID, userOID string) (CatalogDraft, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, project_id, module_id, user_oid, values_json, created_at, updated_at
		 FROM catalog_drafts WHERE project_id = $1 AND module_id = $2 AND user_oid = $3`, projectID, moduleID, userOID)
	if err != nil {
		return CatalogDraft{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CatalogDraft])
}

func (q *Queries) SaveCatalogDraft(ctx context.Context, d CatalogDraft) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO catalog_drafts (project_id, module_id, user_oid, values_json)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (project_id, module_id, user_oid) DO UPDATE SET
		   values_json = EXCLUDED.values_json, updated_at = now()`,
		d.ProjectID, d.ModuleID, d.UserOID, d.ValuesJSON)
	return err
}

func (q *Queries) DeleteCatalogDraft(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM catalog_drafts WHERE id = $1`, id)
	return err
}
