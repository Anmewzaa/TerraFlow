package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type createProjectRequest struct {
	Name    string `json:"name"`
	IACPath string `json:"iac_path"`
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases name and collapses anything non-alphanumeric to "-".
func slugify(name string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// within reports whether path is root or lives under it (both made absolute).
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// ProjectCreateHandler registers a project. With no iac_path it creates
// <workspaceDir>/<slug>; an explicit path must be an existing directory
// inside the workspace dir or beside the sample project.
func ProjectCreateHandler(q *store.Queries, workspaceDir, sampleDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createProjectRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		name := strings.TrimSpace(req.Name)
		slug := slugify(name)
		if name == "" || slug == "" || len(name) > 80 {
			writeJSONError(w, http.StatusBadRequest, "name is required (letters/digits, max 80 chars)")
			return
		}

		wsAbs, err := filepath.Abs(workspaceDir)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "workspace dir unavailable")
			return
		}
		samplesAbs, _ := filepath.Abs(filepath.Dir(sampleDir))

		var dir string
		if p := strings.TrimSpace(req.IACPath); p != "" {
			dir, err = filepath.Abs(p)
			if err != nil || !(within(wsAbs, dir) || within(samplesAbs, dir)) {
				writeJSONError(w, http.StatusBadRequest, "iac_path must be inside the workspace directory")
				return
			}
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				writeJSONError(w, http.StatusBadRequest, "iac_path is not an existing directory")
				return
			}
		} else {
			dir = filepath.Join(wsAbs, slug)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "cannot create project directory")
				return
			}
		}

		p, err := q.CreateProject(r.Context(), store.Project{
			Name: name, Slug: slug, IACPath: dir, SourceType: "local",
		})
		if errors.Is(err, store.ErrSlugTaken) {
			writeJSONError(w, http.StatusConflict, "a project with this name already exists")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

// ProjectDeleteHandler removes a project and its drafts/changesets from the DB.
// IaC files on disk are left untouched.
func ProjectDeleteHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := q.DeleteProject(r.Context(), chi.URLParam(r, "id"))
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
