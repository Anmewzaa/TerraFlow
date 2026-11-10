package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// AdminAPI holds the project, member and user endpoints. Writes are admin-only;
// the router enforces that with RequireAdmin.
type AdminAPI struct {
	Store        AccessStore
	WorkspaceDir string
	SampleDir    string
}

const maxProjectName = 80
const maxDescription = 500

func (a *AdminAPI) audit(r *http.Request, action, projectID string, extra map[string]any) {
	detail := map[string]any{"project_id": projectID}
	for k, v := range extra {
		detail[k] = v
	}
	_ = a.Store.LogAudit(r.Context(), actor(r), action, "project:"+projectID, detail)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// resolveIACPath validates an explicit project folder: it must be an existing
// directory inside the workspace or beside the sample projects.
func (a *AdminAPI) resolveIACPath(p string) (string, string) {
	wsAbs, err := filepath.Abs(a.WorkspaceDir)
	if err != nil {
		return "", "workspace dir unavailable"
	}
	samplesAbs, _ := filepath.Abs(filepath.Dir(a.SampleDir))
	dir, err := filepath.Abs(p)
	if err != nil || !(within(wsAbs, dir) || within(samplesAbs, dir)) {
		return "", "iac_path must be inside the workspace directory"
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", "iac_path is not an existing directory"
	}
	return dir, ""
}

func validName(name string) bool {
	return name != "" && slugify(name) != "" && len(name) <= maxProjectName &&
		!strings.ContainsFunc(name, unicode.IsControl)
}

// List: GET /projects. Admins see every project; everyone else only the
// projects they are assigned to.
func (a *AdminAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		var (
			projects []store.Project
			err      error
		)
		if isDevOps(u) {
			projects, err = a.Store.ListProjects(r.Context())
		} else if u != nil {
			projects, err = a.Store.ListProjectsForUser(r.Context(), u.OID)
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if projects == nil {
			projects = []store.Project{}
		}
		writeJSON(w, http.StatusOK, projects)
	}
}

type createProjectBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IACPath     string `json:"iac_path"`
}

// Create: POST /projects (admin). With no iac_path it creates <workspace>/<slug>.
func (a *AdminAPI) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createProjectBody
		if !decodeBody(w, r, &req) {
			return
		}
		name := strings.TrimSpace(req.Name)
		desc := strings.TrimSpace(req.Description)
		if !validName(name) {
			writeJSONError(w, http.StatusBadRequest, "name is required (letters/digits, max 80 chars)")
			return
		}
		if len(desc) > maxDescription {
			writeJSONError(w, http.StatusBadRequest, "description is too long (max 500 chars)")
			return
		}
		slug := slugify(name)

		var dir string
		if p := strings.TrimSpace(req.IACPath); p != "" {
			var msg string
			if dir, msg = a.resolveIACPath(p); msg != "" {
				writeJSONError(w, http.StatusBadRequest, msg)
				return
			}
		} else {
			wsAbs, err := filepath.Abs(a.WorkspaceDir)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "workspace dir unavailable")
				return
			}
			dir = filepath.Join(wsAbs, slug)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "cannot create project directory")
				return
			}
		}

		p, err := a.Store.CreateProject(r.Context(), store.Project{
			Name: name, Slug: slug, Description: desc, IACPath: dir, SourceType: "local",
		})
		if errors.Is(err, store.ErrSlugTaken) {
			writeJSONError(w, http.StatusConflict, "a project with this name already exists")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "project.create", p.ID, map[string]any{"name": p.Name, "iac_path": p.IACPath})
		writeJSON(w, http.StatusCreated, p)
	}
}

type updateProjectBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IACPath     *string `json:"iac_path"`
}

