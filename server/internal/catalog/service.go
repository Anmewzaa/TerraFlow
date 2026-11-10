package catalog

import (
	"context"
	"encoding/json"
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
		if m.Status == StatusDeprecated {
			continue // retired versions are not offered for new work
		}
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
		var hints struct {
			Canvas   json.RawMessage `json:"x-tf-canvas"`
			Required []string        `json:"required"`
			Module   struct {
				Provider string `json:"provider"`
			} `json:"x-tf-module"`
		}
		if json.Unmarshal(m.SchemaJSON, &hints) == nil {
			if len(hints.Canvas) > 0 {
				sum.Canvas = hints.Canvas
				var c struct {
					Category string `json:"category"`
					Domain   string `json:"domain"`
				}
				if json.Unmarshal(hints.Canvas, &c) == nil {
					sum.Category = c.Category
					if sum.Category == "" {
						sum.Category = CategoryOfDomain(c.Domain)
					}
				}
			}
			sum.Required = hints.Required
			sum.Provider = hints.Module.Provider
		}
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

// CategoryOfDomain maps the canvas colour domain of a module to a palette category, so modules
// written before categories existed are grouped without a change to their schema.
func CategoryOfDomain(domain string) string {
	switch domain {
	case "network":
		return "network"
	case "data":
		return "storage"
	case "compute":
		return "compute"
	case "security", "identity":
		return "security"
	case "monitoring":
		return "monitoring"
	}
	return "other"
}

// ListAllModules returns every version with its status, for the Admin area.
func (s *Service) ListAllModules(ctx context.Context) ([]Module, error) {
	return s.store.ListModules(ctx)
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
