package store

import (
	"context"
	"time"
)

// ComponentConfig is app-side presentation for a component that lives in the
// repository. Address is "<folder>/<module label>".
type ComponentConfig struct {
	Address   string    `json:"address"`
	Color     string    `json:"color"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListComponentConfig returns every setting of a project, by address.
func (q *Queries) ListComponentConfig(ctx context.Context, projectID string) ([]ComponentConfig, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT address, color, updated_by, updated_at FROM project_component_config
		 WHERE project_id = $1 ORDER BY address`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ComponentConfig{}
	for rows.Next() {
		var c ComponentConfig
		if err := rows.Scan(&c.Address, &c.Color, &c.UpdatedBy, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetComponentConfig stores a colour; an empty colour removes the setting.
func (q *Queries) SetComponentConfig(ctx context.Context, projectID, address, color, by string) error {
	if color == "" {
		_, err := q.pool.Exec(ctx,
			`DELETE FROM project_component_config WHERE project_id = $1 AND address = $2`, projectID, address)
		return err
	}
	_, err := q.pool.Exec(ctx,
		`INSERT INTO project_component_config (project_id, address, color, updated_by) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (project_id, address) DO UPDATE SET color = EXCLUDED.color,
		   updated_by = EXCLUDED.updated_by, updated_at = now()`,
		projectID, address, color, by)
	return err
}
