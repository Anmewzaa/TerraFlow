package catalog

import (
	"context"
	"sort"
)

type Service struct {
	store Store
}

func NewService(s Store) *Service {
	return &Service{store: s}
}

func (s *Service) Store() Store {
	return s.store
}

func (s *Service) ListModuleSummaries(ctx context.Context) ([]ModuleSummary, error) {
	modules, err := s.store.ListModules(ctx)
	if err != nil {
		return nil, err
	}

	byName := make(map[string]*ModuleSummary)
	var order []string

	for _, m := range modules {
		sum, ok := byName[m.Name]
		if !ok {
			sum = &ModuleSummary{
				Name:        m.Name,
				Title:       m.Title,
				Description: m.Description,
				Icon:        m.Icon,
			}
			byName[m.Name] = sum
			order = append(order, m.Name)
		}
		sum.Versions = append(sum.Versions, m.Version)
	}

	result := make([]ModuleSummary, 0, len(order))
	for _, name := range order {
		sum := byName[name]
		sort.Strings(sum.Versions)
		sum.LatestVersion = sum.Versions[len(sum.Versions)-1]
		result = append(result, *sum)
	}
	return result, nil
}

func (s *Service) GetModule(ctx context.Context, name, version string) (Module, error) {
	return s.store.GetModule(ctx, name, version)
}

func (s *Service) ListDrafts(ctx context.Context, projectID, userOID string) ([]Draft, error) {
	return s.store.ListDrafts(ctx, projectID, userOID)
}

func (s *Service) GetDraft(ctx context.Context, projectID, moduleID, userOID string) (Draft, error) {
	return s.store.GetDraft(ctx, projectID, moduleID, userOID)
}

func (s *Service) SaveDraft(ctx context.Context, d Draft) error {
	return s.store.SaveDraft(ctx, d)
}

func (s *Service) DeleteDraft(ctx context.Context, id string) error {
	return s.store.DeleteDraft(ctx, id)
}
