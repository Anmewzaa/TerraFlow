package api

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/go-chi/chi/v5"
)

// Repos returns the Source that reads and writes a project's repository. Handlers
// never look at a project's folder themselves, so a project can live on any provider.
type Repos func(ctx context.Context, p store.Project) (gitprovider.Source, error)

// LocalRepos treats every project as a folder of this machine (p.IACPath), the
// development and test provider. Folders inside workspaceDir may be git-initialised
// on the first approve.
func LocalRepos(workspaceDir string) Repos {
	return func(_ context.Context, p store.Project) (gitprovider.Source, error) {
		return gitprovider.Local{Dir: p.IACPath, AutoInit: inside(workspaceDir, p.IACPath)}, nil
	}
}

func inside(workspaceDir, dir string) bool {
	if workspaceDir == "" {
		return false
	}
	ws, err1 := filepath.Abs(workspaceDir)
	d, err2 := filepath.Abs(dir)
	return err1 == nil && err2 == nil && within(ws, d)
}

// graphName is the project name shown in the graph.
func graphName(p store.Project) string {
	if p.IACPath != "" {
		return filepath.Base(p.IACPath)
	}
	return p.Slug
}

// snapshotOf reads the project's repository at the head of its branch.
func snapshotOf(ctx context.Context, repos Repos, p store.Project) (gitprovider.Snapshot, error) {
	src, err := repos(ctx, p)
	if err != nil {
		return gitprovider.Snapshot{}, err
	}
	return src.Snapshot(ctx)
}

// envOf is the environment a request is about ("dev" when it does not say).
func envOf(r *http.Request) string {
	if e := r.URL.Query().Get("env"); hcl.ValidEnv(e) {
		return e
	}
	return "dev"
}

func firstSettings(l []SettingsLoader) SettingsLoader {
	if len(l) == 0 {
		return nil
	}
	return l[0]
}

// viewOf shows a snapshot in the shape the parser and generator work in, for one environment
// (8.9). With the default layout the snapshot is returned as it is.
func viewOf(snap gitprovider.Snapshot, st *hcl.ProjectSettings, env string) (gitprovider.Snapshot, error) {
	r := st.Resolver(env)
	if r.Identity() {
		return snap, nil
	}
	v, err := r.View(snap.FS)
	if err != nil {
		return snap, err
	}
	snap.FS = v
	return snap, nil
}

// joinPath joins a folder and a file; the project folder itself is "".
func joinPath(folder, file string) string {
	if folder == "" || folder == "." {
		return file
	}
	return folder + "/" + file
}

// realFiles says where files of the canonical shape live in the repository (8.9) and drops the
// ones the layout has no place for.
func realFiles(r *hcl.Resolver, in []hcl.GeneratedFile) []hcl.GeneratedFile {
	if r.Identity() {
		return in
	}
	out := make([]hcl.GeneratedFile, 0, len(in))
	for _, f := range in {
		if folder, name, ok := r.Real(f.Folder, f.Filename); ok {
			out = append(out, hcl.GeneratedFile{Folder: folder, Filename: name, Content: f.Content})
		}
	}
	return out
}

// repoFilesOf returns folder/*.tf of a snapshot, for the plan and the preview. It
// is best effort, as before: a missing or unreadable repository gives no files.
func repoFilesOf(ctx context.Context, repos Repos, p store.Project, st *hcl.ProjectSettings, env string) []hcl.GeneratedFile {
	snap, err := snapshotOf(ctx, repos, p)
	if err == nil {
		snap, err = viewOf(snap, st, env)
	}
	if err != nil {
		return []hcl.GeneratedFile{}
	}
	return readRepoFilesFS(snap)
}

func sortRepoFiles(out []hcl.GeneratedFile) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Folder != out[j].Folder {
			return out[i].Folder < out[j].Folder
		}
		return out[i].Filename < out[j].Filename
	})
}

// isHidden reports names that are not Terraform folders (".git", ".github").
func isHidden(name string) bool { return strings.HasPrefix(name, ".") }

