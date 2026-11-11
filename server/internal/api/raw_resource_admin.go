package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// RawResourceStore is what the raw resource endpoints need from the database.
type RawResourceStore interface {
	ListRawResourcesIn(ctx context.Context, projectID string) ([]store.RawResource, error)
	CreateRawResource(ctx context.Context, r store.RawResource) error
	UpdateRawResource(ctx context.Context, r store.RawResource) error
	DeleteRawResourceIn(ctx context.Context, projectID, name string) error
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// RawResourceAPI serves the raw resource templates (D-041): the global ones an admin manages
// and, with Project set, the ones of the project in the URL (owner and co-owner; D-060).
type RawResourceAPI struct {
	Store   RawResourceStore
	Project bool
}

// scope is the project whose templates the request is about ("" for the global ones).
func (a *RawResourceAPI) scope(r *http.Request) string {
	if a.Project {
		return chi.URLParam(r, "id")
	}
	return ""
}

func (a *RawResourceAPI) audit(r *http.Request, action, name string, detail map[string]any) {
	if id := a.scope(r); id != "" {
		detail["project_id"] = id // the project's audit trail finds entries by this
	}
	_ = a.Store.LogAudit(r.Context(), actor(r), action, "rawresource:"+name, detail)
}

// rawView is a template as a project sees it.
type rawView struct {
	store.RawResource
	// Scope is "project" for a template of the project's own catalog, else "global".
	Scope string `json:"scope"`
	// ShadowsGlobal is true for a project template whose name a global one also has.
	ShadowsGlobal bool `json:"shadows_global,omitempty"`
}

// visibleRaw merges the global templates with a project's own: a name the project has comes
// from the project, and the result says so (the same rule as catalog.Service.VisibleModules).
func visibleRaw(global, own []store.RawResource) []rawView {
	mine := make(map[string]bool, len(own))
	for _, t := range own {
		mine[t.Name] = true
	}
	shadowed := map[string]bool{}
	out := make([]rawView, 0, len(global)+len(own))
	for _, t := range global {
		if mine[t.Name] {
			shadowed[t.Name] = true
			continue
		}
		out = append(out, rawView{RawResource: t, Scope: catalog.ScopeGlobal})
	}
	for _, t := range own {
		out = append(out, rawView{RawResource: t, Scope: catalog.ScopeProject, ShadowsGlobal: shadowed[t.Name]})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := strings.ToLower(out[i].Title), strings.ToLower(out[j].Title); a != b {
			return a < b
		}
		return out[i].Name < out[j].Name
	})
	return out
}

var rawNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

const maxRawBody = 64 << 10

type rawBody struct {
	Name          string                 `json:"name"`
	Title         string                 `json:"title"`
	ResourceType  string                 `json:"resource_type"`
	Description   string                 `json:"description"`
	DefaultValues map[string]interface{} `json:"default_values"`
	TargetFolder  string                 `json:"target_folder"`
	// Provider defaults to the one in the resource type; Category groups it in the palette;
	// MinVersion and MaxVersion (major.minor) say which provider versions it exists in.
	Provider   string `json:"provider"`
	Category   string `json:"category"`
	MinVersion string `json:"min_version"`
	MaxVersion string `json:"max_version"`
}

func (b rawBody) check() string {
	switch {
	case !rawNamePattern.MatchString(b.Name):
		return "name must be 2-63 characters: lowercase letters, digits and '-'"
	case strings.TrimSpace(b.Title) == "" || len(b.Title) > 80:
		return "title is required (at most 80 characters)"
	case !hcl.ValidResourceType(b.ResourceType):
		return "resource_type must look like azurerm_resource_group"
	case len(b.Description) > 500:
		return "description is at most 500 characters"
	case strings.ContainsAny(b.TargetFolder, `\:`) || strings.Contains(b.TargetFolder, "..") || strings.HasPrefix(b.TargetFolder, "/"):
		return "target_folder is not a valid folder"
	case b.Provider != "" && b.Provider != hcl.ProviderOfType(b.ResourceType):
		return "the resource type " + b.ResourceType + " does not belong to provider " + b.Provider
	case !ValidCategory(b.Category):
		return "category is not one of the known categories"
	case !hcl.ValidBound(b.MinVersion) || !hcl.ValidBound(b.MaxVersion):
		return "min_version and max_version must look like 3.0 (major.minor) or be empty"
	case b.MinVersion != "" && b.MaxVersion != "" && !hcl.Compatible(b.MinVersion, b.MinVersion, b.MaxVersion):
		return "min_version must not be above max_version"
	}
	return ""
}

