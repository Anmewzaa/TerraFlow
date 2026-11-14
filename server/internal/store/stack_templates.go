package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrStackTemplateExists: the project already has a template of this name.
var ErrStackTemplateExists = errors.New("a stack template with this name already exists")

// StackTemplate is a stored template; Template is the document (nodes, arrows, slots, parameters).
type StackTemplate struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Name      string          `json:"name"`
	CreatedBy string          `json:"created_by"`
	CreatedAt time.Time       `json:"created_at"`
	Template  json.RawMessage `json:"template,omitempty"`
}

// CreateStackTemplate stores a template and returns it; ErrStackTemplateExists when the name is taken.
func (q *Queries) CreateStackTemplate(ctx context.Context, projectID, name, createdBy string, doc json.RawMessage) (StackTemplate, error) {
	var t StackTemplate
	err := q.pool.QueryRow(ctx,
		`INSERT INTO stack_templates (project_id, name, created_by, template) VALUES ($1, $2, $3, $4)
		 RETURNING id, project_id, name, created_by, created_at`, projectID, name, createdBy, []byte(doc)).
		Scan(&t.ID, &t.ProjectID, &t.Name, &t.CreatedBy, &t.CreatedAt)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return StackTemplate{}, ErrStackTemplateExists
	}
	return t, err
}

// ListStackTemplates lists a project's templates without their documents, by name.
func (q *Queries) ListStackTemplates(ctx context.Context, projectID string) ([]StackTemplate, error) {
	rows, err := q.pool.Query(ctx, `SELECT id, project_id, name, created_by, created_at FROM stack_templates WHERE project_id = $1 ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (StackTemplate, error) {
		var t StackTemplate
		err := r.Scan(&t.ID, &t.ProjectID, &t.Name, &t.CreatedBy, &t.CreatedAt)
		return t, err
	})
}

// GetStackTemplate returns one template of the project, with its document; pgx.ErrNoRows when absent.
func (q *Queries) GetStackTemplate(ctx context.Context, projectID, id string) (StackTemplate, error) {
	var t StackTemplate
	var doc []byte
	err := q.pool.QueryRow(ctx, `SELECT id, project_id, name, created_by, created_at, template FROM stack_templates WHERE project_id = $1 AND id = $2`, projectID, id).
		Scan(&t.ID, &t.ProjectID, &t.Name, &t.CreatedBy, &t.CreatedAt, &doc)
	t.Template = doc
	return t, err
}

// DeleteStackTemplate removes a template; pgx.ErrNoRows when it is not the project's.
func (q *Queries) DeleteStackTemplate(ctx context.Context, projectID, id string) error {
	ct, err := q.pool.Exec(ctx, `DELETE FROM stack_templates WHERE project_id = $1 AND id = $2`, projectID, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