// providerStatus maps a provider error to the HTTP status the client should see.
func providerStatus(err error) int {
	switch {
	case errors.Is(err, gitprovider.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, gitprovider.ErrUnauthorized):
		// The project's connection is broken (revoked, expired, no access).
		return http.StatusConflict
	case errors.Is(err, gitprovider.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, gitprovider.ErrBranchExists), errors.Is(err, gitprovider.ErrFileExists),
		errors.Is(err, gitprovider.ErrFileMissing), errors.Is(err, gitprovider.ErrFileChanged):
		return http.StatusConflict
	case errors.Is(err, gitprovider.ErrEmptyRepo):
		return http.StatusUnprocessableEntity
	case errors.Is(err, gitprovider.ErrUnsafePath):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// LayoutDetectHandler: GET /projects/{id}/layout/detect reads the folder names of the repository
// and proposes the folder and values patterns (8.9). Read-only; the owner accepts or edits.
func LayoutDetectHandler(project func(ctx context.Context, id string) (store.Project, error), repos Repos, settings SettingsLoader) http.HandlerFunc {
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
		var envs []string
		if st := settings.of(r.Context(), p.ID); st != nil {
			envs = st.Environments
		}
		writeJSON(w, http.StatusOK, hcl.DetectLayout(snap.FS, envs))
	}
}

// EnvDiffHandler: GET /projects/{id}/env-diff?from=&to= compares two environments of the
// repository: resources paired by stack and label, argument by argument; a value that cannot be
// evaluated is "unknown", never equal (rule 8). Read-only.
func EnvDiffHandler(project func(ctx context.Context, id string) (store.Project, error), repos Repos, settings SettingsLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
		if !hcl.ValidEnv(from) || !hcl.ValidEnv(to) || from == to {
			writeJSONError(w, http.StatusBadRequest, "choose two different environments (from and to)")
			return
		}
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
		st := settings.of(r.Context(), p.ID)
		graphs := make([]*hcl.Graph, 2)
		for i, env := range []string{from, to} {
			v, err := viewOf(snap, st, env)
			if err == nil {
				graphs[i], err = hcl.BuildGraphFS(v.FS, graphName(p), env)
			}
			if err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, hcl.DiffGraphs(graphs[0], graphs[1], from, to, st))
	}
}

// LayoutCheckHandler: GET /projects/{id}/layout/check?pattern=&values= lists which folders of the
// repository fit the given patterns (the ones being edited, not the saved ones). Read-only.
func LayoutCheckHandler(project func(ctx context.Context, id string) (store.Project, error), repos Repos, settings SettingsLoader) http.HandlerFunc {
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
		var envs []string
		if st := settings.of(r.Context(), p.ID); st != nil {
			envs = st.Environments
		}
		q := r.URL.Query()
		folders, err := hcl.CheckLayout(snap.FS, hcl.Layout{Pattern: q.Get("pattern"), Values: q.Get("values")}, envs)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		misfits := 0
		for _, f := range folders {
			if !f.Fits {
				misfits++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"folders": folders, "misfits": misfits})
	}
}

// EnvCopyHandler: GET /projects/{id}/env-copy?from=&to=&id= proposes the module call `id` of
// environment from as a new node for environment to, named for it. It writes nothing: the person
// adds it to a draft, which goes through validate, review and approve like any change (8.10). A
// block that already exists in to is refused (D-036): edit it there instead.
func EnvCopyHandler(project func(ctx context.Context, id string) (store.Project, error), repos Repos, settings SettingsLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from, to, id := q.Get("from"), q.Get("to"), q.Get("id")
		if !hcl.ValidEnv(from) || !hcl.ValidEnv(to) || from == to || id == "" {
			writeJSONError(w, http.StatusBadRequest, "choose two different environments and a resource")
			return
		}
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
		st := settings.of(r.Context(), p.ID)
		src, err := viewOf(snap, st, from)
		if err == nil {
			var dst gitprovider.Snapshot
			if dst, err = viewOf(snap, st, to); err == nil {
				var g *hcl.Graph
				if g, err = hcl.BuildGraphFS(dst.FS, graphName(p), to); err == nil {
					for _, f := range g.Folders {
						for _, n := range f.Nodes {
							if n.ID == id {
								writeJSONError(w, http.StatusConflict, "this resource already exists in "+to+"; edit it there instead")
								return
							}
						}
					}
				}
			}
		}
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		nodes, err := hcl.ImportModulesFS(src.FS)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		for _, n := range nodes {
			if n.Existing != nil && n.Existing.Folder+".module."+n.Existing.Label == id {
				writeJSON(w, http.StatusOK, map[string]any{"node": hcl.CopyNodeToEnv(n, from, to), "env": to})
				return
			}
		}
		writeJSONError(w, http.StatusNotFound, "that resource is not a module call in "+from+"; only module calls can be copied")
	}
}
