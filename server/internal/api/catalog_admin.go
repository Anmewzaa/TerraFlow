package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

const maxSchemaBytes = 1 << 20

// CatalogAdmin is module management: list, validate, import and retire module schemas.
// It serves the global catalog under /admin/catalog (admin role) and, with Project set, one
// project's own catalog under /projects/{id}/catalog (owner and co-owner; D-060).
type CatalogAdmin struct {
	Svc   *catalog.Service
	Audit interface {
		LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
	}
	// Project makes the routes act on the catalog of the project in the URL.
	Project bool
}

// scope is the project whose catalog the request is about ("" for the global one).
func (c *CatalogAdmin) scope(r *http.Request) string {
	if c.Project {
		return chi.URLParam(r, "id")
	}
	return ""
}

func (c *CatalogAdmin) audit(r *http.Request, action, resource string, detail map[string]any) {
	if id := c.scope(r); id != "" {
		detail["project_id"] = id // the project's audit trail finds entries by this
	}
	_ = c.Audit.LogAudit(r.Context(), actor(r), action, resource, detail)
}

type moduleRow struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	TargetFolder string    `json:"target_folder"`
	ImportedBy   string    `json:"imported_by"`
	Checksum     string    `json:"checksum"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func statusOf(m catalog.Module) string {
	if m.Status == "" {
		return catalog.StatusPublished
	}
	return m.Status
}

// List: GET /admin/catalog/modules. Every version, including deprecated ones.
func (c *CatalogAdmin) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mods, err := c.Svc.ListAllModules(r.Context(), c.scope(r))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to list modules")
			return
		}
		rows := make([]moduleRow, 0, len(mods))
		for _, m := range mods {
			rows = append(rows, moduleRow{
				Name: m.Name, Version: m.Version, Title: m.Title, Status: statusOf(m),
				TargetFolder: m.TargetFolder, ImportedBy: m.ImportedBy, Checksum: m.Checksum, UpdatedAt: m.UpdatedAt,
			})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Name != rows[j].Name {
				return rows[i].Name < rows[j].Name
			}
			return versionLess(rows[j].Version, rows[i].Version) // newest first
		})
		writeJSON(w, http.StatusOK, rows)
	}
}

type importBody struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

// checked is the outcome of validating a schema against the catalog.
type checked struct {
	Report catalog.Report `json:"report"`
	// Exists is "none", "identical" (same bytes already published) or
	// "different" (that version is published and cannot be replaced).
	Exists string `json:"exists"`
	// ComparedTo is the version the changes are measured against ("" if none).
	ComparedTo string           `json:"compared_to"`
	Changes    []catalog.Change `json:"changes"`
	Breaking   int              `json:"breaking_changes"`
}

func (c *CatalogAdmin) check(ctx context.Context, projectID string, body importBody) (checked, error) {
	out := checked{Report: catalog.Validate(body.Name, body.Schema), Exists: "none", Changes: []catalog.Change{}}
	mods, err := c.Svc.ListAllModules(ctx, projectID)
	if err != nil {
		return out, err
	}
	if projectID != "" && catalog.ValidateName(body.Name) {
		// A project module may not take a name of the global catalog: it would change what the
		// nodes already on the project's canvases mean. A name the project already owns stays
		// its own, even if the global catalog gained that name later.
		owned := false
		for _, m := range mods {
			owned = owned || m.Name == body.Name
		}
		if !owned {
			taken, err := c.Svc.GlobalHasName(ctx, body.Name)
			if err != nil {
				return out, err
			}
			if taken {
				out.Report.Valid = false
				out.Report.Errors = append(out.Report.Errors, catalog.Issue{Path: "/name", Code: "global_name",
					Message: "the global catalog already has a module named " + body.Name + "; give the project's module another name"})
			}
		}
	}
	if !out.Report.Valid {
		return out, nil
	}
	var prev *catalog.Module
	for i, m := range mods {
		if m.Name != body.Name {
			continue
		}
		if m.Version == out.Report.Version {
			out.Exists = "different"
			if m.Checksum == out.Report.Checksum {
				out.Exists = "identical"
			}
			continue
		}
		if prev == nil || versionLess(prev.Version, m.Version) {
			prev = &mods[i]
		}
	}
	if prev != nil {
		out.ComparedTo = prev.Version
		out.Changes = catalog.Compare(prev.SchemaJSON, body.Schema)
		for _, ch := range out.Changes {
			if ch.Breaking {
				out.Breaking++
			}
		}
	}
	return out, nil
}

func readImport(w http.ResponseWriter, r *http.Request) (importBody, bool) {
	var b importBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSchemaBytes)).Decode(&b); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body (expected {\"name\": ..., \"schema\": {...}})")
		return b, false
	}
	b.Name = strings.TrimSpace(b.Name)
	if len(b.Schema) == 0 || string(b.Schema) == "null" {
		writeJSONError(w, http.StatusBadRequest, "schema is required")
		return b, false
	}
	return b, true
}

// Validate: POST /admin/catalog/validate. Always 200 with a report, so the UI
// can show every problem and what an import would change before saving.
func (c *CatalogAdmin) Validate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := readImport(w, r)
		if !ok {
			return
		}
		res, err := c.check(r.Context(), c.scope(r), body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to read the catalog")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

// Import: POST /admin/catalog/modules. Validates, then adds the version as
// published. A published version is never replaced.
func (c *CatalogAdmin) Import() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := readImport(w, r)
		if !ok {
			return
		}
		res, err := c.check(r.Context(), c.scope(r), body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to read the catalog")
			return
		}
		if !res.Report.Valid {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the schema has errors; nothing was imported", "result": res})
			return
		}
		switch res.Exists {
		case "identical":
			writeJSON(w, http.StatusOK, map[string]any{"status": "unchanged", "result": res})
			return
		case "different":
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":  "version " + res.Report.Version + " of " + body.Name + " is already published and cannot be changed; give the schema a new version",
				"result": res,
			})
			return
		}
		m := catalog.Module{
			ProjectID: c.scope(r), Name: body.Name, Version: res.Report.Version, Title: res.Report.Title, Description: res.Report.Description,
			Icon: res.Report.Icon, TargetFolder: res.Report.TargetFolder, SchemaJSON: body.Schema,
			Checksum: res.Report.Checksum, Status: catalog.StatusPublished, ImportedBy: actor(r),
		}
		err = c.Svc.Store().InsertModule(r.Context(), m)
		if errors.Is(err, catalog.ErrModuleExists) { // lost a race with another admin
			writeJSONError(w, http.StatusConflict, "this version was just imported by someone else")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to import the module")
			return
		}
		c.audit(r, "catalog.import", "module:"+m.Name+"/"+m.Version, map[string]any{
			"checksum": m.Checksum, "warnings": len(res.Report.Warnings), "breaking_changes": res.Breaking, "compared_to": res.ComparedTo,
		})
		writeJSON(w, http.StatusCreated, map[string]any{"status": "imported", "result": res})
	}
}

type draftBody struct {
	Name      string `json:"name"`
	Variables string `json:"variables"`
}

// SchemaDraft: POST .../catalog/draft. Turns the pasted text of a variables.tf into a first
// draft of a schema for the editor. It reads nothing but the text and stores nothing.
func SchemaDraft() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b draftBody
		if !decodeBodyLimit(w, r, &b, maxSchemaBytes) {
			return
		}
		b.Name = strings.TrimSpace(b.Name)
		if !catalog.ValidateName(b.Name) {
			writeJSONError(w, http.StatusBadRequest, "name must be 2-63 characters: lowercase letters, digits and '-', starting with a letter")
			return
		}
		schema, notes, err := catalog.DraftFromVariables(b.Name, []byte(b.Variables))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if notes == nil {
			notes = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"schema": schema, "notes": notes})
	}
}

type statusBody struct {
	Status string `json:"status"`
}

// SetStatus: PATCH /admin/catalog/modules/{name}/{version}.
func (c *CatalogAdmin) SetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
		var b statusBody
		if !decodeBody(w, r, &b) {
			return
		}
		if b.Status != catalog.StatusPublished && b.Status != catalog.StatusDeprecated {
			writeJSONError(w, http.StatusBadRequest, `status must be "published" or "deprecated"`)
			return
		}
		err := c.Svc.Store().SetModuleStatus(r.Context(), c.scope(r), name, version, b.Status)
		if errors.Is(err, catalog.ErrModuleNotFound) {
			writeJSONError(w, http.StatusNotFound, "module version not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to update the module")
			return
		}
		c.audit(r, "catalog."+b.Status, "module:"+name+"/"+version, map[string]any{})
		writeJSON(w, http.StatusOK, map[string]string{"name": name, "version": version, "status": b.Status})
	}
}

var versionPart = regexp.MustCompile(`\d+`)

// versionLess orders versions such as v1, v1.2 and v1.10.0 numerically.
func versionLess(a, b string) bool {
	pa, pb := versionPart.FindAllString(a, -1), versionPart.FindAllString(b, -1)
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}

// --- component config (colours kept in the database) -------------------------

// ConfigStore is the persistence the component-config endpoints need.
type ConfigStore interface {
	ListComponentConfig(ctx context.Context, projectID string) ([]store.ComponentConfig, error)
	SetComponentConfig(ctx context.Context, projectID, address, color, by string) error
}

var addressRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}/[A-Za-z_][A-Za-z0-9_-]{0,127}$`)

// ComponentConfigList: GET /projects/{id}/component-config.
func ComponentConfigList(s ConfigStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := s.ListComponentConfig(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

type configBody struct {
	Address string `json:"address"`
	Color   string `json:"color"`
}

// ComponentConfigSet: PUT /projects/{id}/component-config. An empty colour
// removes the setting.
func ComponentConfigSet(s ConfigStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b configBody
		if !decodeBody(w, r, &b) {
			return
		}
		if !addressRe.MatchString(b.Address) {
			writeJSONError(w, http.StatusBadRequest, `address must look like "<folder>/<module label>"`)
			return
		}
		if b.Color != "" && !catalog.ValidColorKey(b.Color) {
			writeJSONError(w, http.StatusBadRequest, "color must be a short lowercase key such as cyan or blue")
			return
		}
		if err := s.SetComponentConfig(r.Context(), chi.URLParam(r, "id"), b.Address, b.Color, actor(r)); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		writeJSON(w, http.StatusOK, b)
	}
}
