package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FSStore struct {
	dir     string
	mu      sync.RWMutex
	modules []Module
	drafts  map[string]Draft // keyed by "projectID:moduleID:userOID"
	loaded  bool
}

func NewFSStore(dir string) *FSStore {
	return &FSStore{
		dir:    dir,
		drafts: make(map[string]Draft),
	}
}

func (s *FSStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return nil
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}

	for _, modEntry := range entries {
		if !modEntry.IsDir() {
			continue
		}
		modName := modEntry.Name()
		versions, err := os.ReadDir(filepath.Join(s.dir, modName))
		if err != nil {
			continue
		}
		for _, verEntry := range versions {
			if !verEntry.IsDir() {
				continue
			}
			version := verEntry.Name()
			data, err := os.ReadFile(filepath.Join(s.dir, modName, version, "schema.json"))
			if err != nil {
				continue
			}

			var schema struct {
				Title       string       `json:"title"`
				Description string       `json:"description"`
				Meta        TFModuleMeta `json:"x-tf-module"`
			}
			if err := json.Unmarshal(data, &schema); err != nil {
				continue
			}

			s.modules = append(s.modules, Module{
				ID:           fmt.Sprintf("%s-%s", modName, version),
				Name:         modName,
				Version:      version,
				Title:        schema.Title,
				Description:  schema.Description,
				Icon:         schema.Meta.Icon,
				TargetFolder: schema.Meta.TargetFolder,
				SchemaJSON:   json.RawMessage(data),
				Checksum:     fmt.Sprintf("%x", sha256.Sum256(data)),
				Status:       StatusPublished,
			})
		}
	}
	s.loaded = true
	return nil
}

func (s *FSStore) ListModules(_ context.Context, projectID string) ([]Module, error) {
	if err := s.load(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Module, 0, len(s.modules))
	for _, m := range s.modules {
		if m.ProjectID == projectID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *FSStore) GetModule(_ context.Context, projectID, name, version string) (Module, error) {
	if err := s.load(); err != nil {
		return Module{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.modules {
		if m.ProjectID == projectID && m.Name == name && m.Version == version {
			return m, nil
		}
	}
	return Module{}, fmt.Errorf("module %s/%s not found", name, version)
}

func (s *FSStore) GetModuleByID(_ context.Context, id string) (Module, error) {
	if err := s.load(); err != nil {
		return Module{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.modules {
		if m.ID == id {
			return m, nil
		}
	}
	return Module{}, fmt.Errorf("module %s not found", id)
}

func (s *FSStore) UpsertModule(_ context.Context, m Module) error {
	if err := s.load(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m.ProjectID = "" // the seed only fills the global catalog
	for i, existing := range s.modules {
		if existing.ProjectID == "" && existing.Name == m.Name && existing.Version == m.Version {
			s.modules[i] = m
			return nil
		}
	}
	s.modules = append(s.modules, m)
	return nil
}

func (s *FSStore) InsertModule(_ context.Context, m Module) error {
	if err := s.load(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.modules {
		if existing.ProjectID == m.ProjectID && existing.Name == m.Name && existing.Version == m.Version {
			return ErrModuleExists
		}
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("%s-%s", m.Name, m.Version)
		if m.ProjectID != "" {
			m.ID = m.ProjectID + "-" + m.ID
		}
	}
	m.Status = StatusPublished
	s.modules = append(s.modules, m)
	return nil
}

func (s *FSStore) SetModuleStatus(_ context.Context, projectID, name, version, status string) error {
	if err := s.load(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.modules {
		if m.ProjectID == projectID && m.Name == name && m.Version == version {
			s.modules[i].Status = status
			return nil
		}
	}
	return ErrModuleNotFound
}

func draftKey(projectID, moduleID, userOID string) string {
	return projectID + ":" + moduleID + ":" + userOID
}

func (s *FSStore) ListDrafts(_ context.Context, projectID, userOID string) ([]Draft, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Draft
	for _, d := range s.drafts {
		if d.ProjectID == projectID && d.UserOID == userOID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (s *FSStore) GetDraft(_ context.Context, projectID, moduleID, userOID string) (Draft, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.drafts[draftKey(projectID, moduleID, userOID)]
	if !ok {
		return Draft{}, fmt.Errorf("draft not found")
	}
	return d, nil
}

func (s *FSStore) SaveDraft(_ context.Context, d Draft) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := draftKey(d.ProjectID, d.ModuleID, d.UserOID)
	if d.ID == "" {
		d.ID = fmt.Sprintf("draft-%s", key)
	}
	s.drafts[key] = d
	return nil
}

func (s *FSStore) DeleteDraft(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, d := range s.drafts {
		if d.ID == id {
			delete(s.drafts, k)
			return nil
		}
	}
	return nil
}
