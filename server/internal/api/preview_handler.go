package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// ChangesetSource is what preview and revise need; Postgres and the NoDB
// in-memory store both implement it.
type ChangesetSource interface {
	GetChangeset(ctx context.Context, id string) (store.Changeset, error)
	CreateRevision(ctx context.Context, src store.Changeset, userOID string) (string, error)
}

type previewResponse struct {
	// Files is what the current canvas generates; BaseFiles is what the
	// changeset's base canvas generated (empty for a brand-new changeset), so
	// the client can colour added / changed / removed lines.
	Files     []hcl.GeneratedFile `json:"files"`
	BaseFiles []hcl.GeneratedFile `json:"base_files"`
	// RepoFiles are the .tf files that already exist in the project folder, so
	// the preview shows the current code with the draft's changes on top.
	RepoFiles []hcl.GeneratedFile `json:"repo_files"`
}

const (
	maxRepoFiles    = 300
	maxRepoFileSize = 256 << 10
)

// readRepoFiles returns folder/*.tf under dir (one level of folders, as the
// team's layout uses). Unreadable or oversized files are skipped, never fatal.
func readRepoFiles(dir string) []hcl.GeneratedFile {
	out := []hcl.GeneratedFile{}
	folders, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, f := range folders {
		if !f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Size() > maxRepoFileSize {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, f.Name(), e.Name()))
			if err != nil {
				continue
			}
			out = append(out, hcl.GeneratedFile{Folder: f.Name(), Filename: e.Name(), Content: string(b)})
			if len(out) >= maxRepoFiles {
				return out
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Folder != out[j].Folder {
			return out[i].Folder < out[j].Folder
		}
		return out[i].Filename < out[j].Filename
	})
	return out
}

func generateFromCanvas(raw json.RawMessage) ([]hcl.GeneratedFile, error) {
	if len(raw) == 0 {
		return []hcl.GeneratedFile{}, nil
	}
	files, _, err := generate(store.Changeset{CanvasJSON: raw})
	if files == nil {
		files = []hcl.GeneratedFile{}
	}
	return files, err
}

// ChangesetPreviewHandler: GET /projects/{id}/changesets/{csId}/preview
// project may be nil; then no repository files are included.
func ChangesetPreviewHandler(src ChangesetSource, project func(ctx context.Context, id string) (store.Project, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := src.GetChangeset(r.Context(), chi.URLParam(r, "csId"))
		if err != nil || cs.ProjectID != chi.URLParam(r, "id") {
			writeJSONError(w, http.StatusNotFound, "changeset not found")
			return
		}
		files, err := generateFromCanvas(cs.CanvasJSON)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "generation failed: "+err.Error())
			return
		}
		base, err := generateFromCanvas(cs.BaseCanvas)
		if err != nil {
			base = []hcl.GeneratedFile{} // a stale base must not block the preview
		}
		repo := []hcl.GeneratedFile{}
		if project != nil {
			if p, err := project(r.Context(), cs.ProjectID); err == nil {
				repo = readRepoFiles(p.IACPath)
			}
		}
		writeJSON(w, http.StatusOK, previewResponse{Files: files, BaseFiles: base, RepoFiles: repo})
	}
}

// ProjectFilesHandler: GET /projects/{id}/files
// The project's existing .tf files, so the preview shows current code even
// before any changeset exists.
func ProjectFilesHandler(project func(ctx context.Context, id string) (store.Project, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := project(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"repo_files": readRepoFiles(p.IACPath)})
	}
}

// ChangesetReviseHandler: POST /projects/{id}/changesets/{csId}/revise
// Opens a published, approved or rejected changeset as a new draft to keep
// editing; the original is left untouched.
func ChangesetReviseHandler(src ChangesetSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := src.GetChangeset(r.Context(), chi.URLParam(r, "csId"))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && cs.ProjectID != chi.URLParam(r, "id")) {
			writeJSONError(w, http.StatusNotFound, "changeset not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not load the changeset")
			return
		}
		if cs.Status == store.ChangesetDraft {
			writeJSONError(w, http.StatusConflict, "this changeset is already an editable draft")
			return
		}
		oid := ""
		if u := auth.UserFromContext(r.Context()); u != nil {
			oid = u.OID
		}
		id, err := src.CreateRevision(r.Context(), cs, oid)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not create the revision")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

func buildModuleSource(moduleName, version string) string {
	return "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//" + moduleName + "?ref=" + version
}
