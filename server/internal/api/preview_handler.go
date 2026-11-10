package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
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
	// Files are the files the current canvas creates or changes, each with its
	// full final content (existing code kept, new code appended or the edited
	// block replaced). BaseFiles is the same for the changeset's base canvas
	// (empty for a brand-new changeset), so the client can colour added /
	// changed / removed lines.
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

// readRepoFilesFS returns folder/*.tf of a snapshot (one level of folders, as the
// team's layout uses). Unreadable or oversized files are skipped, never fatal.
func readRepoFilesFS(snap gitprovider.Snapshot) []hcl.GeneratedFile {
	out := []hcl.GeneratedFile{}
	if snap.FS == nil {
		return out
	}
	folders, err := fs.ReadDir(snap.FS, ".")
	if err != nil {
		return out
	}
	for _, f := range folders {
		if !f.IsDir() || isHidden(f.Name()) {
			continue
		}
		entries, err := fs.ReadDir(snap.FS, f.Name())
		if err != nil {
			continue
		}
		// env/<env>.tfvars: the values files TerraFlow may add lines to (D-057).
		if envs, err := fs.ReadDir(snap.FS, path.Join(f.Name(), "env")); err == nil {
			for _, e := range envs {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".tfvars") {
					continue
				}
				if info, err := e.Info(); err != nil || info.Size() > maxRepoFileSize {
					continue
				}
				if b, err := fs.ReadFile(snap.FS, path.Join(f.Name(), "env", e.Name())); err == nil {
					out = append(out, hcl.GeneratedFile{Folder: f.Name() + "/env", Filename: e.Name(), Content: string(b)})
				}
			}
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Size() > maxRepoFileSize {
				continue
			}
			b, err := fs.ReadFile(snap.FS, path.Join(f.Name(), e.Name()))
			if err != nil {
				continue
			}
			out = append(out, hcl.GeneratedFile{Folder: f.Name(), Filename: e.Name(), Content: string(b)})
			if len(out) >= maxRepoFiles {
				sortRepoFiles(out)
				return out
			}
		}
	}
	sortRepoFiles(out)
	return out
}

// generateFromCanvas merges the canvas into the repository's files.
func generateFromCanvas(raw json.RawMessage, repo []hcl.GeneratedFile, settings *hcl.ProjectSettings) ([]hcl.GeneratedFile, error) {
	files := []hcl.GeneratedFile{}
	if len(raw) == 0 {
		return files, nil
	}
	changed, _, err := applyCanvas(raw, repo, settings)
	for _, f := range changed {
		files = append(files, f.GeneratedFile)
	}
	return files, err
}

// ChangesetPreviewHandler: GET /projects/{id}/changesets/{csId}/preview
// project may be nil; then no repository files are included.
func ChangesetPreviewHandler(src ChangesetSource, project func(ctx context.Context, id string) (store.Project, error), repos Repos, settings SettingsLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := src.GetChangeset(r.Context(), chi.URLParam(r, "csId"))
		if err != nil || cs.ProjectID != chi.URLParam(r, "id") {
			writeJSONError(w, http.StatusNotFound, "changeset not found")
			return
		}
		repo := []hcl.GeneratedFile{}
		if project != nil {
			if p, err := project(r.Context(), cs.ProjectID); err == nil {
				repo = repoFilesOf(r.Context(), repos, p)
			}
		}
		st := settings.of(r.Context(), cs.ProjectID)
		files, err := generateFromCanvas(cs.CanvasJSON, repo, st)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "generation failed: "+err.Error())
			return
		}
		base, err := generateFromCanvas(cs.BaseCanvas, repo, st)
		if err != nil {
			base = []hcl.GeneratedFile{} // a stale base must not block the preview
		}
		writeJSON(w, http.StatusOK, previewResponse{Files: files, BaseFiles: base, RepoFiles: repo})
	}
}

// ProjectFilesHandler: GET /projects/{id}/files
// The project's existing .tf files, so the preview shows current code even
// before any changeset exists.
func ProjectFilesHandler(project func(ctx context.Context, id string) (store.Project, error), repos Repos) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := project(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"repo_files": repoFilesOf(r.Context(), repos, p)})
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
		if cs.Status == store.ChangesetDraft || cs.Status == store.ChangesetSaved {
			writeJSONError(w, http.StatusConflict, "this changeset has not been sent for review yet")
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

// ProjectImportHandler: GET /projects/{id}/import
// Reads every module call already in the project's folders and returns them as
// canvas nodes that point back at their block (existing), ready to be edited
// in place. Read-only: nothing is written until a changeset is approved.
func ProjectImportHandler(project func(ctx context.Context, id string) (store.Project, error), repos Repos) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := project(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		snap, err := snapshotOf(r.Context(), repos, p)
		if err != nil {
			writeJSONError(w, providerStatus(err), err.Error())
			return
		}
		nodes, err := hcl.ImportModulesFS(snap.FS)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if nodes == nil {
			nodes = []hcl.GenerateNode{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
	}
}
