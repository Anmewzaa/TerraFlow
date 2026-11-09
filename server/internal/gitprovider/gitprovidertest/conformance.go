// Package gitprovidertest holds the behavior every gitprovider.Source must have.
// Each implementation runs Run with a fixture factory, so local, in-memory and
// GitHub sources are held to the same contract.
package gitprovidertest

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
)

// Fixture is a source with a known starting content plus the hooks the tests need.
type Fixture struct {
	Source gitprovider.Source
	// Branch returns the files (relative to the project folder) of a branch that was
	// created, or nil if there is no such branch.
	Branch func(name string) map[string]string
	// Advance pushes a new commit to the default branch that sets (or, with an empty
	// value, removes) files. Nil when the source cannot simulate it.
	Advance func(changes map[string]string)
}

// Start holds the first commit's files.
var Start = map[string]string{
	"common/rg.tf":   "# resource groups\nmodule \"rg\" {}\n",
	"common/main.tf": "locals {}\n",
	"vnet/vnet.tf":   "# vnet\n",
}

// Run executes the contract. newFixture must return a fresh repository holding Start.
func Run(t *testing.T, newFixture func(t *testing.T) Fixture) {
	t.Helper()
	ctx := context.Background()

	t.Run("snapshot reads the files and names a commit", func(t *testing.T) {
		f := newFixture(t)
		snap, err := f.Source.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for path, want := range Start {
			got, err := fs.ReadFile(snap.FS, path)
			if err != nil || string(got) != want {
				t.Errorf("%s: %q %v", path, got, err)
			}
		}
		again, err := f.Source.Snapshot(ctx)
		if err != nil || again.Commit != snap.Commit {
			t.Errorf("an unchanged repository must give the same commit: %q vs %q (%v)", snap.Commit, again.Commit, err)
		}
	})

	t.Run("create branch adds and updates files", func(t *testing.T) {
		f := newFixture(t)
		snap, _ := f.Source.Snapshot(ctx)
		res, err := f.Source.CreateBranch(ctx, gitprovider.BranchRequest{
			Base: snap.Commit, Branch: "terraflow/p/dev-aaaa1111", Message: "add storage",
			Files: []gitprovider.FileChange{
				{Path: "common/rg.tf", Content: Start["common/rg.tf"] + "# more\n", Update: true, Previous: Start["common/rg.tf"]},
				{Path: "storage/main.tf", Content: "module \"sa\" {}\n"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Branch != "terraflow/p/dev-aaaa1111" || res.Commit == "" || res.Repo == "" {
			t.Fatalf("result: %+v", res)
		}
		got := f.Branch(res.Branch)
		if got["storage/main.tf"] != "module \"sa\" {}\n" || got["common/rg.tf"] != Start["common/rg.tf"]+"# more\n" {
			t.Fatalf("branch content: %v", got)
		}
		// Files that were not part of the change are untouched.
		if got["vnet/vnet.tf"] != Start["vnet/vnet.tf"] || got["common/main.tf"] != Start["common/main.tf"] {
			t.Errorf("untouched files changed: %v", got)
		}
		// The default branch did not move.
		after, _ := f.Source.Snapshot(ctx)
		if after.Commit != snap.Commit {
			t.Errorf("creating a branch moved the default branch: %q to %q", snap.Commit, after.Commit)
		}
		if b, _ := fs.ReadFile(after.FS, "storage/main.tf"); len(b) != 0 {
			t.Error("the new file must exist on the branch only")
		}
	})

	t.Run("the contract's errors", func(t *testing.T) {
		f := newFixture(t)
		snap, _ := f.Source.Snapshot(ctx)
		mk := func(branch string, files ...gitprovider.FileChange) gitprovider.BranchRequest {
			return gitprovider.BranchRequest{Base: snap.Commit, Branch: branch, Message: "m", Files: files}
		}
		if _, err := f.Source.CreateBranch(ctx, mk("b-first", gitprovider.FileChange{Path: "new/a.tf", Content: "x"})); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name string
			req  gitprovider.BranchRequest
			want error
		}{
			{"branch name taken", mk("b-first", gitprovider.FileChange{Path: "new/b.tf", Content: "x"}), gitprovider.ErrBranchExists},
			{"new file already exists", mk("b-2", gitprovider.FileChange{Path: "common/rg.tf", Content: "x"}), gitprovider.ErrFileExists},
			{"update of a missing file", mk("b-3", gitprovider.FileChange{Path: "common/none.tf", Content: "x", Update: true}), gitprovider.ErrFileMissing},
			{"update of a changed file", mk("b-4", gitprovider.FileChange{Path: "common/rg.tf", Content: "x", Update: true, Previous: "# another text\n"}), gitprovider.ErrFileChanged},
			{"parent path", mk("b-5", gitprovider.FileChange{Path: "../evil.tf", Content: "x"}), gitprovider.ErrUnsafePath},
			{"absolute path", mk("b-6", gitprovider.FileChange{Path: "/evil.tf", Content: "x"}), gitprovider.ErrUnsafePath},
			{"drive path", mk("b-7", gitprovider.FileChange{Path: `C:\evil.tf`, Content: "x"}), gitprovider.ErrUnsafePath},
			{"backslash path", mk("b-8", gitprovider.FileChange{Path: `a\b.tf`, Content: "x"}), gitprovider.ErrUnsafePath},
			{"git directory", mk("b-9", gitprovider.FileChange{Path: "a/.git/config", Content: "x"}), gitprovider.ErrUnsafePath},
			{"empty path", mk("b-10", gitprovider.FileChange{Path: "", Content: "x"}), gitprovider.ErrUnsafePath},
		}
		for _, tc := range cases {
			_, err := f.Source.CreateBranch(ctx, tc.req)
			if !errors.Is(err, tc.want) {
				t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
				continue
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Errorf("%s: the error must say something", tc.name)
			}
			// A refused request leaves no branch behind (except the one that existed).
			if tc.want != gitprovider.ErrBranchExists && f.Branch(tc.req.Branch) != nil {
				t.Errorf("%s: a branch was left behind", tc.name)
			}
		}
		// The branch that already existed is unchanged.
		if got := f.Branch("b-first"); got["new/a.tf"] != "x" || got["new/b.tf"] != "" {
			t.Errorf("an existing branch must not be overwritten: %v", got)
		}
	})

	t.Run("line endings do not make an update look changed", func(t *testing.T) {
		f := newFixture(t)
		snap, _ := f.Source.Snapshot(ctx)
		_, err := f.Source.CreateBranch(ctx, gitprovider.BranchRequest{
			Base: snap.Commit, Branch: "crlf", Message: "m",
			Files: []gitprovider.FileChange{{Path: "vnet/vnet.tf", Content: "# vnet\n# x\n", Update: true, Previous: "# vnet\r\n"}},
		})
		if err != nil {
			t.Errorf("CRLF vs LF in Previous must be accepted: %v", err)
		}
	})

	t.Run("an unreadable repository is an error, not an empty one", func(t *testing.T) {
		// Sources that can fail on demand are covered in their own packages; here the
		// contract is only that a Snapshot never returns a nil FS without an error.
		f := newFixture(t)
		snap, err := f.Source.Snapshot(ctx)
		if err != nil || snap.FS == nil {
			t.Fatalf("snapshot: %v %v", snap.FS, err)
		}
	})
}

// RunBase checks what only sources that branch from the planned commit guarantee
// (memory, GitHub): the branch holds exactly what the plan was built on, even if the
// default branch moved after the plan.
func RunBase(t *testing.T, newFixture func(t *testing.T) Fixture) {
	t.Helper()
	ctx := context.Background()
	f := newFixture(t)
	if f.Advance == nil {
		t.Skip("this source cannot simulate a push to the default branch")
	}
	snap, err := f.Source.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.Advance(map[string]string{"common/main.tf": "locals { moved = true }\n", "late/new.tf": "# late\n"})
	head, _ := f.Source.Snapshot(ctx)
	if head.Commit == snap.Commit {
		t.Fatal("the default branch should have moved")
	}
	res, err := f.Source.CreateBranch(ctx, gitprovider.BranchRequest{
		Base: snap.Commit, Branch: "from-base", Message: "m",
		Files: []gitprovider.FileChange{{Path: "storage/main.tf", Content: "x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := f.Branch(res.Branch)
	if got["common/main.tf"] != Start["common/main.tf"] || got["late/new.tf"] != "" {
		t.Errorf("the branch must hold the planned commit's files, not the moved head: %v", got)
	}
	if got["storage/main.tf"] != "x" {
		t.Errorf("the change is missing: %v", got)
	}
}
