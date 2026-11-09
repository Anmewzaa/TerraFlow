package api_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Phase 7: access-control tests that walk the real router, so a route added
// later without a guard is caught here without anyone updating a list.

type route struct{ method, pattern string }

func (e *pgEnv) routes() []route {
	var out []route
	err := chi.Walk(e.handler.(chi.Routes), func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		pattern = strings.ReplaceAll(pattern, "/*/", "/")
		if strings.HasPrefix(pattern, "/api/") && pattern != "/api/healthz" {
			out = append(out, route{method, strings.TrimSuffix(pattern, "/")})
		}
		return nil
	})
	if err != nil {
		e.t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pattern+out[i].method < out[j].pattern+out[j].method })
	return out
}

// fill replaces path parameters; {id} becomes the project id.
func fill(pattern, projectID string) string {
	p := strings.ReplaceAll(pattern, "{id}", projectID)
	for _, name := range []string{"{csId}", "{moduleId}", "{name}", "{version}"} {
		p = strings.ReplaceAll(p, name, "x")
	}
	return p
}

// adminOnly lists the routes that need the admin role outside /api/admin/.
var adminOnly = map[string]bool{
	"POST /api/projects":             true,
	"PATCH /api/projects/{id}":       true,
	"DELETE /api/projects/{id}":      true,
	"GET /api/projects/{id}/members": true,
	"PUT /api/projects/{id}/members": true,
}

func isAdminRoute(r route) bool {
	return strings.HasPrefix(r.pattern, "/api/admin/") || adminOnly[r.method+" "+r.pattern]
}

// isProjectRoute: routes below /projects/{id}/ that members (not only admins) may use.
func isProjectRoute(r route) bool {
	return strings.Contains(r.pattern, "/projects/{id}") && !isAdminRoute(r)
}

func (e *pgEnv) newProject() string {
	e.t.Helper()
	e.do("root", "admin", "GET", "/api/projects", nil) // records the admin as a user
	code, p, raw := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "Matrix"})
	if code != 201 {
		e.t.Fatalf("create project: %d %s", code, raw)
	}
	return p["id"].(string)
}

func TestRouteWalkFindsTheRoutes(t *testing.T) {
	e := newPGEnv(t)
	routes := e.routes()
	if len(routes) < 30 {
		t.Fatalf("the walk found only %d routes, so the matrix tests below prove little", len(routes))
	}
	var admin, project int
	for _, r := range routes {
		if isAdminRoute(r) {
			admin++
		}
		if isProjectRoute(r) {
			project++
		}
	}
	if admin < 10 || project < 20 {
		t.Fatalf("expected many admin and project routes, got %d and %d", admin, project)
	}
}

func TestEveryAPIRouteNeedsSignIn(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	for _, r := range e.routes() {
		code, _, _ := e.do("", "", r.method, fill(r.pattern, id), nil)
		if code != http.StatusUnauthorized {
			t.Errorf("%s %s without sign-in: %d, want 401", r.method, r.pattern, code)
		}
	}
}

func TestEveryAdminRouteRefusesDevelopers(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	e.do("alice", "user", "GET", "/api/projects", nil)
	// Even a member of the project is not an admin.
	if code, _, raw := e.do("root", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"alice"}}); code != 200 {
		t.Fatalf("assign: %d %s", code, raw)
	}
	for _, r := range e.routes() {
		if !isAdminRoute(r) {
			continue
		}
		code, _, _ := e.do("alice", "user", r.method, fill(r.pattern, id), map[string]any{})
		if code != http.StatusForbidden {
			t.Errorf("%s %s as a developer: %d, want 403", r.method, r.pattern, code)
		}
	}
}

func TestEveryProjectRouteHidesProjectsFromNonMembers(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	e.do("mallory", "user", "GET", "/api/projects", nil)
	n := 0
	for _, r := range e.routes() {
		if !isProjectRoute(r) {
			continue
		}
		n++
		code, _, _ := e.do("mallory", "user", r.method, fill(r.pattern, id), map[string]any{})
		if code != http.StatusNotFound {
			t.Errorf("%s %s as a non-member: %d, want 404", r.method, r.pattern, code)
		}
	}
	if n == 0 {
		t.Fatal("no project routes were checked")
	}
}

