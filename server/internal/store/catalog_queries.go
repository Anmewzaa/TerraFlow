package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type CatalogModule struct {
	ID string `json:"id"            db:"id"`
	// ProjectID is the project the module belongs to; "" is the global catalog (D-060).
	ProjectID    string          `json:"project_id"    db:"project_id"`
	Name         string          `json:"name"          db:"name"`
	Version      string          `json:"version"       db:"version"`
	Title        string          `json:"title"         db:"title"`
	Description  string          `json:"description"   db:"description"`
	Icon         string          `json:"icon"          db:"icon"`
	TargetFolder string          `json:"target_folder" db:"target_folder"`
	SchemaJSON   json.RawMessage `json:"schema_json"   db:"schema_json"`
	Checksum     string          `json:"checksum"      db:"checksum"`
	Status       string          `json:"status"        db:"status"`
	ImportedBy   string          `json:"imported_by"   db:"imported_by"`
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

const catalogModuleCols = `id, COALESCE(project_id::text, '') AS project_id, name, version, title, description, icon,
	target_folder, schema_json, checksum, status, imported_by, created_at, updated_at`

// scopeArg is the project_id parameter of a scope: NULL for the global catalog.
func scopeArg(projectID string) any {
	if projectID == "" {
		return nil
	}
	return projectID
}

// ListCatalogModules returns the global catalog.
func (q *Queries) ListCatalogModules(ctx context.Context) ([]CatalogModule, error) {
	return q.ListCatalogModulesIn(ctx, "")
}

// ListCatalogModulesIn returns the modules of one scope: a project's own, or the global ones for "".
func (q *Queries) ListCatalogModulesIn(ctx context.Context, projectID string) ([]CatalogModule, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+catalogModuleCols+` FROM catalog_modules
		 WHERE project_id IS NOT DISTINCT FROM $1::uuid ORDER BY name, version`, scopeArg(projectID))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[CatalogModule])
}

// GetCatalogModule reads a module of the global catalog.
func (q *Queries) GetCatalogModule(ctx context.Context, name, version string) (CatalogModule, error) {
	return q.GetCatalogModuleIn(ctx, "", name, version)
}

// GetCatalogModuleIn reads a module of exactly one scope (it does not fall back to the global catalog).
func (q *Queries) GetCatalogModuleIn(ctx context.Context, projectID, name, version string) (CatalogModule, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+catalogModuleCols+` FROM catalog_modules
		 WHERE project_id IS NOT DISTINCT FROM $1::uuid AND name = $2 AND version = $3`, scopeArg(projectID), name, version)
	if err != nil {
		return CatalogModule{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CatalogModule])
}

func (q *Queries) GetCatalogModuleByID(ctx context.Context, id string) (CatalogModule, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+catalogModuleCols+` FROM catalog_modules WHERE id = $1`, id)
	if err != nil {
		return CatalogModule{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CatalogModule])
}

// UpsertCatalogModule refreshes a module of the global catalog from the seed files.
func (q *Queries) UpsertCatalogModule(ctx context.Context, m CatalogModule) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO catalog_modules (name, version, title, description, icon, target_folder, schema_json, checksum)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (name, version) WHERE project_id IS NULL DO UPDATE SET
		   title = EXCLUDED.title, description = EXCLUDED.description,
		   icon = EXCLUDED.icon, target_folder = EXCLUDED.target_folder,
		   schema_json = EXCLUDED.schema_json, checksum = EXCLUDED.checksum,
		   updated_at = now()`,
		m.Name, m.Version, m.Title, m.Description, m.Icon, m.TargetFolder, m.SchemaJSON, m.Checksum)
	return err
}

// ErrModuleExists is returned by InsertCatalogModule for a name+version that exists.
var ErrModuleExists = errors.New("module version already exists")

// InsertCatalogModule adds a new module version to the scope in m.ProjectID; it never overwrites one.
func (q *Queries) InsertCatalogModule(ctx context.Context, m CatalogModule) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO catalog_modules (project_id, name, version, title, description, icon, target_folder, schema_json, checksum, imported_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		scopeArg(m.ProjectID), m.Name, m.Version, m.Title, m.Description, m.Icon, m.TargetFolder, m.SchemaJSON, m.Checksum, m.ImportedBy)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrModuleExists
	}
	return err
}

// SetCatalogModuleStatus publishes or deprecates a global version (pgx.ErrNoRows if unknown).
func (q *Queries) SetCatalogModuleStatus(ctx context.Context, name, version, status string) error {
	return q.SetCatalogModuleStatusIn(ctx, "", name, version, status)
}

// SetCatalogModuleStatusIn publishes or deprecates a version of one scope (pgx.ErrNoRows if unknown).
func (q *Queries) SetCatalogModuleStatusIn(ctx context.Context, projectID, name, version, status string) error {
	ct, err := q.pool.Exec(ctx,
		`UPDATE catalog_modules SET status = $4, updated_at = now()
		 WHERE project_id IS NOT DISTINCT FROM $1::uuid AND name = $2 AND version = $3`,
		scopeArg(projectID), name, version, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
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
