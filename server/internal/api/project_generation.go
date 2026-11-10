package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// SettingsLoader gives the generator a project's settings (D-057). A nil loader, or a project
// without settings, means "as before".
type SettingsLoader func(ctx context.Context, projectID string) *hcl.ProjectSettings

func (f SettingsLoader) of(ctx context.Context, projectID string) *hcl.ProjectSettings {
	if f == nil {
		return nil
	}
	return f(ctx, projectID)
}

type settingsReader interface {
	GetProjectSettings(ctx context.Context, projectID string) (json.RawMessage, error)
	ListProviders(ctx context.Context) ([]store.Provider, error)
}

// SettingsFrom reads settings from the database. A document that cannot be read is logged and
// treated as no settings: the screen shows the problem when the owner opens the settings.
func SettingsFrom(s settingsReader, logger *slog.Logger) SettingsLoader {
	return func(ctx context.Context, projectID string) *hcl.ProjectSettings {
		raw, err := s.GetProjectSettings(ctx, projectID)
		if err != nil {
			return nil
		}
		st, err := hcl.ParseSettings(raw)
		if err != nil || len(st.Validate()) > 0 {
			if logger != nil {
				logger.Warn("project settings ignored", "project", projectID)
			}
			return nil
		}
		// A client cannot say where a provider comes from: the registry does.
		if len(st.Providers) > 0 {
			if list, err := s.ListProviders(ctx); err == nil {
				byID := map[string]store.Provider{}
				for _, p := range list {
					byID[p.ID] = p
				}
				for i := range st.Providers {
					if p, ok := byID[st.Providers[i].ID]; ok {
						st.Providers[i].Source, st.Providers[i].Block = p.Source, p.Block
					}
				}
			}
		}
		return st
	}
}

// SettingsStore is what the settings endpoints need besides the team's role lookup.
type SettingsStore interface {
	GetProjectSettings(ctx context.Context, projectID string) (json.RawMessage, error)
	SetProjectSettings(ctx context.Context, projectID string, raw json.RawMessage) error
	ListProviders(ctx context.Context) ([]store.Provider, error)
}

// SettingsAPI: a project's variables, default layout and starter files. Any member may read
// them (the canvas needs them); the owner and co-owners change them.
type SettingsAPI struct {
	Team  *TeamAPI
	Store SettingsStore
}

// Get: GET /projects/{id}/generation.
func (a *SettingsAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.Team.need(w, r, store.RoleContributor); !ok {
			return
		}
		raw, err := a.Store.GetProjectSettings(r.Context(), chi.URLParam(r, "id"))
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		st, err := hcl.ParseSettings(raw)
		if err != nil {
			st = &hcl.ProjectSettings{} // a document from a newer version: show the empty one, do not fail
		}
		writeJSON(w, http.StatusOK, st)
	}
}

// Put: PUT /projects/{id}/generation replaces the settings after validating every part.
func (a *SettingsAPI) Put() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		var body json.RawMessage
		if !decodeBody(w, r, &body) {
			return
		}
		st, err := hcl.ParseSettings(body)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		problems := st.Validate()
		if len(st.Providers) > 0 {
			known := map[string]bool{}
			if list, err := a.Store.ListProviders(r.Context()); err == nil {
				for _, p := range list {
					known[p.ID] = true
				}
			}
			for i, p := range st.Providers {
				if !known[p.ID] {
					problems = append(problems, fmt.Sprintf("providers[%d]: %q is not a registered provider", i, p.ID))
				}
			}
		}
		if len(problems) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": problems[0], "problems": problems})
			return
		}
		norm, _ := json.Marshal(st)
		err = a.Store.SetProjectSettings(r.Context(), id, norm)
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.Team.audit(r, "project.generation.update", id, map[string]any{
			"layout": st.Layout.Mode, "variables": len(st.Variables), "starters": len(st.Starters),
		})
		writeJSON(w, http.StatusOK, st)
	}
}