func TestAdminCanReachEveryRoute(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	for _, r := range e.routes() {
		if r.method == "DELETE" && r.pattern == "/api/projects/{id}" {
			continue // would remove the project the other calls use
		}
		code, _, _ := e.do("root", "admin", r.method, fill(r.pattern, id), map[string]any{})
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			t.Errorf("%s %s as an admin: %d", r.method, r.pattern, code)
		}
		if code >= 500 {
			t.Errorf("%s %s as an admin with an empty body: %d, a bad request must not be a 5xx", r.method, r.pattern, code)
		}
	}
}

// A non-member must not be able to tell a real project from one that does not exist.
func TestNonMemberCannotTellRealFromMissingProject(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	e.do("mallory", "user", "GET", "/api/projects", nil)
	for _, path := range []string{"/files", "/graph", "/changesets/", "/audit", "/import"} {
		c1, _, b1 := e.do("mallory", "user", "GET", "/api/projects/"+id+path, nil)
		c2, _, b2 := e.do("mallory", "user", "GET", "/api/projects/00000000-0000-0000-0000-000000000000"+path, nil)
		if c1 != c2 || string(b1) != string(b2) {
			t.Errorf("GET %s differs: %d %s vs %d %s", path, c1, b1, c2, b2)
		}
	}
}

func TestMemberCannotReviewTheirOwnChangeset(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	e.do("alice", "user", "GET", "/api/projects", nil)
	e.do("root", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"alice"}})

	code, cs, raw := e.do("alice", "user", "POST", "/api/projects/"+id+"/changesets/", map[string]string{"env": "dev"})
	if code != 201 && code != 200 {
		t.Fatalf("member creates a changeset: %d %s", code, raw)
	}
	csID, _ := cs["id"].(string)
	if csID == "" {
		t.Fatalf("no changeset id: %s", raw)
	}
	base := "/api/projects/" + id + "/changesets/" + csID
	if code, _, raw := e.do("alice", "user", "POST", base+"/publish", map[string]string{"title": "mine"}); code != 200 {
		t.Fatalf("publish: %d %s", code, raw)
	}
	for _, action := range []string{"/approve", "/reject"} {
		code, _, _ := e.do("alice", "user", "POST", base+action, map[string]string{"reason": "x"})
		if code != http.StatusForbidden {
			t.Errorf("a developer on %s: %d, want 403", action, code)
		}
	}
	// And the changeset is still waiting, so nothing was decided.
	_, got, raw := e.do("alice", "user", "GET", base, nil)
	if got["status"] != "published" {
		t.Errorf("status after refused review: %v %s", got["status"], raw)
	}
}

func TestOversizedBodiesAreRefused(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	big := `{"name":"` + strings.Repeat("a", 3<<20) + `"}`
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/projects"},
		{"PATCH", "/api/projects/" + id},
		{"POST", "/api/admin/catalog/validate"},
		{"POST", "/api/admin/catalog/modules"},
		{"POST", "/api/admin/resources"},
		{"POST", "/api/projects/" + id + "/changesets/x/reject"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(big))
		req.Header.Set("X-User", "root")
		req.Header.Set("X-Role", "admin")
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s %s with a 3 MB body: %d, want a 4xx", tc.method, tc.path, rec.Code)
		}
	}
}

func TestMalformedBodiesAreClientErrors(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/projects"},
		{"PATCH", "/api/projects/" + id},
		{"PUT", "/api/projects/" + id + "/members"},
		{"POST", "/api/admin/catalog/validate"},
		{"POST", "/api/admin/resources"},
		{"POST", "/api/projects/" + id + "/changesets/"},
		{"PUT", "/api/projects/" + id + "/component-config"},
	} {
		for _, body := range []string{`{`, `[]`, `"text"`, `null`, `{"name":`} {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(body))
			req.Header.Set("X-User", "root")
			req.Header.Set("X-Role", "admin")
			rec := httptest.NewRecorder()
			e.handler.ServeHTTP(rec, req)
			if rec.Code >= 500 {
				t.Errorf("%s %s with body %q: %d, want no 5xx", tc.method, tc.path, body, rec.Code)
			}
		}
	}
}

func TestProjectNamesAreNotInjectable(t *testing.T) {
	e := newPGEnv(t)
	e.do("root", "admin", "GET", "/api/projects", nil)
	for _, name := range []string{"'; DROP TABLE projects;--", "../../etc", `<script>alert(1)</script>`, "a\x00b", strings.Repeat("x", 5000)} {
		code, p, _ := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": name})
		if code >= 500 {
			t.Errorf("name %q: %d", name, code)
		}
		if code == 201 {
			if slug, _ := p["slug"].(string); strings.ContainsAny(slug, "/.\\' ;<>") {
				t.Errorf("slug %q from name %q is not safe as a folder name", slug, name)
			}
		}
	}
	if names := e.projectNames("root", "admin"); names == nil {
		t.Fatal("the projects table must still answer")
	}
}

