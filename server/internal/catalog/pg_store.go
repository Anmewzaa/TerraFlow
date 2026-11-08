package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type PGStore struct {
	q *store.Queries
}

func NewPGStore(q *store.Queries) *PGStore {
	return &PGStore{q: q}
}

func (s *PGStore) ListModules(ctx context.Context) ([]Module, error) {
	rows, err := s.q.ListCatalogModules(ctx)
	if err != nil {
		return nil, err
	}
	modules := make([]Module, len(rows))
	for i, r := range rows {
		modules[i] = fromDBModule(r)
	}
	return modules, nil
}

func (s *PGStore) GetModule(ctx context.Context, name, version string) (Module, error) {
	r, err := s.q.GetCatalogModule(ctx, name, version)
	if err != nil {
		return Module{}, err
	}
	return fromDBModule(r), nil
}

func (s *PGStore) GetModuleByID(ctx context.Context, id string) (Module, error) {
	r, err := s.q.GetCatalogModuleByID(ctx, id)
	if err != nil {
		return Module{}, err
	}
	return fromDBModule(r), nil
}

func (s *PGStore) UpsertModule(ctx context.Context, m Module) error {
	return s.q.UpsertCatalogModule(ctx, store.CatalogModule{
		Name:         m.Name,
		Version:      m.Version,
		Title:        m.Title,
		Description:  m.Description,
		Icon:         m.Icon,
		TargetFolder: m.TargetFolder,
		SchemaJSON:   m.SchemaJSON,
		Checksum:     m.Checksum,
	})
}

func (s *PGStore) InsertModule(ctx context.Context, m Module) error {
	err := s.q.InsertCatalogModule(ctx, store.CatalogModule{
		Name: m.Name, Version: m.Version, Title: m.Title, Description: m.Description, Icon: m.Icon,
		TargetFolder: m.TargetFolder, SchemaJSON: m.SchemaJSON, Checksum: m.Checksum, ImportedBy: m.ImportedBy,
	})
	if errors.Is(err, store.ErrModuleExists) {
		return ErrModuleExists
	}
	return err
}

func (s *PGStore) SetModuleStatus(ctx context.Context, name, version, status string) error {
	err := s.q.SetCatalogModuleStatus(ctx, name, version, status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrModuleNotFound
	}
	return err
}

func (s *PGStore) ListDrafts(ctx context.Context, projectID, userOID string) ([]Draft, error) {
	rows, err := s.q.ListCatalogDrafts(ctx, projectID, userOID)
	if err != nil {
		return nil, err
	}
	drafts := make([]Draft, len(rows))
	for i, r := range rows {
		drafts[i] = fromDBDraft(r)
	}
	return drafts, nil
}

func (s *PGStore) GetDraft(ctx context.Context, projectID, moduleID, userOID string) (Draft, error) {
	r, err := s.q.GetCatalogDraft(ctx, projectID, moduleID, userOID)
	if err != nil {
		return Draft{}, err
	}
	return fromDBDraft(r), nil
}

func (s *PGStore) SaveDraft(ctx context.Context, d Draft) error {
	return s.q.SaveCatalogDraft(ctx, store.CatalogDraft{
		ProjectID:  d.ProjectID,
		ModuleID:   d.ModuleID,
		UserOID:    d.UserOID,
		ValuesJSON: json.RawMessage(d.ValuesJSON),
	})
}

func (s *PGStore) DeleteDraft(ctx context.Context, id string) error {
	return s.q.DeleteCatalogDraft(ctx, id)
}

func fromDBModule(r store.CatalogModule) Module {
	return Module{
		ID:           r.ID,
		Name:         r.Name,
		Version:      r.Version,
		Title:        r.Title,
		Description:  r.Description,
		Icon:         r.Icon,
		TargetFolder: r.TargetFolder,
		SchemaJSON:   r.SchemaJSON,
		Checksum:     r.Checksum,
		Status:       r.Status,
		ImportedBy:   r.ImportedBy,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func fromDBDraft(r store.CatalogDraft) Draft {
	return Draft{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		ModuleID:   r.ModuleID,
		UserOID:    r.UserOID,
		ValuesJSON: r.ValuesJSON,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}
