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

// VisibleModules is what a project sees: the global catalog plus the project's own (D-060).
// A name belongs to one scope inside a project: when the project has a module of some name,
// every version of that name comes from the project and the global ones are left out. The
// second result holds the names where that happened, so nothing is shadowed silently.
// projectID "" gives the global catalog alone.
func (s *Service) VisibleModules(ctx context.Context, projectID string) ([]Module, map[string]bool, error) {
	global, err := s.store.ListModules(ctx, "")
	if err != nil || projectID == "" {
		return global, nil, err
	}
	own, err := s.store.ListModules(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	if len(own) == 0 {
		return global, nil, nil
	}
	mine := make(map[string]bool, len(own))
	for _, m := range own {
		mine[m.Name] = true
	}
	shadowed := map[string]bool{}
	out := make([]Module, 0, len(global)+len(own))
	for _, m := range global {
		if mine[m.Name] {
			shadowed[m.Name] = true
			continue
		}
		out = append(out, m)
	}
	out = append(out, own...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Version < out[j].Version
	})
	return out, shadowed, nil
}

// GlobalHasName reports whether the global catalog has any version of a module name.
func (s *Service) GlobalHasName(ctx context.Context, name string) (bool, error) {
	global, err := s.store.ListModules(ctx, "")
	if err != nil {
		return false, err
	}
	for _, m := range global {
		if m.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// ListModuleSummaries is the palette of a project ("" for the global catalog alone).
func (s *Service) ListModuleSummaries(ctx context.Context, projectID string) ([]ModuleSummary, error) {
	modules, shadowed, err := s.VisibleModules(ctx, projectID)
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
				Scope:       ScopeOf(m.ProjectID),
			}
			sum.ShadowsGlobal = shadowed[m.Name]
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

// ListAllModules returns every version of one scope with its status, for the screens that manage it.
func (s *Service) ListAllModules(ctx context.Context, projectID string) ([]Module, error) {
	return s.store.ListModules(ctx, projectID)
}

// GetModule reads a module as a project sees it ("" for the global catalog alone): the project's
// own when it has that name, else the global one. The rule is the one of VisibleModules.
func (s *Service) GetModule(ctx context.Context, projectID, name, version string) (Module, error) {
	if projectID == "" {
		return s.store.GetModule(ctx, "", name, version)
	}
	own, err := s.store.ListModules(ctx, projectID)
	if err != nil {
		return Module{}, err
	}
	named := false
	for _, m := range own {
		if m.Name != name {
			continue
		}
		named = true
		if m.Version == version {
			return m, nil
		}
	}
	if named {
		return Module{}, ErrModuleNotFound
	}
	return s.store.GetModule(ctx, "", name, version)
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
