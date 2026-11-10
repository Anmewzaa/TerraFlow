package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// memGH is an in-memory GitHubStore.
type memGH struct {
	mu       sync.Mutex
	conns    map[string]store.Connection
	grants   map[string]store.RepoGrant // user|conn|repo
	projects map[string]store.Project
	owners   map[string]string
	repos    map[string]store.ProjectRepo
	audit    []string
	now      func() time.Time
	seq      int
}

func newMemGH(now func() time.Time) *memGH {
	return &memGH{conns: map[string]store.Connection{}, grants: map[string]store.RepoGrant{}, projects: map[string]store.Project{},
		owners: map[string]string{}, repos: map[string]store.ProjectRepo{}, now: now}
}

func (m *memGH) UpsertConnection(_ context.Context, c store.Connection) (store.Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.conns {
		if e.Provider == c.Provider && e.ExternalID == c.ExternalID {
			e.AccountLogin, e.Status = c.AccountLogin, store.ConnActive
			m.conns[id] = e
			return e, nil
		}
	}
	m.seq++
	c.ID, c.Status = "c"+strconv.Itoa(m.seq), store.ConnActive
	m.conns[c.ID] = c
	return c, nil
}
func (m *memGH) GetConnection(_ context.Context, id string) (store.Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[id]
	if !ok {
		return store.Connection{}, pgx.ErrNoRows
	}
	return c, nil
}
func (m *memGH) ListConnections(context.Context) ([]store.Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.Connection
	for _, c := range m.conns {
		out = append(out, c)
	}
	return out, nil
}
func (m *memGH) SetConnectionStatus(_ context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[id]
	if !ok {
		return pgx.ErrNoRows
	}
	c.Status = status
	m.conns[id] = c
	return nil
}
func (m *memGH) ProjectsOfConnection(_ context.Context, id string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.repos {
		if r.ConnectionID == id {
			n++
		}
	}
	return n, nil
}
func (m *memGH) ReplaceGrants(_ context.Context, user, conn string, gs []store.RepoGrant) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, g := range m.grants {
		if strings.HasPrefix(k, user+"|"+conn+"|") {
			delete(m.grants, k)
			_ = g
		}
	}
	for _, g := range gs {
		g.ConnectionID, g.UserOID = conn, user
		m.grants[user+"|"+conn+"|"+g.RepoExternalID] = g
	}
	return nil
}
func (m *memGH) ListGrants(_ context.Context, user string) ([]store.RepoGrant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.RepoGrant
	for _, g := range m.grants {
		if g.UserOID == user && g.ExpiresAt.After(m.now()) {
			out = append(out, g)
		}
	}
	return out, nil
}
func (m *memGH) GetGrant(_ context.Context, user, conn, repo string) (store.RepoGrant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.grants[user+"|"+conn+"|"+repo]
	if !ok || !g.ExpiresAt.After(m.now()) {
		return store.RepoGrant{}, pgx.ErrNoRows
	}
	return g, nil
}
func (m *memGH) DeleteGrant(_ context.Context, user, conn, repo string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.grants, user+"|"+conn+"|"+repo)
	return nil
}
func (m *memGH) CreateProject(_ context.Context, p store.Project) (store.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.projects {
		if e.Slug == p.Slug {
			return store.Project{}, store.ErrSlugTaken
		}
	}
	m.seq++
	p.ID = "p" + strconv.Itoa(m.seq)
	m.projects[p.ID] = p
	return p, nil
}
func (m *memGH) DeleteProject(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.projects, id)
	delete(m.owners, id)
	delete(m.repos, id)
	return nil
}
func (m *memGH) CreateOwner(_ context.Context, id, oid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.owners[id] = oid
	return nil
}
func (m *memGH) SetProjectRepo(_ context.Context, r store.ProjectRepo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repos[r.ProjectID] = r
	return nil
}
func (m *memGH) LogAudit(_ context.Context, _, action, _ string, _ any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = append(m.audit, action)
	return nil
}

type ghEnv struct {
	fake  *githubtest.Fake
	st    *memGH
	api   *GitHubAPI
	clock time.Time
	h     http.Handler
}

