package gitprovider

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T) (root, project string) {
	t.Helper()
	needGit(t)
	root = t.TempDir()
	run(t, root, "init", "-q", "-b", "main")
	project = filepath.Join(root, "iac")
	if err := os.MkdirAll(filepath.Join(project, "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "common", "rg.tf"), []byte("# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "add", ".")
	run(t, root, "commit", "-q", "-m", "init")
	return root, project
}

func TestLocalSnapshotReadsTheFolderAndTheCommit(t *testing.T) {
	root, project := newRepo(t)
	snap, err := Local{Dir: project}.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Commit != run(t, root, "rev-parse", "HEAD") {
		t.Errorf("commit %q", snap.Commit)
	}
	b, err := fs.ReadFile(snap.FS, "common/rg.tf")
	if err != nil || string(b) != "# old\n" {
		t.Fatalf("file: %q %v", b, err)
	}
}

func TestLocalSnapshotOfAMissingFolderIsEmpty(t *testing.T) {
	snap, err := Local{Dir: filepath.Join(t.TempDir(), "nope")}.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Commit != "" {
		t.Errorf("no repository means no commit, got %q", snap.Commit)
	}
	if _, err := fs.ReadDir(snap.FS, "."); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the folder is not there: %v", err)
	}
}

func TestLocalCreateBranch(t *testing.T) {
	root, project := newRepo(t)
	l := Local{Dir: project}
	res, err := l.CreateBranch(context.Background(), BranchRequest{
		Branch: "terraflow/p/dev-1", Message: "add",
		Files: []FileChange{
			{Path: "common/rg.tf", Content: "# old\n# new\n", Update: true, Previous: "# old\n"},
			{Path: "net/vnet.tf", Content: "# vnet\n"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Branch != "terraflow/p/dev-1" || res.Commit == "" || res.Repo == "" {
		t.Fatalf("result: %+v", res)
	}
	if got := run(t, root, "show", res.Branch+":iac/net/vnet.tf"); got != "# vnet" {
		t.Errorf("new file: %q", got)
	}
	if got := run(t, root, "show", res.Branch+":iac/common/rg.tf"); got != "# old\n# new" {
		t.Errorf("updated file: %q", got)
	}
	// The checked-out branch was not touched.
	if got := run(t, root, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Errorf("checked out: %q", got)
	}
}

func TestLocalCreateBranchErrorsAreThePackagesErrors(t *testing.T) {
	_, project := newRepo(t)
	l := Local{Dir: project}
	ctx := context.Background()
	ok := BranchRequest{Branch: "b1", Message: "m", Files: []FileChange{{Path: "net/a.tf", Content: "x"}}}
	if _, err := l.CreateBranch(ctx, ok); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		req  BranchRequest
		want error
	}{
		"branch exists":    {BranchRequest{Branch: "b1", Message: "m", Files: []FileChange{{Path: "net/b.tf", Content: "x"}}}, ErrBranchExists},
		"file exists":      {BranchRequest{Branch: "b2", Message: "m", Files: []FileChange{{Path: "common/rg.tf", Content: "x"}}}, ErrFileExists},
		"update missing":   {BranchRequest{Branch: "b3", Message: "m", Files: []FileChange{{Path: "common/none.tf", Content: "x", Update: true}}}, ErrFileMissing},
		"update changed":   {BranchRequest{Branch: "b4", Message: "m", Files: []FileChange{{Path: "common/rg.tf", Content: "x", Update: true, Previous: "# something else\n"}}}, ErrFileChanged},
		"unsafe path":      {BranchRequest{Branch: "b5", Message: "m", Files: []FileChange{{Path: "../evil.tf", Content: "x"}}}, ErrUnsafePath},
		"unsafe drive":     {BranchRequest{Branch: "b6", Message: "m", Files: []FileChange{{Path: `C:\x.tf`, Content: "x"}}}, ErrUnsafePath},
		"unsafe backslash": {BranchRequest{Branch: "b7", Message: "m", Files: []FileChange{{Path: `a\..\x.tf`, Content: "x"}}}, ErrUnsafePath},
	}
	for name, tc := range cases {
		_, err := l.CreateBranch(ctx, tc.req)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
		if err != nil && strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s: the message must keep the detail", name)
		}
	}
}
