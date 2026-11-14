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
