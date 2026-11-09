package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// These tests run the review flow against an in-memory repository: no disk and no
// `git` binary, which is what a remote provider (GitHub) will look like to the API.

const rgFile = `module "rg_app" {
  source   = "git::https://example.invalid/modules//resource-group?ref=v1"
  name     = "rg-app"
  location = "southeastasia"
}
`

type memEnv struct {
	t      *testing.T
	router chi.Router
	mem    *MemChangesetStore
	repo   *gitprovider.Memory
}

func newMemEnv(t *testing.T) *memEnv {
	t.Helper()
	repo := gitprovider.NewMemory(map[string]string{"common/rg.tf": rgFile})
	mem := NewMemChangesetStore()
	rs := NewMemReviewStore(mem)
	repos := Repos(func(context.Context, store.Project) (gitprovider.Source, error) { return repo, nil })
	rv := &Reviewer{
		Store: rs, Repos: repos,
		Project: func(_ context.Context, id string) (store.Project, error) {
			return store.Project{ID: id, Slug: "demo"}, nil // no folder: the repository is the memory source
		},
	}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			u := &auth.User{OID: "reviewer-1", Roles: []string{"devops"}}
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), u)))
		})
	})
	r.Route("/projects/{id}/changesets/{csId}", func(cs chi.Router) {
		cs.Get("/review", rv.Detail())
		cs.Post("/approve", rv.Approve())
	})
	r.Get("/projects/{id}/files", ProjectFilesHandler(rv.Project, repos))
	r.Get("/projects/{id}/import", ProjectImportHandler(rv.Project, repos))
	return &memEnv{t: t, router: r, mem: mem, repo: repo}
}

func (e *memEnv) seed(id string) {
	canvas, _ := json.Marshal(store.CanvasState{Nodes: []store.CanvasNode{{
		ID: "n1", ModuleName: "storage-account", ModuleVersion: "v1", InstanceName: "sa_demo",
		TargetFolder: "storage", Values: map[string]interface{}{"name": "stdemo01", "account_tier": "Standard"},
	}}})
	e.mem.data[id] = store.Changeset{ID: id, ProjectID: "p1", Env: "dev", UserOID: "dev-1", Title: "Add storage", Status: store.ChangesetPublished, CanvasJSON: canvas}
}

func (e *memEnv) do(method, path string) (*httptest.ResponseRecorder, map[string]any) {
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewBufferString("")))
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestApproveOnAnInMemoryRepository(t *testing.T) {
	e := newMemEnv(t)
	e.seed("abcdef12-0000")

	rec, out := e.do("POST", "/projects/p1/changesets/abcdef12-0000/approve")
	if rec.Code != 200 || out["status"] != "approved" || out["branch"] != "terraflow/demo/dev-abcdef12" {
		t.Fatalf("approve: %d %v", rec.Code, out)
	}
	got := e.repo.Branch("terraflow/demo/dev-abcdef12")
	if !strings.Contains(got["storage/main.tf"], `module "sa_demo"`) {
		t.Fatalf("branch content: %v", got)
	}
	if got["common/rg.tf"] != rgFile {
		t.Errorf("an untouched file changed: %q", got["common/rg.tf"])
	}
	cs := e.mem.data["abcdef12-0000"]
	if cs.Status != store.ChangesetApproved || cs.BranchName == nil || cs.CommitSHA == nil || *cs.CommitSHA == "" {
		t.Fatalf("not recorded: %+v", cs)
	}
}

func TestGraphFilesAndImportReadTheInMemoryRepository(t *testing.T) {
	e := newMemEnv(t)

	rec, out := e.do("GET", "/projects/p1/files")
	files, _ := out["repo_files"].([]any)
	if rec.Code != 200 || len(files) != 1 {
		t.Fatalf("files: %d %v", rec.Code, out)
	}
	rec, out = e.do("GET", "/projects/p1/import")
	nodes, _ := out["nodes"].([]any)
	if rec.Code != 200 || len(nodes) != 1 {
		t.Fatalf("import: %d %v", rec.Code, out)
	}
	n := nodes[0].(map[string]any)
	if n["instance_name"] != "rg_app" || n["target_folder"] != "common" {
		t.Errorf("imported node: %v", n)
	}
}

// A provider that fails must stop the plan. Planning against an empty repository
// would generate every resource again and a branch would duplicate existing code.
func TestProviderFailuresStopTheReviewAndAreNotAnEmptyRepository(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"credential refused", gitprovider.ErrUnauthorized, http.StatusConflict},
		{"repository not found", gitprovider.ErrNotFound, http.StatusNotFound},
		{"rate limited", gitprovider.ErrRateLimited, http.StatusTooManyRequests},
		{"anything else", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newMemEnv(t)
			e.seed("c1")
			e.repo.FailWith = tc.err

			if rec, _ := e.do("GET", "/projects/p1/changesets/c1/review"); rec.Code != tc.want {
				t.Errorf("review: %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			rec, _ := e.do("POST", "/projects/p1/changesets/c1/approve")
			if rec.Code != tc.want {
				t.Errorf("approve: %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			if got := e.mem.data["c1"].Status; got != store.ChangesetPublished {
				t.Errorf("a failed approve must leave the changeset waiting, got %s", got)
			}
			if names := e.repo.Names(); len(names) != 0 {
				t.Errorf("a branch was created: %v", names)
			}
		})
	}
}

func TestBranchNameTakenIsAConflictAndNothingIsApproved(t *testing.T) {
	e := newMemEnv(t)
	e.seed("abcdef12-0000")
	if _, err := e.repo.CreateBranch(context.Background(), gitprovider.BranchRequest{
		Branch: "terraflow/demo/dev-abcdef12", Message: "m", Files: []gitprovider.FileChange{{Path: "x/y.tf", Content: "x"}},
	}); err != nil {
		t.Fatal(err)
	}
	rec, _ := e.do("POST", "/projects/p1/changesets/abcdef12-0000/approve")
	if rec.Code != http.StatusConflict {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body)
	}
	if got := e.mem.data["abcdef12-0000"].Status; got != store.ChangesetPublished {
		t.Errorf("status %s", got)
	}
	// The branch that was already there was not touched.
	if b := e.repo.Branch("terraflow/demo/dev-abcdef12"); b["storage/main.tf"] != "" {
		t.Errorf("an existing branch was overwritten: %v", b)
	}
}

func TestProviderStatusMapping(t *testing.T) {
	for err, want := range map[error]int{
		gitprovider.ErrNotFound:     404,
		gitprovider.ErrUnauthorized: 409,
		gitprovider.ErrRateLimited:  429,
		gitprovider.ErrBranchExists: 409,
		gitprovider.ErrFileExists:   409,
		gitprovider.ErrFileMissing:  409,
		gitprovider.ErrFileChanged:  409,
		gitprovider.ErrUnsafePath:   422,
		errors.New("other"):         500,
	} {
		if got := providerStatus(err); got != want {
			t.Errorf("%v: %d, want %d", err, got, want)
		}
	}
}
