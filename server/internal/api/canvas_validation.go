package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/validate"
)

// MaxCanvasBytes is the largest canvas body the API reads (D-065).
const MaxCanvasBytes = 2 << 20

// moduleSource is what the canvas checks need from the catalog: the modules a project can
// use (its own and the global ones, D-060). *catalog.Service implements it.
type moduleSource interface {
	VisibleModules(ctx context.Context, projectID string) ([]catalog.Module, map[string]bool, error)
}

// summarySource lists a project's modules with their versions. *catalog.Service implements it.
type summarySource interface {
	ListModuleSummaries(ctx context.Context, projectID string) ([]catalog.ModuleSummary, error)
}

func catalogFor(ctx context.Context, src moduleSource, projectID string) (validate.Catalog, error) {
	if src == nil {
		return nil, nil
	}
	mods, _, err := src.VisibleModules(ctx, projectID)
	if err != nil {
		return nil, err
	}
	list := make([]validate.Module, 0, len(mods))
	for _, m := range mods {
		list = append(list, validate.Module{Name: m.Name, Version: m.Version, Schema: m.SchemaJSON})
	}
	return validate.NewMapCatalog(list), nil
}

type findingsBody struct {
	Error    string             `json:"error,omitempty"`
	Findings []validate.Finding `json:"findings"`
}

func writeFindings(w http.ResponseWriter, status int, msg string, fs []validate.Finding) {
	if fs == nil {
		fs = []validate.Finding{}
	}
	writeJSON(w, status, findingsBody{Error: msg, Findings: fs})
}

// readCanvasBody reads a canvas request body, refusing one that is too big, not JSON, or
// not shaped like a canvas. It writes the answer itself and reports false when it did.
func readCanvasBody(w http.ResponseWriter, r *http.Request) ([]byte, store.CanvasState, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxCanvasBytes))
	if err != nil {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "the canvas is too large")
		return nil, store.CanvasState{}, false
	}
	if !json.Valid(body) {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return nil, store.CanvasState{}, false
	}
	c, err := validate.Parse(body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return nil, store.CanvasState{}, false
	}
	return body, c, true
}

// refuseUnsafeCanvas runs the structural rules a draft save must pass: input that must never
// be stored. Unfinished work passes (a draft may be incomplete); only malformed, oversized
// or unsafe input is refused.
func refuseUnsafeCanvas(w http.ResponseWriter, c store.CanvasState) bool {
	fs := validate.Structural(c)
	if !validate.HasStructural(fs) {
		return false
	}
	writeFindings(w, http.StatusUnprocessableEntity, "the canvas has input that cannot be saved", fs)
	return true
}

// ValidateCanvas: POST /projects/{id}/validate takes a canvas and answers the findings. It
// changes nothing; the screens use it for instant feedback, and it is the very check that
// publish and approve run.
func ValidateCanvas(src moduleSource, settings ...SettingsLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, c, ok := readCanvasBody(w, r)
		if !ok {
			return
		}
		cat, err := catalogFor(r.Context(), src, chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not read the catalog")
			return
		}
		writeFindings(w, http.StatusOK, "", validate.CanvasWithPolicy(c, cat, firstSettings(settings).of(r.Context(), chi.URLParam(r, "id")), r.URL.Query().Get("env")))
	}
}

// ValidChangeset is a middleware for the routes that move a changeset forward (publish and
// approve): it runs every rule on the stored canvas and answers 422 with the findings when
// there is an error, so a client that skips the screens cannot skip the rules.
func ValidChangeset(get func(ctx context.Context, id string) (store.Changeset, error), src moduleSource, settings ...SettingsLoader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cs, err := get(r.Context(), chi.URLParam(r, "csId"))
			if err != nil {
				next.ServeHTTP(w, r) // the handler answers not found in its own words
				return
			}
			c, err := validate.Parse(cs.CanvasJSON)
			if err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			cat, err := catalogFor(r.Context(), src, cs.ProjectID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "could not read the catalog")
				return
			}
			fs := validate.CanvasWithPolicy(c, cat, firstSettings(settings).of(r.Context(), cs.ProjectID), cs.Env)
			if validate.HasErrors(fs) {
				writeFindings(w, http.StatusUnprocessableEntity, "fix these problems first", fs)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// canvasCatalog is the catalog the checks use; nil when there is no catalog service, in which
// case module rules are skipped (the structural ones still run).
func canvasCatalog(cfg RouterConfig) moduleSource {
	if cfg.CatalogService == nil {
		return nil
	}
	return cfg.CatalogService
}

// summaries is the same catalog as a module list with versions; nil without a catalog.
func summaries(cfg RouterConfig) summarySource {
	if cfg.CatalogService == nil {
		return nil
	}
	return cfg.CatalogService
}

// ChangesetFindings: GET /projects/{id}/changesets/{csId}/findings answers what publish and approve
// would say about the stored canvas, without changing anything: the errors that would stop it and
// the project's policy warnings. The send-for-review dialog lists them.
func ChangesetFindings(get func(ctx context.Context, id string) (store.Changeset, error), src moduleSource, settings ...SettingsLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := get(r.Context(), chi.URLParam(r, "csId"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "changeset not found")
			return
		}
		c, err := validate.Parse(cs.CanvasJSON)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		cat, err := catalogFor(r.Context(), src, cs.ProjectID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not read the catalog")
			return
		}
		writeFindings(w, http.StatusOK, "", validate.CanvasWithPolicy(c, cat, firstSettings(settings).of(r.Context(), cs.ProjectID), cs.Env))
	}
}
