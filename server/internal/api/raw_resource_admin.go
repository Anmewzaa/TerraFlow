package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// RawResourceStore is what the raw resource endpoints need from the database.
type RawResourceStore interface {
	ListRawResources(ctx context.Context) ([]store.RawResource, error)
	CreateRawResource(ctx context.Context, r store.RawResource) error
	UpdateRawResource(ctx context.Context, r store.RawResource) error
	DeleteRawResource(ctx context.Context, name string) error
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// RawResourceAPI serves the admin-managed raw resource templates (D-041).
type RawResourceAPI struct{ Store RawResourceStore }

var rawNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

const maxRawBody = 64 << 10

type rawBody struct {
	Name          string                 `json:"name"`
	Title         string                 `json:"title"`
	ResourceType  string                 `json:"resource_type"`
	Description   string                 `json:"description"`
	DefaultValues map[string]interface{} `json:"default_values"`
	TargetFolder  string                 `json:"target_folder"`
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
	}
	return ""
}

func (b rawBody) toStore(by string) store.RawResource {
	if b.DefaultValues == nil {
		b.DefaultValues = map[string]interface{}{}
	}
	return store.RawResource{
		Name: b.Name, Title: strings.TrimSpace(b.Title), ResourceType: b.ResourceType,
		Description: b.Description, DefaultValues: b.DefaultValues, TargetFolder: b.TargetFolder, UpdatedBy: by,
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

// List: GET /catalog/resources. Every signed-in user reads the templates.
func (a *RawResourceAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := a.Store.ListRawResources(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not load raw resources")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// Create: POST /admin/resources.
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
		if err := a.Store.CreateRawResource(r.Context(), rec); errors.Is(err, store.ErrRawResourceExists) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not save the raw resource")
			return
		}
		_ = a.Store.LogAudit(r.Context(), actor(r), "rawresource.create", "rawresource:"+rec.Name, map[string]any{"resource_type": rec.ResourceType})
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
		if err := a.Store.UpdateRawResource(r.Context(), rec); errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "raw resource not found")
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not save the raw resource")
			return
		}
		_ = a.Store.LogAudit(r.Context(), actor(r), "rawresource.update", "rawresource:"+rec.Name, map[string]any{"resource_type": rec.ResourceType})
		writeJSON(w, http.StatusOK, rec)
	}
}

// Delete: DELETE /admin/resources/{name}.
func (a *RawResourceAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if err := a.Store.DeleteRawResource(r.Context(), name); errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "raw resource not found")
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not delete the raw resource")
			return
		}
		_ = a.Store.LogAudit(r.Context(), actor(r), "rawresource.delete", "rawresource:"+name, map[string]any{})
		w.WriteHeader(http.StatusNoContent)
	}
}
