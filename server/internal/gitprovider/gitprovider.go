// Package gitprovider is the one place that reads a project's repository and
// writes its review branch. A Source is bound to one project (repository, branch
// and folder); the `local` implementation uses a folder on this machine and the
// `github` implementation uses the GitHub API. An Azure Repos implementation can
// satisfy the same interface later.
//
// Rules for every implementation (D-049): no `git` binary and no disk in the
// remote implementations, no state kept between calls, and never a credential in a
// returned value or an error message.
package gitprovider

import (
	"context"
	"errors"
	"io/fs"
)

var (
	// ErrBranchExists: the branch name is already taken. Nothing was changed.
	ErrBranchExists = errors.New("branch already exists")
	// ErrFileExists: a new file would overwrite one that already exists.
	ErrFileExists = errors.New("file already exists in the repository")
	// ErrFileMissing: a file to update is not in the repository.
	ErrFileMissing = errors.New("file to update does not exist in the repository")
	// ErrFileChanged: a file to update differs from what the change was built on.
	ErrFileChanged = errors.New("file changed in the repository since it was generated")
	// ErrUnsafePath: a path that could leave the project folder.
	ErrUnsafePath = errors.New("unsafe file path")
	// ErrNotFound: the repository, branch or folder does not exist (or is not visible).
	ErrNotFound = errors.New("repository, branch or folder not found")
	// ErrUnauthorized: the credential was refused (revoked, expired, no access).
	ErrUnauthorized = errors.New("the git provider refused the credential")
	// ErrRateLimited: the provider's API limit was reached.
	ErrRateLimited = errors.New("the git provider's rate limit was reached")
)

// Snapshot is the project's folder as it was at one commit. FS is rooted at the
// project folder: its top level holds one directory per Terraform root module.
type Snapshot struct {
	// Commit is the commit the files were read at ("" when the source has none).
	Commit string
	FS     fs.FS
}

// FileChange is one file to write, with a path relative to the project folder.
type FileChange struct {
	Path    string
	Content string
	// Update replaces a file that must already exist; otherwise it must not exist.
	Update bool
	// Previous, with Update, is the content the new file was built from; the change
	// is refused if the file in the repository differs.
	Previous string
}

// BranchRequest asks for one commit on a new branch.
type BranchRequest struct {
	// Base is the commit the change was planned on (Snapshot.Commit). A remote source
	// branches from exactly this commit, so the branch holds what the reviewer saw.
	Base    string
	Branch  string
	Message string
	Files   []FileChange
}

// Result describes the branch that was created.
type Result struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	// Repo names where the branch lives (a path for local, owner/name for GitHub).
	Repo string `json:"repo"`
	// URL opens the branch in the provider's web UI, when there is one.
	URL string `json:"url,omitempty"`
}

// Source reads one project's repository and creates its review branch.
type Source interface {
	// Snapshot reads the project's folder at the head of its branch.
	Snapshot(ctx context.Context) (Snapshot, error)
	// CreateBranch writes one commit on a new branch. It never overwrites an
	// existing branch and never force-pushes.
	CreateBranch(ctx context.Context, req BranchRequest) (Result, error)
}
