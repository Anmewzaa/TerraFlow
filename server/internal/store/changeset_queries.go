package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

type ChangesetStatus string

const (
	ChangesetDraft     ChangesetStatus = "draft"
	ChangesetPublished ChangesetStatus = "published"
	ChangesetApproved  ChangesetStatus = "approved"
	ChangesetRejected  ChangesetStatus = "rejected"
)

type Changeset struct {
	ID           string          `json:"id"            db:"id"`
	ProjectID    string          `json:"project_id"    db:"project_id"`
	Env          string          `json:"env"           db:"env"`
	UserOID      string          `json:"user_oid"      db:"user_oid"`
	Title        string          `json:"title"         db:"title"`
	Status       ChangesetStatus `json:"status"        db:"status"`
	CanvasJSON   json.RawMessage `json:"canvas"        db:"canvas_json"`
	BaseCanvas   json.RawMessage `json:"base_canvas"   db:"base_canvas_json"`
	RejectReason *string         `json:"reject_reason" db:"reject_reason"`
	ReviewerOID  *string         `json:"reviewer_oid"  db:"reviewer_oid"`
	ReviewedAt   *time.Time      `json:"reviewed_at"   db:"reviewed_at"`
	BranchName   *string         `json:"branch_name"   db:"branch_name"`
	CommitSHA    *string         `json:"commit_sha"    db:"commit_sha"`
	CreatedAt    time.Time       `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"    db:"updated_at"`
}

// CanvasState is the JSONB shape stored in canvas_json.
type CanvasState struct {
	Nodes []CanvasNode `json:"nodes"`
	Edges []CanvasEdge `json:"edges"`
}

// CanvasNode represents a module instance added by the developer.
type CanvasNode struct {
	ID           string                 `json:"id"`
	ModuleName   string                 `json:"module_name"`
	ModuleVersion string                `json:"module_version"`
	InstanceName string                 `json:"instance_name"`
	TargetFolder string                 `json:"target_folder"`
	Values       map[string]interface{} `json:"values"`
	PositionX    float64                `json:"position_x"`
	PositionY    float64                `json:"position_y"`
	// ParentID places the node inside a container node; NestField (from the
	// module schema) folds it into the parent's input map when generating.
	ParentID  string `json:"parent_id,omitempty"`
	NestField string `json:"nest_field,omitempty"`
	// Color is the container colour key chosen in the UI.
	Color string `json:"color,omitempty"`
	// Filename is the .tf file this resource is written to (default main.tf).
	Filename string `json:"filename,omitempty"`
	// Existing marks a node that edits a module call already in the repository
	// (folder, file and label); the generator replaces only that block.
	Existing *ExistingRef `json:"existing,omitempty"`
	// Kind is "resource" for a raw provider resource (D-040); ResourceType is
	// then e.g. azurerm_resource_group. Absent or "module" = a module call.
	Kind         string `json:"kind,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

// ExistingRef locates a module call that already exists in the repository.
type ExistingRef struct {
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Label    string `json:"label"`
}

// CanvasEdge represents a reference between draft nodes or to existing resources.
type CanvasEdge struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

func (q *Queries) ListChangesets(ctx context.Context, projectID, userOID string) ([]Changeset, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, project_id, env, user_oid, title, status, canvas_json,
		        reject_reason, reviewer_oid, reviewed_at, branch_name, commit_sha, base_canvas_json, created_at, updated_at
		 FROM changesets
		 WHERE project_id = $1 AND (user_oid = $2 OR status != 'draft')
		 ORDER BY updated_at DESC`, projectID, userOID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Changeset])
}

func (q *Queries) GetChangeset(ctx context.Context, id string) (Changeset, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, project_id, env, user_oid, title, status, canvas_json,
		        reject_reason, reviewer_oid, reviewed_at, branch_name, commit_sha, base_canvas_json, created_at, updated_at
		 FROM changesets WHERE id = $1`, id)
	if err != nil {
		return Changeset{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Changeset])
}

func (q *Queries) CreateChangeset(ctx context.Context, c Changeset) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO changesets (project_id, env, user_oid, title, canvas_json)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		c.ProjectID, c.Env, c.UserOID, c.Title, c.CanvasJSON).Scan(&id)
	return id, err
}

func (q *Queries) UpdateChangesetCanvas(ctx context.Context, id string, canvas json.RawMessage) error {
	ct, err := q.pool.Exec(ctx,
		`UPDATE changesets SET canvas_json = $2, updated_at = now()
		 WHERE id = $1 AND status = 'draft'`, id, canvas)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (q *Queries) UpdateChangesetStatus(ctx context.Context, id string, status ChangesetStatus) error {
	ct, err := q.pool.Exec(ctx,
		`UPDATE changesets SET status = $2, updated_at = now()
		 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// PendingReviewError means another changeset of the project is still waiting
// for review, so a new one cannot be published yet (D-043).
type PendingReviewError struct {
	ID    string
	Title string
}

func (e *PendingReviewError) Error() string {
	t := e.Title
	if t == "" {
		t = "untitled"
	}
	return "changeset \"" + t + "\" is still waiting for review; approve or reject it before publishing another"
}

// PublishChangeset moves a draft to published. Only one changeset per project
// can wait for review at a time: while one does, this returns *PendingReviewError.
func (q *Queries) PublishChangeset(ctx context.Context, id, title string) error {
	ct, err := q.pool.Exec(ctx,
		`UPDATE changesets c SET status = 'published', title = $2, updated_at = now()
		 WHERE c.id = $1 AND c.status = 'draft'
		   AND NOT EXISTS (SELECT 1 FROM changesets o WHERE o.project_id = c.project_id AND o.status = 'published')`,
		id, title)
	if err != nil {
		return err
	}
	if ct.RowsAffected() > 0 {
		return nil
	}
	// Nothing changed: either the changeset is not a draft, or another one is pending.
	var pend PendingReviewError
	err = q.pool.QueryRow(ctx,
		`SELECT o.id, o.title FROM changesets o
		 WHERE o.status = 'published'
		   AND o.project_id = (SELECT project_id FROM changesets WHERE id = $1 AND status = 'draft')
		 ORDER BY o.updated_at LIMIT 1`, id).Scan(&pend.ID, &pend.Title)
	if err == nil {
		return &pend
	}
	return pgx.ErrNoRows
}

func (q *Queries) DeleteChangeset(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx,
		`DELETE FROM changesets WHERE id = $1 AND status = 'draft'`, id)
	return err
}

// CreateRevision copies a reviewed changeset into a new draft whose base is the
// source canvas, so edits can be diffed against it.
func (q *Queries) CreateRevision(ctx context.Context, src Changeset, userOID string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO changesets (project_id, env, user_oid, title, canvas_json, base_canvas_json)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 RETURNING id`,
		src.ProjectID, src.Env, userOID, src.Title, src.CanvasJSON).Scan(&id)
	return id, err
}