func newGHEnv(t *testing.T, allowed ...string) *ghEnv {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e := &ghEnv{clock: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}
	now := func() time.Time { return e.clock }
	e.fake = githubtest.New("1", &k.PublicKey, now)
	t.Cleanup(e.fake.Close)
	e.fake.AddRepo("alice/infra", "iac/", map[string]string{"prod/main.tf": "# x\n", "README.md": "hi"})
	e.fake.AddRepo("alice/docs", "", map[string]string{"README.md": "no terraform"})
	e.fake.AddRepo("alice/secret", "", map[string]string{"main.tf": ""})
	e.fake.AddInstallation(7, "alice", "alice/infra", "alice/docs", "alice/secret")
	e.st = newMemGH(now)
	e.api = &GitHubAPI{
		App:   &github.App{ID: "1", Key: k, APIBase: e.fake.URL, WebBase: e.fake.URL, ClientID: githubtest.ClientID, ClientSecret: githubtest.ClientSecret, Now: now},
		Store: e.st, AllowedOwners: allowed, Now: now,
	}
	r := chi.NewRouter()
	r.Get("/api/github/authorize", e.api.Authorize())
	r.Get("/api/github/callback", e.api.Callback())
	r.Get("/api/github/repos", e.api.Repos())
	r.Get("/api/github/connections", e.api.MyConnections())
	r.Post("/api/github/connections/{id}/disconnect", e.api.DisconnectMine())
	r.Get("/api/github/folders", e.api.Folders())
	r.Post("/api/projects/from-repo", e.api.CreateFromRepo())
	r.Post("/api/github/init", e.api.InitRepo())
	e.h = r
	return e
}

func (e *ghEnv) do(oid, method, target string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, rd)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{OID: oid, Name: oid}))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return w
}

// authorize runs the whole browser round trip for a GitHub user and returns the callback answer.
func (e *ghEnv) authorize(t *testing.T, oid, code string) *httptest.ResponseRecorder {
	t.Helper()
	w := e.do(oid, "GET", "/api/github/authorize", nil)
	if w.Code != http.StatusFound {
		t.Fatalf("authorize = %d", w.Code)
	}
	var state string
	for _, c := range w.Result().Cookies() {
		if c.Name == stateCookie {
			state = c.Value
		}
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	if loc.Query().Get("state") != state || state == "" {
		t.Fatalf("the redirect must carry the same state as the cookie")
	}
	return e.do(oid, "GET", "/api/github/callback?code="+code+"&state="+state, nil, &http.Cookie{Name: stateCookie, Value: state})
}

func listRepos(t *testing.T, e *ghEnv, oid string) []repoView {
	t.Helper()
	w := e.do(oid, "GET", "/api/github/repos", nil)
	var out []repoView
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%v: %s", err, w.Body)
	}
	return out
}

func TestRepoPickerListsOnlyRepositoriesTheUserCanReach(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/docs") // not alice/secret
	w := e.authorize(t, "u1", code)
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "connected=1") {
		t.Fatalf("callback = %d %s", w.Code, w.Header().Get("Location"))
	}
	got := listRepos(t, e, "u1")
	if len(got) != 2 {
		t.Fatalf("repos = %+v, want infra and docs only", got)
	}
	for _, r := range got {
		if r.FullName == "alice/secret" {
			t.Fatal("a repository the user cannot reach must not be offered")
		}
	}
	if n := len(listRepos(t, e, "someone-else")); n != 0 {
		t.Fatalf("another TerraFlow user sees %d repositories; grants are per user", n)
	}
}

func TestCallbackRefusesAWrongOrMissingState(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra")
	w := e.do("u1", "GET", "/api/github/callback?code="+code+"&state=forged", nil, &http.Cookie{Name: stateCookie, Value: "real"})
	if !strings.Contains(w.Header().Get("Location"), "error=state") {
		t.Fatalf("a forged state must be refused, got %s", w.Header().Get("Location"))
	}
	w = e.do("u1", "GET", "/api/github/callback?code="+code+"&state=x", nil)
	if !strings.Contains(w.Header().Get("Location"), "error=state") {
		t.Fatalf("no state cookie must be refused, got %s", w.Header().Get("Location"))
	}
	if len(listRepos(t, e, "u1")) != 0 {
		t.Fatal("nothing may be granted after a refused callback")
	}
}

