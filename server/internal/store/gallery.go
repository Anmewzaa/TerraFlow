package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Gallery template scopes and statuses (8.16).
const (
	GalleryPrivate = "private"
	GalleryPublic  = "public"

	GalleryApproved  = "approved"
	GalleryPending   = "pending"
	GalleryRejected  = "rejected"
	GalleryHidden    = "hidden"
	GalleryWithdrawn = "withdrawn"
)

// ErrGalleryExists: this author already has a gallery template of this name.
var ErrGalleryExists = errors.New("you already have a gallery template with this name")

// GalleryTemplate is a template on the shared shelf. Template (the document) is set by Get only.
type GalleryTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Author      string          `json:"author"`
	AuthorName  string          `json:"author_name"`
	Scope       string          `json:"scope"`
	Status      string          `json:"status"`
	ReviewNote  string          `json:"review_note,omitempty"`
	ReviewedBy  string          `json:"reviewed_by,omitempty"`
	Version     int             `json:"version"`
	Modules     []string        `json:"modules"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Imports     int             `json:"imports"`
	Projects    int             `json:"projects"`
	Template    json.RawMessage `json:"template,omitempty"`
}

// GalleryFilter chooses what a list returns. The visibility rule lives in the query so that
// no caller can forget it: approved public templates, the viewer's own, and everything for an admin.
type GalleryFilter struct {
	Viewer   string
	Admin    bool
	Query    string // matches name, description, author name and module names
	Category string
	Status   string // admin review queue: "pending"
	Mine     bool   // only the viewer's own
}

const galleryCols = `g.id, g.name, g.description, g.category, g.author, g.author_name, g.scope, g.status,
	g.review_note, g.reviewed_by, g.version, g.modules, g.created_at, g.updated_at,
	(SELECT count(*) FROM gallery_imports i WHERE i.template_id = g.id),
	(SELECT count(DISTINCT i.project_id) FROM gallery_imports i WHERE i.template_id = g.id)`

func scanGallery(r pgx.Row, withDoc bool) (GalleryTemplate, error) {
	var g GalleryTemplate
	var doc []byte
	dst := []any{&g.ID, &g.Name, &g.Description, &g.Category, &g.Author, &g.AuthorName, &g.Scope, &g.Status,
		&g.ReviewNote, &g.ReviewedBy, &g.Version, &g.Modules, &g.CreatedAt, &g.UpdatedAt, &g.Imports, &g.Projects}
	if withDoc {
		dst = append(dst, &doc)
	}
	if err := r.Scan(dst...); err != nil {
		return GalleryTemplate{}, err
	}
	if g.Modules == nil {
		g.Modules = []string{}
	}
	g.Template = doc
	return g, nil
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// CreateGalleryTemplate stores a template on the shelf. The caller sets Scope and Status.
func (q *Queries) CreateGalleryTemplate(ctx context.Context, g GalleryTemplate) (GalleryTemplate, error) {
	if g.Modules == nil {
		g.Modules = []string{}
	}
	row := q.pool.QueryRow(ctx, `
		WITH g AS (
		  INSERT INTO gallery_templates (name, description, category, author, author_name, scope, status, template, modules)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		  RETURNING *)
		SELECT `+galleryCols+` FROM g`,
		g.Name, g.Description, g.Category, g.Author, g.AuthorName, g.Scope, g.Status, []byte(g.Template), g.Modules)
	out, err := scanGallery(row, false)
	if isUnique(err) {
		return GalleryTemplate{}, ErrGalleryExists
	}
	return out, err
}

// ListGalleryTemplates lists the shelf as the filter allows, without documents, newest first.
func (q *Queries) ListGalleryTemplates(ctx context.Context, f GalleryFilter) ([]GalleryTemplate, error) {
	where := []string{}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if !f.Admin || f.Mine {
		v := arg(f.Viewer)
		if f.Mine {
			where = append(where, "g.author = "+v)
		} else {
			where = append(where, "((g.scope = 'public' AND g.status = 'approved') OR g.author = "+v+")")
		}
	}
	if f.Status != "" {
		where = append(where, "g.status = "+arg(f.Status))
	}
	if f.Category != "" {
		where = append(where, "g.category = "+arg(f.Category))
	}
	if f.Query != "" {
		p := arg("%" + escapeLike(strings.ToLower(f.Query)) + "%")
		where = append(where, "(lower(g.name) LIKE "+p+" OR lower(g.description) LIKE "+p+" OR lower(g.author_name) LIKE "+p+
			" OR lower(array_to_string(g.modules, ' ')) LIKE "+p+")")
	}
	sql := `SELECT ` + galleryCols + ` FROM gallery_templates g`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += ` ORDER BY g.updated_at DESC, g.name LIMIT 500`
	rows, err := q.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (GalleryTemplate, error) { return scanGallery(r, false) })
	if out == nil {
		out = []GalleryTemplate{}
	}
	return out, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// GetGalleryTemplate returns one template with its document; pgx.ErrNoRows when absent. The caller
// decides who may see it.
func (q *Queries) GetGalleryTemplate(ctx context.Context, id string) (GalleryTemplate, error) {
	return scanGallery(q.pool.QueryRow(ctx, `SELECT `+galleryCols+`, g.template FROM gallery_templates g WHERE g.id = $1`, id), true)
}

// UpdateGalleryTemplate replaces what the author may edit and raises the version. The caller sets
// Scope and Status (a public template goes back to pending).
func (q *Queries) UpdateGalleryTemplate(ctx context.Context, id string, g GalleryTemplate) error {
	if g.Modules == nil {
		g.Modules = []string{}
	}
	ct, err := q.pool.Exec(ctx, `
		UPDATE gallery_templates SET name = $2, description = $3, category = $4, scope = $5, status = $6,
		  template = $7, modules = $8, review_note = '', reviewed_by = '', version = version + 1, updated_at = now()
		WHERE id = $1`, id, g.Name, g.Description, g.Category, g.Scope, g.Status, []byte(g.Template), g.Modules)
	if isUnique(err) {
		return ErrGalleryExists
	}
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SetGalleryStatus changes the status (review, hide, withdraw) and records who and why.
func (q *Queries) SetGalleryStatus(ctx context.Context, id, status, note, by string) error {
	ct, err := q.pool.Exec(ctx, `UPDATE gallery_templates SET status = $2, review_note = $3, reviewed_by = $4, updated_at = now() WHERE id = $1`,
		id, status, note, by)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteGalleryTemplate removes a template and its import records.
func (q *Queries) DeleteGalleryTemplate(ctx context.Context, id string) error {
	ct, err := q.pool.Exec(ctx, `DELETE FROM gallery_templates WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// RecordGalleryImport notes that a template was imported into a project.
func (q *Queries) RecordGalleryImport(ctx context.Context, id, projectID, by string) error {
	_, err := q.pool.Exec(ctx, `INSERT INTO gallery_imports (template_id, project_id, imported_by) VALUES ($1, $2, $3)`, id, projectID, by)
	return err
}
