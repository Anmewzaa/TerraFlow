package gitprovider

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"testing/fstest"
)

// Memory is a Source that lives in memory: no disk, no `git` binary, no network.
// It follows the same rules as the remote sources (the branch is made from the
// commit the plan was made on, nothing is overwritten), so it stands in for GitHub
// in tests and proves that the flow needs nothing from the machine.
type Memory struct {
	mu       sync.Mutex
	commits  []memCommit // the default branch, oldest first; the last is the head
	branches map[string]memBranch
	seq      int // last commit number handed out (branch commits included)

	// FailWith, when set, makes every call return this error (to test how callers
	// handle an unreachable or refused provider).
	FailWith error
}

type memCommit struct {
	id    string
	files map[string]string
}

type memBranch struct {
	commit string
	files  map[string]string
}

// NewMemory starts a repository whose first commit holds files (paths relative to
// the project folder, slash-separated).
func NewMemory(files map[string]string) *Memory {
	m := &Memory{branches: map[string]memBranch{}}
	m.commits = []memCommit{{id: m.newID(), files: copyFiles(files)}}
	return m
}

func (m *Memory) newID() string {
	m.seq++
	return fmt.Sprintf("m%d", m.seq)
}

func copyFiles(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Advance adds a commit on the default branch that sets (or, with an empty string
// value, removes) the given files, as if someone pushed to main. It returns the
// new commit.
func (m *Memory) Advance(changes map[string]string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := copyFiles(m.commits[len(m.commits)-1].files)
	for k, v := range changes {
		if v == "" {
			delete(next, k)
		} else {
			next[k] = v
		}
	}
	c := memCommit{id: m.newID(), files: next}
	m.commits = append(m.commits, c)
	return c.id
}

// Branch returns the files of a created branch, or nil if there is none.
func (m *Memory) Branch(name string) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.branches[name]
	if !ok {
		return nil
	}
	return copyFiles(b.files)
}

// Snapshot reads the head of the default branch.
func (m *Memory) Snapshot(_ context.Context) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWith != nil {
		return Snapshot{}, m.FailWith
	}
	head := m.commits[len(m.commits)-1]
	fsys := fstest.MapFS{}
	for p, c := range head.files {
		fsys[p] = &fstest.MapFile{Data: []byte(c)}
	}
	return Snapshot{Commit: head.id, FS: fs.FS(fsys)}, nil
}

// CreateBranch makes a branch from req.Base (the head if Base is empty).
func (m *Memory) CreateBranch(_ context.Context, req BranchRequest) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWith != nil {
		return Result{}, m.FailWith
	}
	if _, taken := m.branches[req.Branch]; taken {
		return Result{}, ErrBranchExists
	}
	base := m.commits[len(m.commits)-1]
	if req.Base != "" {
		found := false
		for _, c := range m.commits {
			if c.id == req.Base {
				base, found = c, true
			}
		}
		if !found {
			return Result{}, fmt.Errorf("%w: base commit %q", ErrNotFound, req.Base)
		}
	}
	files := copyFiles(base.files)
	for _, f := range req.Files {
		p, err := ValidatePath(f.Path)
		if err != nil {
			return Result{}, err
		}
		cur, exists := files[p]
		switch {
		case !f.Update && exists:
			return Result{}, fmt.Errorf("%w: %s", ErrFileExists, p)
		case f.Update && !exists:
			return Result{}, fmt.Errorf("%w: %s", ErrFileMissing, p)
		case f.Update && f.Previous != "" && normEOL(cur) != normEOL(f.Previous):
			return Result{}, fmt.Errorf("%w: %s", ErrFileChanged, p)
		}
		files[p] = f.Content
	}
	// The default branch does not move: the new commit belongs to the branch only.
	id := m.newID()
	m.branches[req.Branch] = memBranch{commit: id, files: files}
	return Result{Branch: req.Branch, Commit: id, Repo: "memory"}, nil
}

func normEOL(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// Names lists the created branches, sorted.
func (m *Memory) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.branches))
	for n := range m.branches {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