func TestAllowlistBlocksOtherAccounts(t *testing.T) {
	e := newGHEnv(t, "someone-else")
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra")
	w := e.authorize(t, "u1", code)
	if !strings.Contains(w.Header().Get("Location"), "error=not_allowed") {
		t.Fatalf("location = %s", w.Header().Get("Location"))
	}
	if len(e.st.conns) != 0 {
		t.Fatal("a blocked account must not become a connection")
	}
}

func TestNoInstallationIsAClearError(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(43, "nobody", nil)
	w := e.authorize(t, "u1", code)
	if !strings.Contains(w.Header().Get("Location"), "error=no_installation") {
		t.Fatalf("location = %s", w.Header().Get("Location"))
	}
}

func createBody(e *ghEnv, oid, name, repoName, folder string) map[string]string {
	for _, r := range listRepos2(e, oid) {
		if r.FullName == repoName {
			return map[string]string{"name": name, "connection_id": r.ConnectionID, "repo_id": r.RepoID, "folder": folder}
		}
	}
	return map[string]string{"name": name, "connection_id": "c1", "repo_id": "999", "folder": folder}
}

func listRepos2(e *ghEnv, oid string) []repoView {
	w := e.do(oid, "GET", "/api/github/repos", nil)
	var out []repoView
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func TestCreateProjectFromARepositoryMakesTheCallerTheOwner(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/docs")
	e.authorize(t, "u1", code)

	w := e.do("u1", "POST", "/api/projects/from-repo", createBody(e, "u1", "Platform", "alice/infra", "iac"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	var p store.Project
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.SourceType != "github" || e.st.owners[p.ID] != "u1" {
		t.Fatalf("project %+v owner %q", p, e.st.owners[p.ID])
	}
	pr := e.st.repos[p.ID]
	if pr.RepoFullName != "alice/infra" || pr.RepoPath != "iac" || pr.ConnectedBy != "u1" || pr.DefaultBranch != "main" {
		t.Fatalf("project repo = %+v", pr)
	}
	// The grant is used up: creating a second project needs a fresh authorization.
	w = e.do("u1", "POST", "/api/projects/from-repo", createBody(e, "u1", "Again", "alice/infra", "iac"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("second create without authorizing again = %d, want 403", w.Code)
	}
}

func TestCannotCreateFromARepositoryWithoutAGrant(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra")
	e.authorize(t, "u1", code)
	conn := listRepos(t, e, "u1")[0].ConnectionID

	// alice/secret is in the installation but the user cannot reach it: no grant exists.
	body := map[string]string{"name": "Sneaky", "connection_id": conn, "repo_id": "1001", "folder": ""}
	if w := e.do("u1", "POST", "/api/projects/from-repo", body); w.Code != http.StatusForbidden {
		t.Fatalf("no grant = %d, want 403", w.Code)
	}
	// Another TerraFlow user cannot use u1's grant.
	good := createBody(e, "u1", "Theirs", "alice/infra", "iac")
	if w := e.do("u2", "POST", "/api/projects/from-repo", good); w.Code != http.StatusForbidden {
		t.Fatalf("someone else's grant = %d, want 403", w.Code)
	}
	if len(e.st.projects) != 0 {
		t.Fatal("no project may be created without a grant")
	}
}

func TestAnExpiredGrantNeedsAuthorizingAgain(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra")
	e.authorize(t, "u1", code)
	body := createBody(e, "u1", "Late", "alice/infra", "iac")
	e.clock = e.clock.Add(11 * time.Minute)
	if w := e.do("u1", "POST", "/api/projects/from-repo", body); w.Code != http.StatusForbidden {
		t.Fatalf("expired = %d, want 403", w.Code)
	}
	if n := len(listRepos(t, e, "u1")); n != 0 {
		t.Fatalf("an expired grant is still listed (%d)", n)
	}
}

func TestFolderMustExistHoldTerraformAndBeASafePath(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/docs")
	e.authorize(t, "u1", code)
	// Every refusal leaves the grant in place (it is only used up by a success), so the
	// cases run one after another on the same authorization.
	for _, c := range []struct {
		name, repo, folder string
	}{
		{"missing folder", "alice/infra", "nope"},
		{"no terraform", "alice/docs", ""},
		{"parent traversal", "alice/infra", "../etc"},
		{"backslash", "alice/infra", `iac\prod`},
	} {
		w := e.do("u1", "POST", "/api/projects/from-repo", createBody(e, "u1", "P "+c.name, c.repo, c.folder))
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s = %d %s, want 422", c.name, w.Code, w.Body)
		}
	}
	if len(e.st.projects) != 0 {
		t.Fatal("a refused folder must not leave a project behind")
	}
}

func TestFolderPickerListsTerraformFolders(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra")
	e.authorize(t, "u1", code)
	r := listRepos(t, e, "u1")[0]
	w := e.do("u1", "GET", "/api/github/folders?connection_id="+r.ConnectionID+"&repo_id="+r.RepoID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("folders = %d %s", w.Code, w.Body)
	}
	var out struct {
		Folders []string `json:"folders"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	want := map[string]bool{"iac": true, "iac/prod": true}
	for _, f := range out.Folders {
		delete(want, f)
	}
	if len(want) != 0 {
		t.Fatalf("folders = %v, missing %v", out.Folders, want)
	}
	if w := e.do("u2", "GET", "/api/github/folders?connection_id="+r.ConnectionID+"&repo_id="+r.RepoID, nil); w.Code != http.StatusForbidden {
		t.Fatalf("another user's folders = %d, want 403", w.Code)
	}
}

func TestADisconnectedConnectionCannotCreateProjects(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra")
	e.authorize(t, "u1", code)
	body := createBody(e, "u1", "Off", "alice/infra", "iac")
	e.st.SetConnectionStatus(context.Background(), body["connection_id"], store.ConnDisconnected)
	if w := e.do("u1", "POST", "/api/projects/from-repo", body); w.Code != http.StatusConflict {
		t.Fatalf("disconnected = %d, want 409", w.Code)
	}
	if n := len(listRepos(t, e, "u1")); n != 0 {
		t.Fatalf("a disconnected connection's repositories are still offered (%d)", n)
	}
}

func TestNotConfiguredSaysSo(t *testing.T) {
	g := &GitHubAPI{}
	r := chi.NewRouter()
	r.Get("/a", g.Authorize())
	r.Get("/s", g.Status())
	if w := httptest.NewRecorder(); true {
		r.ServeHTTP(w, httptest.NewRequest("GET", "/a", nil))
		if w.Code != http.StatusConflict {
			t.Fatalf("authorize without an App = %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/s", nil))
	if !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("status = %s", w.Body)
	}
}

func TestMyConnectionsShowOnlyMineAndDisconnectIsLimitedToTheOwner(t *testing.T) {
	e := newGHEnv(t)
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/docs")
	e.authorize(t, "u1", code)

	var mine []myConnectionView
	w := e.do("u1", "GET", "/api/github/connections", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &mine); err != nil || len(mine) != 1 {
		t.Fatalf("mine = %v %s", err, w.Body)
	}
	if mine[0].Account != "alice" || mine[0].Repositories != 2 || mine[0].ExpiresAt == "" || mine[0].Projects != 0 {
		t.Fatalf("view = %+v", mine[0])
	}
	// Someone else sees none of it and cannot disconnect it (the answer does not reveal it exists).
	w = e.do("u2", "GET", "/api/github/connections", nil)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("u2 sees %s", w.Body)
	}
	if w = e.do("u2", "POST", "/api/github/connections/"+mine[0].ID+"/disconnect", nil); w.Code != http.StatusNotFound {
		t.Fatalf("other user disconnect = %d, want 404", w.Code)
	}
	if w = e.do("u1", "POST", "/api/github/connections/"+mine[0].ID+"/disconnect", nil); w.Code != http.StatusOK {
		t.Fatalf("owner disconnect = %d %s", w.Code, w.Body)
	}
	w = e.do("u1", "GET", "/api/github/connections", nil)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("a disconnected account is still listed: %s", w.Body)
	}
}

func TestNewFolderStartsAProjectWhereNoTerraformExistsYet(t *testing.T) {
	e := newGHEnv(t)
	e.fake.AddEmptyRepo("alice/blank")
	e.fake.AddInstallation(7, "alice", "alice/infra", "alice/docs", "alice/secret", "alice/blank")
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/docs", "alice/blank")
	e.authorize(t, "u1", code)
	body := func(name, repo, folder string, newFolder bool) map[string]any {
		b := createBody(e, "u1", name, repo, folder)
		return map[string]any{"name": b["name"], "connection_id": b["connection_id"], "repo_id": b["repo_id"], "folder": folder, "new_folder": newFolder}
	}
	for _, c := range []struct {
		name, repo, folder string
		want               int
	}{
		{"has terraform already", "alice/infra", "iac", http.StatusUnprocessableEntity},
		{"no folder named", "alice/docs", "", http.StatusUnprocessableEntity},
		{"unsafe path", "alice/docs", "../x", http.StatusUnprocessableEntity},
		{"empty repository", "alice/blank", "iac", http.StatusUnprocessableEntity},
	} {
		if w := e.do("u1", "POST", "/api/projects/from-repo", body("P "+c.name, c.repo, c.folder, true)); w.Code != c.want {
			t.Fatalf("%s = %d %s, want %d", c.name, w.Code, w.Body, c.want)
		}
	}
	if len(e.st.projects) != 0 {
		t.Fatal("a refused new folder must not leave a project behind")
	}
	w := e.do("u1", "POST", "/api/projects/from-repo", body("Fresh", "alice/docs", "terraform/dev", true))
	if w.Code != http.StatusCreated {
		t.Fatalf("new folder = %d %s", w.Code, w.Body)
	}
	var p store.Project
	json.Unmarshal(w.Body.Bytes(), &p)
	if e.st.repos[p.ID].RepoPath != "terraform/dev" {
		t.Fatalf("repo path = %q", e.st.repos[p.ID].RepoPath)
	}
}

func TestInitializingAnEmptyRepositoryNeedsAGrantAndIsAuditedAndRefusedWhenNotEmpty(t *testing.T) {
	e := newGHEnv(t)
	e.fake.AddEmptyRepo("alice/blank")
	e.fake.AddInstallation(7, "alice", "alice/infra", "alice/docs", "alice/secret", "alice/blank")
	code := e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/blank")
	e.authorize(t, "u1", code)
	body := func(repo string) map[string]string {
		b := createBody(e, "u1", "x", repo, "")
		return map[string]string{"connection_id": b["connection_id"], "repo_id": b["repo_id"]}
	}
	// The picker says what is wrong with an empty repository, with a code the screen acts on.
	b := body("alice/blank")
	w := e.do("u1", "GET", "/api/github/folders?connection_id="+b["connection_id"]+"&repo_id="+b["repo_id"], nil)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"code":"empty_repo"`) {
		t.Fatalf("folders of an empty repository: %d %s", w.Code, w.Body)
	}
	// Someone without a grant for the repository cannot initialize it.
	if w := e.do("u2", "POST", "/api/github/init", b); w.Code != http.StatusForbidden {
		t.Fatalf("no grant: %d, want 403", w.Code)
	}
	if w := e.do("u1", "POST", "/api/github/init", body("alice/infra")); w.Code != http.StatusConflict {
		t.Fatalf("repository with commits: %d %s, want 409", w.Code, w.Body)
	}
	if w := e.do("u1", "POST", "/api/github/init", b); w.Code != http.StatusCreated {
		t.Fatalf("initialize: %d %s", w.Code, w.Body)
	}
	if got := e.st.audit[len(e.st.audit)-1]; got != "repo.initialize" {
		t.Fatalf("audit = %v", e.st.audit)
	}
	// Now the folders can be read (there is no Terraform yet: the screen offers a new folder).
	w = e.do("u1", "GET", "/api/github/folders?connection_id="+b["connection_id"]+"&repo_id="+b["repo_id"], nil)
	if w.Code != http.StatusOK {
		t.Fatalf("folders after initialize: %d %s", w.Code, w.Body)
	}
}
