package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// bbStore is the little of a team store the base branch endpoints use.
type bbStore struct {
	TeamStore
	mu    sync.Mutex
	repo  store.ProjectRepo
	roles map[string]string
}

func (s *bbStore) MemberRole(_ context.Context, _, oid string) (string, error) {
	return s.roles[oid], nil
}
func (s *bbStore) GetProjectRepo(context.Context, string) (store.ProjectRepo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo, nil
}
func (s *bbStore) GetConnection(context.Context, string) (store.Connection, error) {
	return store.Connection{ID: "c1", ExternalID: "7", Status: store.ConnActive}, nil
}
func (s *bbStore) SetBaseBranch(_ context.Context, _, b string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repo.BaseBranch = b
	return nil
}
func (s *bbStore) ListMembersWithRoles(context.Context, string) ([]store.RoleMember, error) {
	return nil, nil
}
func (s *bbStore) LogAudit(context.Context, string, string, string, any) error { return nil }

type bbEnv struct {
	fake *githubtest.Fake
	st   *bbStore
	app  *github.App
	h    http.Handler
}

func newBBEnv(t *testing.T) *bbEnv {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }
	e := &bbEnv{fake: githubtest.New("1", &k.PublicKey, now)}
	t.Cleanup(e.fake.Close)
	e.fake.AddRepo("alice/infra", "iac/", map[string]string{"prod/main.tf": "# main\n"})
	e.fake.AddInstallation(7, "alice", "alice/infra")
	e.fake.MakeBranch("alice/infra", "release", "main")
	e.fake.PushTo("alice/infra", "release", "iac/", map[string]string{"prod/release.tf": "# release\n"})
	e.fake.MakeBranch("alice/infra", "terraflow/p/dev-1234abcd", "main")
	e.st = &bbStore{
		repo:  store.ProjectRepo{ProjectID: "p1", ConnectionID: "c1", RepoFullName: "alice/infra", RepoPath: "iac", DefaultBranch: "main"},
		roles: map[string]string{"owner": store.RoleOwner, "co": store.RoleCoOwner, "contrib": store.RoleContributor},
	}
	e.app = &github.App{ID: "1", Key: k, APIBase: e.fake.URL, WebBase: e.fake.URL, ClientID: githubtest.ClientID, ClientSecret: githubtest.ClientSecret, Now: now}
	bb := &BaseBranchAPI{Team: &TeamAPI{Store: e.st}, App: e.app}
	r := chi.NewRouter()
	r.Get("/projects/{id}/branches", bb.List())
	r.Put("/projects/{id}/base-branch", bb.Set())
	e.h = r
	return e
}

func (e *bbEnv) do(oid, method, target string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, target, bytes.NewReader(b))
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{OID: oid, Name: oid}))
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return w
}

func (e *bbEnv) src() *github.Source {
	pr, _ := e.st.GetProjectRepo(context.Background(), "p1")
	return &github.Source{App: e.app, InstallationID: 7, Owner: "alice", Repo: "infra", Branch: pr.Branch(), Path: pr.RepoPath}
}

func TestBranchListHidesReviewBranches(t *testing.T) {
	e := newBBEnv(t)
	w := e.do("owner", "GET", "/projects/p1/branches", nil)
	if w.Code != 200 {
		t.Fatalf("list = %d %s", w.Code, w.Body)
	}
	var got struct {
		Branches []string `json:"branches"`
		Default  string   `json:"default_branch"`
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got.Branches) != 2 || got.Branches[0] != "main" || got.Branches[1] != "release" || got.Default != "main" {
		t.Fatalf("branches = %+v, want main and release without the review branch", got)
	}
}

func TestReadsAndBranchesStartFromTheChosenBase(t *testing.T) {
	e := newBBEnv(t)
	ctx := context.Background()
	if w := e.do("co", "PUT", "/projects/p1/base-branch", map[string]string{"branch": "release"}); w.Code != 200 {
		t.Fatalf("set = %d %s", w.Code, w.Body)
	}
	snap, err := e.src().Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(snap.FS, "prod/release.tf"); err != nil {
		t.Fatalf("the project must read from release: %v", err)
	}
	if _, err := e.src().CreateBranch(ctx, gitprovider.BranchRequest{Base: snap.Commit, Branch: "terraflow/p1/dev-aaaa1111", Message: "m", Files: []gitprovider.FileChange{{Path: "prod/new.tf", Content: "# n\n"}}}); err != nil {
		t.Fatal(err)
	}
	if parents := e.fake.Parents("alice/infra", e.fake.Head("alice/infra", "terraflow/p1/dev-aaaa1111")); len(parents) != 1 || parents[0] != e.fake.Head("alice/infra", "release") {
		t.Fatalf("the review branch must start from release, parents = %v", parents)
	}
	// The base itself is never written to.
	if got := e.fake.Branch("alice/infra", "iac/", "release"); got["prod/new.tf"] != "" {
		t.Fatal("the base branch must not be committed to")
	}
}

func TestBaseBranchRefusals(t *testing.T) {
	e := newBBEnv(t)
	for name, c := range map[string]struct {
		who, branch string
		want        int
	}{
		"a contributor":         {"contrib", "release", http.StatusForbidden},
		"a stranger":            {"nobody", "release", http.StatusForbidden},
		"a missing branch":      {"owner", "nope", http.StatusUnprocessableEntity},
		"a review branch":       {"owner", "terraflow/p/dev-1234abcd", http.StatusUnprocessableEntity},
		"an invalid name":       {"owner", "a..b", http.StatusUnprocessableEntity},
		"a name with a space":   {"owner", "a b", http.StatusUnprocessableEntity},
		"the owner, a real one": {"owner", "release", http.StatusOK},
	} {
		if w := e.do(c.who, "PUT", "/projects/p1/base-branch", map[string]string{"branch": c.branch}); w.Code != c.want {
			t.Errorf("%s: %d %s, want %d", name, w.Code, w.Body, c.want)
		}
	}
	if e.st.repo.BaseBranch != "release" {
		t.Fatalf("only the allowed change must be kept, base = %q", e.st.repo.BaseBranch)
	}
	// The default's own name, or an empty one, goes back to following the default.
	for _, b := range []string{"main", ""} {
		e.do("owner", "PUT", "/projects/p1/base-branch", map[string]string{"branch": "release"})
		if w := e.do("owner", "PUT", "/projects/p1/base-branch", map[string]string{"branch": b}); w.Code != 200 || e.st.repo.BaseBranch != "" {
			t.Fatalf("%q must clear the base: %d base=%q", b, w.Code, e.st.repo.BaseBranch)
		}
	}
}

func TestADeletedBaseBranchIsAnErrorNotAFallback(t *testing.T) {
	e := newBBEnv(t)
	e.do("owner", "PUT", "/projects/p1/base-branch", map[string]string{"branch": "release"})
	e.fake.DeleteBranch("alice/infra", "release")
	if _, err := e.src().Snapshot(context.Background()); err == nil {
		t.Fatal("reading a deleted base branch must fail, not quietly read the default")
	}
}
