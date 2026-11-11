package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

// --- Projects ---

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	// EntraGroupID is unused since D-032 (project visibility is assigned in the app).
	EntraGroupID *string   `json:"entra_group_id,omitempty"`
	Description  string    `json:"description"`
	IACPath      string    `json:"iac_path"`
	SourceType   string    `json:"source_type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (q *Queries) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, name, slug, entra_group_id, description, iac_path, source_type, created_at, updated_at
		 FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Project])
}

func (q *Queries) GetProject(ctx context.Context, id string) (Project, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, name, slug, entra_group_id, description, iac_path, source_type, created_at, updated_at
		 FROM projects WHERE id = $1`, id)
	if err != nil {
		return Project{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Project])
}

func (q *Queries) UpsertProject(ctx context.Context, p Project) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO projects (id, name, slug, entra_group_id, iac_path, source_type)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (slug) DO UPDATE SET
		   name = EXCLUDED.name, entra_group_id = EXCLUDED.entra_group_id,
		   iac_path = EXCLUDED.iac_path, source_type = EXCLUDED.source_type,
		   updated_at = now()`,
		p.ID, p.Name, p.Slug, p.EntraGroupID, p.IACPath, p.SourceType)
	return err
}

// --- Sessions ---

type Session struct {
	ID        string    `json:"id"`
	UserOID   string    `json:"user_oid"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Roles     []string  `json:"roles"`
	GroupIDs  []string  `json:"group_ids"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// sessionKey is what the database stores for a session id. The browser holds the
// id; the table holds only its SHA-256, so a leaked backup or SQL read cannot be
// replayed as a cookie.
func sessionKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

// CreateSession stores a session under the hash of s.ID and drops expired ones.
func (q *Queries) CreateSession(ctx context.Context, s Session) error {
	// Expired rows are useless; purging here keeps the table small without a job.
	if _, err := q.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`); err != nil {
		return err
	}
	_, err := q.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_oid, email, name, roles, group_ids, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sessionKey(s.ID), s.UserOID, s.Email, s.Name, s.Roles, s.GroupIDs, s.ExpiresAt)
	return err
}

func (q *Queries) GetSession(ctx context.Context, id string) (Session, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_oid, email, name, roles, group_ids, expires_at, created_at
		 FROM sessions WHERE id = $1 AND expires_at > now()`, sessionKey(id))
	if err != nil {
		return Session{}, err
	}
	sess, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Session])
	sess.ID = id // callers know the session by the id the browser holds
	return sess, err
}

func (q *Queries) DeleteSession(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionKey(id))
	return err
}

// --- Audit ---

func (q *Queries) LogAudit(ctx context.Context, userOID, action, resource string, detail any) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO audit_log (user_oid, action, resource, detail)
		 VALUES ($1, $2, $3, $4)`,
		userOID, action, resource, detail)
	return err
}

// ErrSlugTaken is returned by CreateProject when the slug already exists.
var ErrSlugTaken = errors.New("project slug already exists")

// CreateProject inserts a new project; it never overwrites an existing slug.
func (q *Queries) CreateProject(ctx context.Context, p Project) (Project, error) {
	rows, err := q.pool.Query(ctx,
		`INSERT INTO projects (name, slug, entra_group_id, iac_path, source_type)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, name, slug, entra_group_id, description, iac_path, source_type, created_at, updated_at`,
		p.Name, p.Slug, p.EntraGroupID, p.IACPath, p.SourceType)
	if err != nil {
		return Project{}, err
	}
	out, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Project])
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Project{}, ErrSlugTaken
	}
	return out, err
}

// DeleteProject removes the project row and its drafts/changesets.
// It never touches the IaC files on disk.
func (q *Queries) DeleteProject(ctx context.Context, id string) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, stmt := range []string{
		`DELETE FROM catalog_drafts WHERE project_id = $1`,
		// A draft of another project that points at one of this project's modules would block the cascade.
		`DELETE FROM catalog_drafts WHERE module_id IN (SELECT id FROM catalog_modules WHERE project_id = $1)`,
		`DELETE FROM changesets WHERE project_id = $1`,
	} {
		if _, err := tx.Exec(ctx, stmt, id); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}
