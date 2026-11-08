package localrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateBranchUpdatesExistingFile(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "common"), 0o755)
	orig := "module \"a\" {}\n"
	os.WriteFile(filepath.Join(dir, "common", "rg.tf"), []byte(orig), 0o644)

	res, err := CreateBranch(context.Background(), Request{
		Dir: dir, AutoInit: true, Branch: "b1", Message: "edit",
		Files: []File{{Path: "common/rg.tf", Content: orig + "\nmodule \"b\" {}\n", Update: true, Base: orig}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := run(t, dir, "show", res.Branch+":common/rg.tf"); !strings.Contains(got, `module "b"`) || !strings.Contains(got, `module "a"`) {
		t.Fatalf("update not committed: %q", got)
	}
	if disk, _ := os.ReadFile(filepath.Join(dir, "common", "rg.tf")); string(disk) != orig {
		t.Fatal("working tree changed")
	}
}

func TestCreateBranchUpdateGuards(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.tf"), []byte("old\n"), 0o644)
	ctx := context.Background()

	_, err := CreateBranch(ctx, Request{Dir: dir, AutoInit: true, Branch: "b1", Message: "m",
		Files: []File{{Path: "nope.tf", Content: "x", Update: true}}})
	if !errors.Is(err, ErrFileMissing) {
		t.Fatalf("update of a missing file: %v", err)
	}
	_, err = CreateBranch(ctx, Request{Dir: dir, Branch: "b2", Message: "m",
		Files: []File{{Path: "a.tf", Content: "x", Update: true, Base: "something else\n"}}})
	if !errors.Is(err, ErrFileChanged) {
		t.Fatalf("stale base: %v", err)
	}
	// Failed attempts leave no branch and no worktree behind.
	if out := run(t, dir, "branch", "--list", "b1", "b2"); strings.TrimSpace(out) != "" {
		t.Fatalf("branches left behind: %q", out)
	}
	if wts := run(t, dir, "worktree", "list"); strings.Count(wts, "\n") != 0 {
		t.Fatalf("worktree left behind: %s", wts)
	}
	// Creating a file that exists is still refused.
	_, err = CreateBranch(ctx, Request{Dir: dir, Branch: "b3", Message: "m", Files: []File{{Path: "a.tf", Content: "x"}}})
	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("create over existing: %v", err)
	}
}
