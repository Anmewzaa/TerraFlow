package localrepo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(context.Background(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateBranchAutoInit(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644)

	res, err := CreateBranch(context.Background(), Request{
		Dir: dir, AutoInit: true, Branch: "terraflow/demo/dev-1234",
		Message: "Add storage",
		Files:   []File{{Path: "storage/main.tf", Content: "module \"a\" {}\n"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit == "" || res.Branch != "terraflow/demo/dev-1234" {
		t.Fatalf("bad result %+v", res)
	}
	// Branch holds the file; main (checked out) does not and stays clean.
	if got := run(t, dir, "show", res.Branch+":storage/main.tf"); !strings.Contains(got, "module") {
		t.Fatalf("file missing on branch: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "storage")); err == nil {
		t.Fatal("working tree was modified")
	}
	if cur := run(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Fatalf("HEAD moved to %s", cur)
	}
	if wts := run(t, dir, "worktree", "list"); strings.Count(wts, "\n") != 0 {
		t.Fatalf("temporary worktree left behind: %s", wts)
	}

	// Same branch again, and an existing file, are conflicts.
	_, err = CreateBranch(context.Background(), Request{
		Dir: dir, Branch: "terraflow/demo/dev-1234", Message: "x",
		Files: []File{{Path: "a/main.tf", Content: "x"}},
	})
	if !errors.Is(err, ErrBranchExists) {
		t.Fatalf("want ErrBranchExists, got %v", err)
	}
	_, err = CreateBranch(context.Background(), Request{
		Dir: dir, Branch: "other", Message: "x",
		Files: []File{{Path: "README.md", Content: "x"}},
	})
	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("want ErrFileExists, got %v", err)
	}
	if out := run(t, dir, "branch", "--list", "other"); out != "" {
		t.Fatal("failed attempt left its branch behind")
	}
}

func TestCreateBranchSubfolderOfRepo(t *testing.T) {
	needGit(t)
	root := t.TempDir()
	run(t, root, "init", "-b", "main")
	sub := filepath.Join(root, "iac", "proj")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "x.tf"), []byte("#"), 0o644)
	run(t, root, "add", "-A")
	run(t, root, "commit", "-m", "init")

	res, err := CreateBranch(context.Background(), Request{
		Dir: sub, Branch: "b1", Message: "m",
		Files: []File{{Path: "vnet/main.tf", Content: "y"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := run(t, root, "show", res.Branch+":iac/proj/vnet/main.tf"); got != "y" {
		t.Fatalf("got %q", got)
	}
}

func TestCreateBranchRejectsUnsafeAndNonRepo(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	for _, p := range []string{"../evil.tf", "/abs.tf", "a/.git/config", "", ".", `C:\x.tf`} {
		_, err := CreateBranch(context.Background(), Request{
			Dir: dir, AutoInit: true, Branch: "b", Message: "m", Files: []File{{Path: p, Content: "x"}},
		})
		if !errors.Is(err, ErrUnsafePath) {
			t.Errorf("path %q: want ErrUnsafePath, got %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error("unsafe request must not initialise a repository")
	}
}