// Update: PATCH /projects/{id} (admin). The slug never changes, so links and
// branch names stay stable when a project is renamed.
func (a *AdminAPI) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req updateProjectBody
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Name == nil && req.Description == nil && req.IACPath == nil {
			writeJSONError(w, http.StatusBadRequest, "nothing to update")
			return
		}
		patch := store.ProjectPatch{}
		if req.Name != nil {
			n := strings.TrimSpace(*req.Name)
			if !validName(n) {
				writeJSONError(w, http.StatusBadRequest, "name is required (letters/digits, max 80 chars)")
				return
			}
			patch.Name = &n
		}
		if req.Description != nil {
			d := strings.TrimSpace(*req.Description)
			if len(d) > maxDescription {
				writeJSONError(w, http.StatusBadRequest, "description is too long (max 500 chars)")
				return
			}
			patch.Description = &d
		}
		if req.IACPath != nil {
			dir, msg := a.resolveIACPath(strings.TrimSpace(*req.IACPath))
			if msg != "" {
				writeJSONError(w, http.StatusBadRequest, msg)
				return
			}
			patch.IACPath = &dir
		}
		p, err := a.Store.UpdateProject(r.Context(), id, patch)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "project.update", id, map[string]any{"name": p.Name, "iac_path": p.IACPath})
		writeJSON(w, http.StatusOK, p)
	}
}

// Delete: DELETE /projects/{id} (admin). Removes the project, its drafts and its
// changesets from the app; the IaC files and the audit log are kept. Changesets
// still waiting for review make this a 409 unless ?force=true.
func (a *AdminAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if _, err := a.Store.GetProject(r.Context(), id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSONError(w, http.StatusNotFound, "project not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		pending, err := a.Store.CountPendingChangesets(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if pending > 0 && r.URL.Query().Get("force") != "true" {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":              "project has changesets waiting for review; delete anyway with force=true",
				"pending_changesets": pending,
			})
			return
		}
		err = a.Store.DeleteProject(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "project.delete", id, map[string]any{"pending_changesets": pending})
		w.WriteHeader(http.StatusNoContent)
	}
}

// Members: GET /projects/{id}/members (admin).
func (a *AdminAPI) Members() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if _, err := a.Store.GetProject(r.Context(), id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSONError(w, http.StatusNotFound, "project not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		members, err := a.Store.ListMembers(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if members == nil {
			members = []store.Member{}
		}
		writeJSON(w, http.StatusOK, members)
	}
}

type setMembersBody struct {
	UserOIDs []string `json:"user_oids"`
}

// SetMembers: PUT /projects/{id}/members (admin). Replaces the whole list.
func (a *AdminAPI) SetMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req setMembersBody
		if !decodeBody(w, r, &req) {
			return
		}
		if req.UserOIDs == nil {
			writeJSONError(w, http.StatusBadRequest, "user_oids is required (use [] to remove everyone)")
			return
		}
		seen := map[string]bool{}
		oids := make([]string, 0, len(req.UserOIDs))
		for _, oid := range req.UserOIDs {
			oid = strings.TrimSpace(oid)
			if oid == "" {
				writeJSONError(w, http.StatusBadRequest, "user_oids must not contain empty values")
				return
			}
			if !seen[oid] {
				seen[oid] = true
				oids = append(oids, oid)
			}
		}
		// Someone who is not active (waiting, rejected, suspended) cannot be assigned.
		if len(oids) > 0 {
			users, uerr := a.Store.ListUsers(r.Context())
			if uerr != nil {
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			for _, u := range users {
				if seen[u.OID] && u.Status != "" && u.Status != store.UserActive {
					writeJSONError(w, http.StatusUnprocessableEntity, "user "+u.OID+" is not active, so they cannot be assigned")
					return
				}
			}
		}
		err := a.Store.SetMembers(r.Context(), id, oids, actor(r))
		var unknown store.UnknownUserError
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		case errors.As(err, &unknown):
			writeJSONError(w, http.StatusUnprocessableEntity, "unknown user "+unknown.OID+": they must sign in once before they can be assigned")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "project.members.set", id, map[string]any{"user_oids": oids})
		members, err := a.Store.ListMembers(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		writeJSON(w, http.StatusOK, members)
	}
}

// Users: GET /admin/users.
func (a *AdminAPI) Users() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := a.Store.ListUsers(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if users == nil {
			users = []store.UserSummary{}
		}
		writeJSON(w, http.StatusOK, users)
	}
}
