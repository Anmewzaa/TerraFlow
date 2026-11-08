package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// fakeAccess is an in-memory AccessStore with the same error behaviour as the
// Postgres implementation.
type fakeAccess struct {
	mu        sync.Mutex
	projects  map[string]store.Project
	users     map[string]store.User
	members   map[string]map[string]bool // project -> oid
	pending   map[string]int
	audit     []auditRow
	next      int
	upserts   int
	failUsers bool
}

type auditRow struct {
	User, Action, Resource string
	Detail                 map[string]any
}

func newFakeAccess() *fakeAccess {
	return &fakeAccess{
		projects: map[string]store.Project{}, users: map[string]store.User{},
		members: map[string]map[string]bool{}, pending: map[string]int{},
	}
}

func (f *fakeAccess) addProject(name string) store.Project {
	f.next++
	p := store.Project{ID: "p" + strconv.Itoa(f.next), Name: name, Slug: slugify(name), IACPath: "/x/" + slugify(name)}
	f.projects[p.ID] = p
	return p
}

func (f *fakeAccess) UpsertUser(_ context.Context, u store.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts++
	if f.failUsers {
		return errors.New("db down")
	}
	f.users[u.OID] = u
	return nil
}

func (f *fakeAccess) ListUsers(context.Context) ([]store.UserSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.UserSummary{}
	for _, u := range f.users {
		n := 0
		for _, m := range f.members {
			if m[u.OID] {
				n++
			}
		}
		out = append(out, store.UserSummary{User: u, ProjectCount: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OID < out[j].OID })
	return out, nil
}

func (f *fakeAccess) ListProjects(context.Context) ([]store.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.Project
	for _, p := range f.projects {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *fakeAccess) ListProjectsForUser(_ context.Context, oid string) ([]store.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.Project
	for id, p := range f.projects {
		if f.members[id][oid] {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *fakeAccess) GetProject(_ context.Context, id string) (store.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.projects[id]
	if !ok {
		return store.Project{}, pgx.ErrNoRows
	}
	return p, nil
}

func (f *fakeAccess) CreateProject(_ context.Context, p store.Project) (store.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.projects {
		if e.Slug == p.Slug {
			return store.Project{}, store.ErrSlugTaken
		}
	}
	f.next++
	p.ID = "p" + strconv.Itoa(f.next)
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeAccess) UpdateProject(_ context.Context, id string, patch store.ProjectPatch) (store.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.projects[id]
	if !ok {
		return store.Project{}, pgx.ErrNoRows
	}
	if patch.Name != nil {
		p.Name = *patch.Name
	}
	if patch.Description != nil {
		p.Description = *patch.Description
	}
	if patch.IACPath != nil {
		p.IACPath = *patch.IACPath
	}
	f.projects[id] = p
	return p, nil
}

func (f *fakeAccess) DeleteProject(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.projects[id]; !ok {
		return pgx.ErrNoRows
	}
	delete(f.projects, id)
	delete(f.members, id)
	return nil
}

func (f *fakeAccess) CountPendingChangesets(_ context.Context, id string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pending[id], nil
}

func (f *fakeAccess) IsMember(_ context.Context, projectID, oid string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.members[projectID][oid], nil
}

func (f *fakeAccess) ListMembers(_ context.Context, id string) ([]store.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.Member{}
	for oid := range f.members[id] {
		u := f.users[oid]
		out = append(out, store.Member{UserOID: oid, Email: u.Email, Name: u.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserOID < out[j].UserOID })
	return out, nil
}

func (f *fakeAccess) SetMembers(_ context.Context, id string, oids []string, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.projects[id]; !ok {
		return pgx.ErrNoRows
	}
	for _, oid := range oids {
		if _, ok := f.users[oid]; !ok {
			return store.UnknownUserError{OID: oid}
		}
	}
	m := map[string]bool{}
	for _, oid := range oids {
		m[oid] = true
	}
	f.members[id] = m
	return nil
}

func (f *fakeAccess) LogAudit(_ context.Context, user, action, resource string, detail any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, _ := detail.(map[string]any)
	f.audit = append(f.audit, auditRow{user, action, resource, d})
	return nil
}

func (f *fakeAccess) actions() []string {
	var out []string
	for _, a := range f.audit {
		out = append(out, a.Action)
	}
	return out
}

// --- harness ----------------------------------------------------------------

type adminEnv struct {
	t         *testing.T
	fs        *fakeAccess
	router    chi.Router
	workspace string
	samples   string
}

// newAdminEnv wires the same middleware and handlers as NewRouter does for the
// database mode. The caller picks the user per request with as().
func newAdminEnv(t *testing.T) *adminEnv {
	t.Helper()
	root := t.TempDir()
	e := &adminEnv{t: t, fs: newFakeAccess(), workspace: filepath.Join(root, "ws"), samples: filepath.Join(root, "samples", "AIC")}
	os.MkdirAll(e.workspace, 0o755)
	os.MkdirAll(e.samples, 0o755)

	adm := &AdminAPI{Store: e.fs, WorkspaceDir: e.workspace, SampleDir: e.samples}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if oid := req.Header.Get("X-User"); oid != "" {
				u := &auth.User{OID: oid, Name: "name-" + oid, Email: oid + "@x.test"}
				if req.Header.Get("X-Role") == "admin" {
					u.Roles = []string{"devops"}
				} else {
					u.Roles = []string{"developer"}
				}
				req = req.WithContext(auth.WithUser(req.Context(), u))
			}
			next.ServeHTTP(w, req)
		})
	})
	r.Use(UserSync(e.fs, nil))
	r.Get("/projects", adm.List())
	admin := r.With(RequireAdmin)
	admin.Post("/projects", adm.Create())
	admin.Patch("/projects/{id}", adm.Update())
	admin.Delete("/projects/{id}", adm.Delete())
	admin.Get("/projects/{id}/members", adm.Members())
	admin.Put("/projects/{id}/members", adm.SetMembers())
	admin.Get("/admin/users", adm.Users())
	guard := r.With(ProjectGuard(e.fs))
	guard.Get("/projects/{id}/files", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]any{"ok": true}) })
	e.router = r
	return e
}