// provider is the stated provider, or the one the resource type belongs to.
func (b rawBody) provider() string {
	if b.Provider != "" {
		return b.Provider
	}
	return hcl.ProviderOfType(b.ResourceType)
}

func (b rawBody) toStore(by string) store.RawResource {
	if b.DefaultValues == nil {
		b.DefaultValues = map[string]interface{}{}
	}
	return store.RawResource{
		Name: b.Name, Title: strings.TrimSpace(b.Title), ResourceType: b.ResourceType,
		Description: b.Description, DefaultValues: b.DefaultValues, TargetFolder: b.TargetFolder,
		Provider: b.provider(), Category: b.Category, MinVersion: b.MinVersion, MaxVersion: b.MaxVersion, UpdatedBy: by,
	}
}

func decodeRaw(w http.ResponseWriter, r *http.Request) (rawBody, bool) {
	var b rawBody
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRawBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		writeJSONError(w, http.StatusBadRequest, "the body is not valid JSON, has unknown fields, or is too large")
		return b, false
	}
	return b, true
}

// List: GET /catalog/resources (the global templates, for every signed-in user) and
// GET /projects/{id}/catalog/resources (what the project sees: global plus its own).
func (a *RawResourceAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		global, err := a.Store.ListRawResourcesIn(r.Context(), "")
		var own []store.RawResource
		if id := a.scope(r); err == nil && id != "" {
			own, err = a.Store.ListRawResourcesIn(r.Context(), id)
		}
		if isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not load raw resources")
			return
		}
		writeJSON(w, http.StatusOK, visibleRaw(global, own))
	}
}

// globalHas reports whether a global template has the name.
func (a *RawResourceAPI) globalHas(ctx context.Context, name string) (bool, error) {
	global, err := a.Store.ListRawResourcesIn(ctx, "")
	for _, t := range global {
		if t.Name == name {
			return true, err
		}
	}
	return false, err
}

// Create: POST /admin/resources and POST /projects/{id}/catalog/resources. A project template
// may not take the name of a global one.
func (a *RawResourceAPI) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := decodeRaw(w, r)
		if !ok {
			return
		}
		if msg := b.check(); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		rec := b.toStore(actor(r))
		rec.ProjectID = a.scope(r)
		if rec.ProjectID != "" {
			taken, err := a.globalHas(r.Context(), rec.Name)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "could not load raw resources")
				return
			}
			if taken {
				writeJSONError(w, http.StatusConflict, "the global catalog already has a raw resource named "+rec.Name+"; give the project's one another name")
				return
			}
		}
		if err := a.Store.CreateRawResource(r.Context(), rec); errors.Is(err, store.ErrRawResourceExists) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not save the raw resource")
			return
		}
		a.audit(r, "rawresource.create", rec.Name, map[string]any{"resource_type": rec.ResourceType})
		writeJSON(w, http.StatusCreated, rec)
	}
}

// Update: PUT /admin/resources/{name}. The name never changes.
func (a *RawResourceAPI) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := decodeRaw(w, r)
		if !ok {
			return
		}
		b.Name = chi.URLParam(r, "name")
		if msg := b.check(); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		rec := b.toStore(actor(r))
		rec.ProjectID = a.scope(r)
		if err := a.Store.UpdateRawResource(r.Context(), rec); errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "raw resource not found")
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not save the raw resource")
			return
		}
		a.audit(r, "rawresource.update", rec.Name, map[string]any{"resource_type": rec.ResourceType})
		writeJSON(w, http.StatusOK, rec)
	}
}

// Delete: DELETE /admin/resources/{name}.
func (a *RawResourceAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if err := a.Store.DeleteRawResourceIn(r.Context(), a.scope(r), name); errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "raw resource not found")
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not delete the raw resource")
			return
		}
		a.audit(r, "rawresource.delete", name, map[string]any{})
		w.WriteHeader(http.StatusNoContent)
	}
}
