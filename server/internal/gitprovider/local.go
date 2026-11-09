package gitprovider

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/localrepo"
)

// Local is a Source over a folder of this machine, for development and tests.
// Unlike the remote sources it uses the `git` binary and the disk.
type Local struct {
	// Dir is the project's folder (each sub-folder is a root module).
	Dir string
	// AutoInit allows `git init` when Dir is not yet inside a repository.
	AutoInit bool
}

// Snapshot reads Dir. A folder that does not exist yet is an empty project.
func (l Local) Snapshot(ctx context.Context) (Snapshot, error) {
	return Snapshot{Commit: l.head(ctx), FS: os.DirFS(l.Dir)}, nil
}

// head is the commit of the repository around Dir, or "" if there is none.
func (l Local) head(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "-C", l.Dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CreateBranch commits on a new branch of the local repository. The branch is made
// from the repository's current HEAD (the local repository is a development stand-in).
func (l Local) CreateBranch(ctx context.Context, req BranchRequest) (Result, error) {
	files := make([]localrepo.File, len(req.Files))
	for i, f := range req.Files {
		files[i] = localrepo.File{Path: f.Path, Content: f.Content, Update: f.Update, Base: f.Previous}
	}
	res, err := localrepo.CreateBranch(ctx, localrepo.Request{
		Dir: l.Dir, AutoInit: l.AutoInit, Branch: req.Branch, Message: req.Message, Files: files,
	})
	if err != nil {
		return Result{}, mapLocalError(err)
	}
	return Result{Branch: res.Branch, Commit: res.Commit, Repo: res.Repo}, nil
}

// mapLocalError turns localrepo's errors into the package's, keeping the original
// as the cause so its message is not lost.
func mapLocalError(err error) error {
	pairs := []struct{ from, to error }{
		{localrepo.ErrBranchExists, ErrBranchExists},
		{localrepo.ErrFileExists, ErrFileExists},
		{localrepo.ErrFileMissing, ErrFileMissing},
		{localrepo.ErrFileChanged, ErrFileChanged},
		{localrepo.ErrUnsafePath, ErrUnsafePath},
	}
	for _, p := range pairs {
		if errors.Is(err, p.from) {
			return &wrapped{to: p.to, cause: err}
		}
	}
	return err
}

// wrapped makes errors.Is match the package's error while Error() keeps the detail.
type wrapped struct{ to, cause error }

func (w *wrapped) Error() string   { return w.cause.Error() }
func (w *wrapped) Unwrap() []error { return []error{w.to, w.cause} }
