// Package localrepo creates a review branch in a local git repository.
//
// It stands in for the ADO Pushes API during Phase 5: one commit on a new
// branch, created in a temporary git worktree so the user's checked-out branch
// and working tree are never touched.
package localrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrBranchExists = errors.New("branch already exists")
	ErrFileExists   = errors.New("file already exists in the repository")
	ErrFileMissing  = errors.New("file to update does not exist in the repository")
	ErrFileChanged  = errors.New("file changed in the repository since it was generated")
	ErrNotRepo      = errors.New("project folder is not inside a git repository")
	ErrUnsafePath   = errors.New("unsafe file path")
)

// File is one file to add, with a path relative to Request.Dir.
type File struct {
	Path    string
	Content string
	// Update replaces a file that must already exist in the branch's base;
	// otherwise the file must not exist yet.
	Update bool
	// Base, when set with Update, is the content the new file was built from; the
	// update is refused if the file in the branch's base differs.
	Base string
}

type Request struct {
	// Dir is the project's IaC folder; file paths are relative to it.
	Dir string
	// AutoInit allows `git init` when Dir is not yet in a repository.
	AutoInit bool
	Branch   string
	Message  string
	Files    []File
}

type Result struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	// Repo is the repository root the branch lives in.
	Repo string `json:"repo"`
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	full := append([]string{
		"-c", "user.name=TerraFlow", "-c", "user.email=terraflow@localhost",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func normEOL(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// safeRel validates a generated relative path (no traversal, no .git).
func safeRel(p string) (string, error) {
	// Backslashes and drive prefixes (C:\x) are rejected on every OS: on Linux
	// they are legal filename bytes, but a generated path never contains them.
	if p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.ContainsRune(p, '\\') ||
		filepath.VolumeName(p) != "" || (len(p) >= 2 && p[1] == ':') {
		return "", ErrUnsafePath
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if strings.EqualFold(part, ".git") {
			return "", ErrUnsafePath
		}
	}
	return clean, nil
}

// repoRoot returns the repository containing dir, initialising one if allowed.
func repoRoot(ctx context.Context, dir string, autoInit bool) (string, error) {
	if autoInit {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			if _, err := git(ctx, dir, "init", "-b", "main"); err != nil {
				return "", err
			}
			if _, err := git(ctx, dir, "add", "-A"); err != nil {
				return "", err
			}
			if _, err := git(ctx, dir, "commit", "--allow-empty", "-m", "Initial import"); err != nil {
				return "", err
			}
		}
	}
	top, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotRepo
	}
	return filepath.FromSlash(top), nil
}

// CreateBranch commits Files on a new branch cut from the repository's HEAD.
func CreateBranch(ctx context.Context, req Request) (Result, error) {
	if len(req.Files) == 0 {
		return Result{}, errors.New("no files to commit")
	}
	dir, err := filepath.Abs(req.Dir)
	if err != nil {
		return Result{}, err
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return Result{}, fmt.Errorf("project folder %q is not a directory", dir)
	}
	// Validate every path before touching the repository.
	rels := make([]string, len(req.Files))
	for i, f := range req.Files {
		if rels[i], err = safeRel(f.Path); err != nil {
			return Result{}, fmt.Errorf("%w: %q", err, f.Path)
		}
	}

	root, err := repoRoot(ctx, dir, req.AutoInit)
	if err != nil {
		return Result{}, err
	}
	// EvalSymlinks keeps Rel correct on platforms where temp/short paths differ.
	rootReal, _ := filepath.EvalSymlinks(root)
	dirReal, _ := filepath.EvalSymlinks(dir)
	sub, err := filepath.Rel(rootReal, dirReal)
	if err != nil || strings.HasPrefix(sub, "..") {
		return Result{}, ErrNotRepo
	}

	if out, _ := git(ctx, root, "branch", "--list", req.Branch); out != "" {
		return Result{}, ErrBranchExists
	}

	tmp, err := os.MkdirTemp("", "terraflow-wt-")
	if err != nil {
		return Result{}, err
	}
	wt := filepath.Join(tmp, "wt")
	defer os.RemoveAll(tmp)

	if _, err := git(ctx, root, "worktree", "add", "-b", req.Branch, wt, "HEAD"); err != nil {
		return Result{}, err
	}
	cleanup := func(deleteBranch bool) {
		git(ctx, root, "worktree", "remove", "--force", wt)
		git(ctx, root, "worktree", "prune")
		if deleteBranch {
			git(ctx, root, "branch", "-D", req.Branch)
		}
	}

	var add []string
	for i, f := range req.Files {
		rel := filepath.Join(sub, rels[i])
		target := filepath.Join(wt, rel)
		_, statErr := os.Stat(target)
		switch {
		case !f.Update && statErr == nil:
			cleanup(true)
			return Result{}, fmt.Errorf("%w: %s", ErrFileExists, filepath.ToSlash(rel))
		case f.Update && statErr != nil:
			cleanup(true)
			return Result{}, fmt.Errorf("%w: %s", ErrFileMissing, filepath.ToSlash(rel))
		case f.Update && f.Base != "":
			cur, err := os.ReadFile(target)
			if err != nil || normEOL(string(cur)) != normEOL(f.Base) {
				cleanup(true)
				return Result{}, fmt.Errorf("%w: %s", ErrFileChanged, filepath.ToSlash(rel))
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			cleanup(true)
			return Result{}, err
		}
		if err := os.WriteFile(target, []byte(f.Content), 0o644); err != nil {
			cleanup(true)
			return Result{}, err
		}
		add = append(add, filepath.ToSlash(rel))
	}

	if _, err := git(ctx, wt, append([]string{"add", "--"}, add...)...); err != nil {
		cleanup(true)
		return Result{}, err
	}
	if _, err := git(ctx, wt, "commit", "-m", req.Message); err != nil {
		cleanup(true)
		return Result{}, err
	}
	sha, err := git(ctx, wt, "rev-parse", "HEAD")
	if err != nil {
		cleanup(true)
		return Result{}, err
	}
	cleanup(false)
	return Result{Branch: req.Branch, Commit: sha, Repo: root}, nil
}