type call struct {
	e    *adminEnv
	user string
	role string
}

func (e *adminEnv) as(user, role string) call { return call{e, user, role} }

func (c call) do(method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		json.NewEncoder(&buf).Encode(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	if c.user != "" {
		req.Header.Set("X-User", c.user)
		req.Header.Set("X-Role", c.role)
	}
	rec := httptest.NewRecorder()
	c.e.router.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func (c call) list(path string) []map[string]any {
	rec, _ := c.do("GET", path, nil)
	var out []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func names(list []map[string]any) []string {
	out := []string{}
	for _, m := range list {
		out = append(out, m["name"].(string))
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// --- who sees which projects --------------------------------------------------

func TestProjectListFollowsMembershipAndRole(t *testing.T) {
	e := newAdminEnv(t)
	a, b := e.fs.addProject("Alpha"), e.fs.addProject("Beta")
	e.fs.addProject("Gamma")
	for _, u := range []string{"alice", "bob", "root"} {
		e.as(u, "user").do("GET", "/projects", nil) // first request records the user
	}
	e.fs.members[a.ID] = map[string]bool{"alice": true}
	e.fs.members[b.ID] = map[string]bool{"alice": true, "bob": true}

	eq(t, names(e.as("alice", "user").list("/projects")), []string{"Alpha", "Beta"})
	eq(t, names(e.as("bob", "user").list("/projects")), []string{"Beta"})
	eq(t, names(e.as("root", "admin").list("/projects")), []string{"Alpha", "Beta", "Gamma"})
	eq(t, names(e.as("nobody", "user").list("/projects")), []string{})
	// A user with no assignment gets an empty list, never null and never everything.
	rec, _ := e.as("nobody", "user").do("GET", "/projects", nil)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list must be [], got %q", rec.Body.String())
	}
}

func TestProjectGuard(t *testing.T) {
	e := newAdminEnv(t)
	p := e.fs.addProject("Alpha")
	e.fs.users["alice"] = store.User{OID: "alice"}
	e.fs.members[p.ID] = map[string]bool{"alice": true}
	path := "/projects/" + p.ID + "/files"

	if rec, _ := e.as("alice", "user").do("GET", path, nil); rec.Code != 200 {
		t.Fatalf("member: %d", rec.Code)
	}
	if rec, _ := e.as("root", "admin").do("GET", path, nil); rec.Code != 200 {
		t.Fatalf("admin: %d", rec.Code)
	}
	if rec, _ := e.as("mallory", "user").do("GET", path, nil); rec.Code != 404 {
		t.Fatalf("non-member must get 404 (not 403, to hide existence): %d", rec.Code)
	}
	if rec, _ := e.as("mallory", "user").do("GET", "/projects/nope/files", nil); rec.Code != 404 {
		t.Fatalf("unknown project: %d", rec.Code)
	}
	if rec, _ := e.as("", "").do("GET", path, nil); rec.Code != 401 {
		t.Fatalf("anonymous: %d", rec.Code)
	}
}

// --- create ---------------------------------------------------------------------

func TestCreateProject(t *testing.T) {
	e := newAdminEnv(t)
	admin := e.as("root", "admin")

	rec, out := admin.do("POST", "/projects", map[string]string{"name": "  New Project ", "description": " hi "})
	if rec.Code != 201 || out["slug"] != "new-project" || out["description"] != "hi" {
		t.Fatalf("create: %d %v", rec.Code, out)
	}
	dir := out["iac_path"].(string)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() || !strings.HasPrefix(dir, e.workspace) {
		t.Fatalf("default folder not created inside the workspace: %q %v", dir, err)
	}
	if got := e.fs.actions(); len(got) != 1 || got[0] != "project.create" {
		t.Fatalf("audit: %v", got)
	}

	if rec, _ := admin.do("POST", "/projects", map[string]string{"name": "new project"}); rec.Code != 409 {
		t.Fatalf("same slug: %d", rec.Code)
	}
	for _, body := range []any{
		map[string]string{"name": ""}, map[string]string{"name": "!!!"},
		map[string]string{"name": strings.Repeat("a", 81)},
		map[string]string{"name": "ok", "description": strings.Repeat("d", 501)},
		map[string]string{"name": "ok", "iac_path": "/etc"},
		map[string]string{"name": "ok", "iac_path": filepath.Join(e.workspace, "missing")},
		map[string]string{"name": "ok", "iac_path": filepath.Join(e.workspace, "..", "..")},
		`{"name":"ok","unknown_field":1}`, `not json`,
	} {
		if rec, _ := admin.do("POST", "/projects", body); rec.Code != 400 {
			t.Errorf("%v: got %d, want 400", body, rec.Code)
		}
	}

	// An explicit existing folder inside the workspace or beside the samples is allowed.
	existing := filepath.Join(e.workspace, "AIH")
	os.MkdirAll(existing, 0o755)
	if rec, out := admin.do("POST", "/projects", map[string]string{"name": "AIH", "iac_path": existing}); rec.Code != 201 || out["iac_path"] != existing {
		t.Fatalf("explicit path: %d %v", rec.Code, out)
	}
	beside := filepath.Join(filepath.Dir(e.samples), "AIH")
	os.MkdirAll(beside, 0o755)
	if rec, _ := admin.do("POST", "/projects", map[string]string{"name": "Beside", "iac_path": beside}); rec.Code != 201 {
		t.Fatalf("path beside samples: %d", rec.Code)
	}
}

func TestOnlyAdminsCanChangeProjects(t *testing.T) {
	e := newAdminEnv(t)
	p := e.fs.addProject("Alpha")
	e.fs.users["alice"] = store.User{OID: "alice"}
	e.fs.members[p.ID] = map[string]bool{"alice": true}
	user := e.as("alice", "user")
	id := "/projects/" + p.ID

	for _, tc := range []struct{ method, path string }{
		{"POST", "/projects"}, {"PATCH", id}, {"DELETE", id},
		{"GET", id + "/members"}, {"PUT", id + "/members"}, {"GET", "/admin/users"},
	} {
		if rec, _ := user.do(tc.method, tc.path, `{"name":"x","user_oids":[]}`); rec.Code != 403 {
			t.Errorf("%s %s as a user: got %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
	if len(e.fs.projects) != 1 || len(e.fs.audit) != 0 {
		t.Fatal("a rejected request must change nothing")
	}
}

// --- update ---------------------------------------------------------------------

func TestUpdateProject(t *testing.T) {
	e := newAdminEnv(t)
	p := e.fs.addProject("Alpha")
	admin := e.as("root", "admin")
	id := "/projects/" + p.ID

	rec, out := admin.do("PATCH", id, map[string]string{"name": "Alpha Renamed", "description": "d"})
	if rec.Code != 200 || out["name"] != "Alpha Renamed" || out["description"] != "d" || out["slug"] != "alpha" {
		t.Fatalf("rename keeps the slug stable: %d %v", rec.Code, out)
	}
	dir := filepath.Join(e.workspace, "other")
	os.MkdirAll(dir, 0o755)
	if rec, out := admin.do("PATCH", id, map[string]string{"iac_path": dir}); rec.Code != 200 || out["iac_path"] != dir || out["name"] != "Alpha Renamed" {
		t.Fatalf("path change leaves other fields: %d %v", rec.Code, out)
	}
	if rec, _ := admin.do("PATCH", id, map[string]string{"description": ""}); rec.Code != 200 || e.fs.projects[p.ID].Description != "" {
		t.Fatalf("an empty description clears it: %d", rec.Code)
	}

	for _, body := range []any{
		`{}`, map[string]string{"name": " "}, map[string]string{"name": strings.Repeat("x", 81)},
		map[string]string{"iac_path": "/etc"}, map[string]string{"iac_path": filepath.Join(e.workspace, "nope")},
		`{"slug":"hack"}`, `nope`,
	} {
		if rec, _ := admin.do("PATCH", id, body); rec.Code != 400 {
			t.Errorf("%v: got %d, want 400", body, rec.Code)
		}
	}
	if rec, _ := admin.do("PATCH", "/projects/missing", map[string]string{"name": "x"}); rec.Code != 404 {
		t.Fatalf("unknown project: %d", rec.Code)
	}
	if e.fs.projects[p.ID].Slug != "alpha" || e.fs.projects[p.ID].IACPath != dir {
		t.Fatal("rejected updates must not change the project")
	}
}

// --- delete ---------------------------------------------------------------------

func TestDeleteProject(t *testing.T) {
	e := newAdminEnv(t)
	admin := e.as("root", "admin")

	// Files of the project stay on disk; only the app record goes.
	dir := filepath.Join(e.workspace, "alpha")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "rg.tf"), []byte("# keep me"), 0o644)
	p := e.fs.addProject("Alpha")
	e.fs.projects[p.ID] = store.Project{ID: p.ID, Name: "Alpha", Slug: "alpha", IACPath: dir}

	e.fs.pending[p.ID] = 2
	rec, out := admin.do("DELETE", "/projects/"+p.ID, nil)
	if rec.Code != 409 || out["pending_changesets"] != float64(2) {
		t.Fatalf("pending changesets must ask for confirmation: %d %v", rec.Code, out)
	}
	if _, ok := e.fs.projects[p.ID]; !ok {
		t.Fatal("a refused delete must keep the project")
	}

	if rec, _ := admin.do("DELETE", "/projects/"+p.ID+"?force=true", nil); rec.Code != 204 {
		t.Fatalf("forced delete: %d", rec.Code)
	}
	if _, ok := e.fs.projects[p.ID]; ok {
		t.Fatal("project still exists")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "rg.tf")); err != nil || string(b) != "# keep me" {
		t.Fatalf("delete must not touch repository files: %v %q", err, b)
	}
	last := e.fs.audit[len(e.fs.audit)-1]
	if last.Action != "project.delete" || last.Detail["pending_changesets"] != 2 || last.Detail["project_id"] != p.ID {
		t.Fatalf("delete must be audited with what was dropped: %+v", last)
	}

	q := e.fs.addProject("Quiet")
	if rec, _ := admin.do("DELETE", "/projects/"+q.ID, nil); rec.Code != 204 {
		t.Fatalf("no pending changesets: plain delete works: %d", rec.Code)
	}
	if rec, _ := admin.do("DELETE", "/projects/"+q.ID, nil); rec.Code != 404 {
		t.Fatalf("second delete: %d", rec.Code)
	}
}

// --- members & users ------------------------------------------------------------

func TestMembers(t *testing.T) {
	e := newAdminEnv(t)
	p := e.fs.addProject("Alpha")
	admin := e.as("root", "admin")
	for _, u := range []string{"alice", "bob"} {
		e.as(u, "user").do("GET", "/projects", nil)
	}
	path := "/projects/" + p.ID + "/members"

	if rec, _ := admin.do("GET", path, nil); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty members must be []: %d %q", rec.Code, rec.Body.String())
	}
	rec, _ := admin.do("PUT", path, map[string]any{"user_oids": []string{"alice", " bob ", "alice"}})
	var got []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != 200 || len(got) != 2 {
		t.Fatalf("set members (duplicates and spaces collapse): %d %s", rec.Code, rec.Body.String())
	}
	// The assigned user now sees the project.
	eq(t, names(e.as("alice", "user").list("/projects")), []string{"Alpha"})

	// The list is replaced as a whole.
	admin.do("PUT", path, map[string]any{"user_oids": []string{"bob"}})
	eq(t, names(e.as("alice", "user").list("/projects")), []string{})
	admin.do("PUT", path, map[string]any{"user_oids": []string{}})
	eq(t, names(e.as("bob", "user").list("/projects")), []string{})

	before := len(e.fs.members[p.ID])
	for _, tc := range []struct {
		body any
		want int
	}{
		{map[string]any{"user_oids": []string{"ghost"}}, 422},
		{map[string]any{"user_oids": []string{"alice", ""}}, 400},
		{map[string]any{}, 400},
		{`{"user_oids":null}`, 400},
		{`oops`, 400},
	} {
		if rec, _ := admin.do("PUT", path, tc.body); rec.Code != tc.want {
			t.Errorf("%v: got %d, want %d", tc.body, rec.Code, tc.want)
		}
	}
	if len(e.fs.members[p.ID]) != before {
		t.Fatal("a rejected request must not change the member list")
	}
	if rec, _ := admin.do("PUT", "/projects/missing/members", map[string]any{"user_oids": []string{}}); rec.Code != 404 {
		t.Fatalf("unknown project: %d", rec.Code)
	}
	if rec, _ := admin.do("GET", "/projects/missing/members", nil); rec.Code != 404 {
		t.Fatalf("unknown project: %d", rec.Code)
	}
	if got := e.fs.actions(); got[len(got)-1] != "project.members.set" {
		t.Fatalf("member changes are audited: %v", got)
	}
}

func TestUsersAreRecordedOnFirstRequestAndListed(t *testing.T) {
	e := newAdminEnv(t)
	p := e.fs.addProject("Alpha")
	e.as("alice", "user").do("GET", "/projects", nil)
	e.as("alice", "user").do("GET", "/projects", nil)
	e.as("alice", "user").do("GET", "/projects", nil)
	if e.fs.upserts != 1 {
		t.Fatalf("a user must be written once per interval, got %d writes", e.fs.upserts)
	}
	e.fs.members[p.ID] = map[string]bool{"alice": true}
	rec, _ := e.as("root", "admin").do("GET", "/admin/users", nil)
	var users []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &users)
	var alice map[string]any
	for _, u := range users {
		if u["oid"] == "alice" {
			alice = u
		}
	}
	if alice == nil || alice["email"] != "alice@x.test" || alice["project_count"] != float64(1) {
		t.Fatalf("users: %v", users)
	}
}

func TestUserSyncFailureDoesNotBlockTheRequest(t *testing.T) {
	e := newAdminEnv(t)
	e.fs.failUsers = true
	if rec, _ := e.as("alice", "user").do("GET", "/projects", nil); rec.Code != 200 {
		t.Fatalf("a failing user write must not break the page: %d", rec.Code)
	}
	e.as("alice", "user").do("GET", "/projects", nil)
	if e.fs.upserts != 2 {
		t.Fatalf("a failed write is retried on the next request, got %d", e.fs.upserts)
	}
}