// The dev sign-in routes exist only when the server was started with it.
func TestDevLoginRoutesAbsentWithoutDevLogin(t *testing.T) {
	e := newPGEnv(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/dev/users"}, {"POST", "/api/dev/login"}, {"POST", "/api/dev/logout"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"oid":"dev-admin-0001"}`))
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)
		if rec.Code/100 == 2 || len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s %s without DevLogin: %d cookies=%v", tc.method, tc.path, rec.Code, rec.Result().Cookies())
		}
	}
}

// Known gaps (docs/03-phases.md, Phase 7), skipped until fixed.

func TestCORSIsNotWideOpen(t *testing.T) {
	t.Skip("known gap (Phase 7): corsMiddleware answers Access-Control-Allow-Origin: * on every route")
	e := newPGEnv(t)
	req := httptest.NewRequest("GET", "/api/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatal("must not allow every origin")
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Skip("known gap (Phase 7): no CSP, X-Frame-Options, nosniff or HSTS headers yet")
	e := newPGEnv(t)
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/healthz", nil))
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
}

// A member of one project must not reach another project's changeset by id.
func TestChangesetsAreScopedToTheirProject(t *testing.T) {
	e := newPGEnv(t)
	idA := e.newProject()
	_, pb, _ := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "Other"})
	idB := pb["id"].(string)
	e.do("alice", "user", "GET", "/api/projects", nil)
	e.do("root", "admin", "PUT", "/api/projects/"+idA+"/members", map[string]any{"user_oids": []string{"alice"}})

	// A changeset that lives in project B.
	code, cs, raw := e.do("root", "admin", "POST", "/api/projects/"+idB+"/changesets/", map[string]string{"env": "dev"})
	if code != 200 && code != 201 {
		t.Fatalf("create in B: %d %s", code, raw)
	}
	csB := cs["id"].(string)

	// Alice is a member of A only. Asking for B's changeset through A must fail
	// for every action that takes a changeset id.
	base := "/api/projects/" + idA + "/changesets/" + csB
	for _, tc := range []struct{ method, path string }{
		{"GET", base}, {"PUT", base}, {"DELETE", base},
		{"POST", base + "/versions"}, {"POST", base + "/publish"}, {"GET", base + "/preview"},
		{"POST", base + "/revise"}, {"GET", base + "/review"}, {"POST", base + "/approve"}, {"POST", base + "/reject"},
	} {
		code, _, raw := e.do("alice", "user", tc.method, tc.path, map[string]string{"title": "x", "reason": "x"})
		if code != http.StatusNotFound {
			t.Errorf("%s %s: %d %s, want 404", tc.method, tc.path, code, raw)
		}
		// Even an admin cannot act on B's changeset through A's URL.
		code, _, _ = e.do("root", "admin", tc.method, tc.path, map[string]string{"title": "x", "reason": "x"})
		if code != http.StatusNotFound {
			t.Errorf("admin %s %s: %d, want 404", tc.method, tc.path, code)
		}
	}
	// The changeset is untouched.
	if code, got, _ := e.do("root", "admin", "GET", "/api/projects/"+idB+"/changesets/"+csB, nil); code != 200 || got["status"] != "draft" {
		t.Fatalf("B's changeset after the attempts: %d %v", code, got)
	}
}

func TestMalformedIDsAreNotFoundNotServerErrors(t *testing.T) {
	e := newPGEnv(t)
	id := e.newProject()
	for _, raw := range []string{"x", "not-a-uuid", "1' OR '1'='1", "\x00", "../../etc/passwd", "00000000-0000-0000-0000-00000000000g"} {
		bad := url.PathEscape(raw)
		for _, path := range []string{
			"/api/projects/" + bad + "/files",
			"/api/projects/" + id + "/changesets/" + bad,
			"/api/projects/" + id + "/changesets/" + bad + "/preview",
		} {
			for _, who := range [][2]string{{"root", "admin"}, {"alice", "user"}} {
				code, _, _ := e.do(who[0], who[1], "GET", path, nil)
				if code >= 500 {
					t.Errorf("%s as %s: %d", path, who[1], code)
				}
			}
		}
	}
}
