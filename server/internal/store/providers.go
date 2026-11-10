package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Provider is a cloud provider as data (D-058).
type Provider struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Source   string   `json:"source"`
	Logo     string   `json:"logo"`
	Versions []string `json:"versions"`
	// Block is written to providers.tf after required_providers (azurerm needs `features {}`).
	Block     string    `json:"block"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	// ErrProviderExists: a provider with that id is already registered.
	ErrProviderExists = errors.New("a provider with this id already exists")
	// ErrProviderInUse: raw resources still belong to the provider.
	ErrProviderInUse = errors.New("raw resources still use this provider")
)

const providerColumns = `id, name, source, logo, versions, block, updated_at`

func scanProvider(row pgx.Row) (Provider, error) {
	var p Provider
	err := row.Scan(&p.ID, &p.Name, &p.Source, &p.Logo, &p.Versions, &p.Block, &p.UpdatedAt)
	if p.Versions == nil {
		p.Versions = []string{}
	}
	return p, err
}

// ListProviders returns every provider by name.
func (q *Queries) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+providerColumns+` FROM providers ORDER BY lower(name), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Provider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProvider registers a provider; ErrProviderExists when the id is taken.
func (q *Queries) CreateProvider(ctx context.Context, p Provider) error {
	if p.Versions == nil {
		p.Versions = []string{}
	}
	_, err := q.pool.Exec(ctx,
		`INSERT INTO providers (id, name, source, logo, versions, block) VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.Name, p.Source, p.Logo, p.Versions, p.Block)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrProviderExists
	}
	return err
}

// UpdateProvider replaces a provider's fields; pgx.ErrNoRows when it does not exist.
func (q *Queries) UpdateProvider(ctx context.Context, p Provider) error {
	if p.Versions == nil {
		p.Versions = []string{}
	}
	tag, err := q.pool.Exec(ctx,
		`UPDATE providers SET name = $2, source = $3, logo = $4, versions = $5, block = $6, updated_at = now() WHERE id = $1`,
		p.ID, p.Name, p.Source, p.Logo, p.Versions, p.Block)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteProvider removes a provider that no raw resource uses.
func (q *Queries) DeleteProvider(ctx context.Context, id string) error {
	var n int
	if err := q.pool.QueryRow(ctx, `SELECT count(*)::int FROM raw_resource_templates WHERE provider = $1`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrProviderInUse
	}
	tag, err := q.pool.Exec(ctx, `DELETE FROM providers WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
