package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/api"
	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

// pgEnv runs the real router (NewRouter) on a real database, as the server does
// in database mode, so the wiring of guards and admin routes is what gets tested.
type pgEnv struct {
	t       *testing.T
	q       *store.Queries
	handler http.Handler
	ws      string
}

func newPGEnv(t *testing.T) *pgEnv {
	t.Helper()
	q, _ := storetest.New(t)
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	samples := filepath.Join(root, "samples", "AIC")
	os.MkdirAll(ws, 0o755)
	os.MkdirAll(samples, 0o755)

	// The test picks the user per request, like a session cookie would.
	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if oid := r.Header.Get("X-User"); oid != "" {
				roles := []string{"developer"}
				if r.Header.Get("X-Role") == "admin" {
					roles = []string{"devops"}
				}
				u := &auth.User{OID: oid, Name: "name-" + oid, Email: oid + "@x.test", Roles: roles}
				r = r.WithContext(auth.WithUser(r.Context(), u))
			} else {
				http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	h := api.NewRouter(api.RouterConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Queries: q, AuthMiddleware: authMW,
		SampleDir: samples, WorkspaceDir: ws, CatalogService: catalog.NewService(catalog.NewPGStore(q)),
	})
	return &pgEnv{t: t, q: q, handler: h, ws: ws}
}

func (e *pgEnv) do(user, role, method, path string, body any) (int, map[string]any, []byte) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if user != "" {
		req.Header.Set("X-User", user)
		req.Header.Set("X-Role", role)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec.Body.Bytes()
}

func (e *pgEnv) projectNames(user, role string) []string {
	_, _, raw := e.do(user, role, "GET", "/api/projects", nil)
	var list []map[string]any
	json.Unmarshal(raw, &list)
	out := []string{}
	for _, p := range list {
		out = append(out, p["name"].(string))
	}
	return out
}

func TestRouterAdminAssignsProjectToUser(t *testing.T) {
	e := newPGEnv(t)

	// Both people sign in once, which records them as users.
	e.do("alice", "user", "GET", "/api/projects", nil)
	e.do("root", "admin", "GET", "/api/projects", nil)

	code, p, _ := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "AIC", "description": "core"})
	if code != 201 || p["slug"] != "aic" {
		t.Fatalf("create: %d %v", code, p)
	}
	id := p["id"].(string)
	if names := e.projectNames("alice", "user"); len(names) != 0 {
		t.Fatalf("an unassigned user sees nothing, got %v", names)
	}
	if code, _, _ := e.do("alice", "user", "GET", "/api/projects/"+id+"/files", nil); code != 404 {
		t.Fatalf("unassigned user on a project route: %d", code)
	}

	if code, _, raw := e.do("root", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"alice"}}); code != 200 {
		t.Fatalf("assign: %d %s", code, raw)
	}
	if names := e.projectNames("alice", "user"); len(names) != 1 || names[0] != "AIC" {
		t.Fatalf("assigned user sees the project, got %v", names)
	}
	for _, path := range []string{"/api/projects/" + id + "/files", "/api/projects/" + id + "/changesets/", "/api/projects/" + id + "/audit"} {
		if code, _, raw := e.do("alice", "user", "GET", path, nil); code != 200 {
			t.Errorf("member on %s: %d %s", path, code, raw)
		}
	}

	// Alice may use the project but not administer it.
	if code, _, _ := e.do("alice", "user", "PATCH", "/api/projects/"+id, map[string]string{"name": "x"}); code != 403 {
		t.Fatalf("user PATCH: %d", code)
	}
	if code, _, _ := e.do("alice", "user", "DELETE", "/api/projects/"+id, nil); code != 403 {
		t.Fatalf("user DELETE: %d", code)
	}
	if code, _, _ := e.do("alice", "user", "POST", "/api/projects", map[string]string{"name": "Mine"}); code != 403 {
		t.Fatalf("user POST: %d", code)
	}
	if code, _, _ := e.do("alice", "user", "GET", "/api/admin/users", nil); code != 403 {
		t.Fatalf("user GET admin/users: %d", code)
	}
	if code, _, _ := e.do("", "", "GET", "/api/projects", nil); code != 401 {
		t.Fatalf("anonymous: %d", code)
	}

	// Admin edits, then removes access.
	if code, out, _ := e.do("root", "admin", "PATCH", "/api/projects/"+id, map[string]string{"name": "AIC Core"}); code != 200 || out["name"] != "AIC Core" || out["slug"] != "aic" {
		t.Fatalf("rename: %d %v", code, out)
	}
	e.do("root", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{}})
	if names := e.projectNames("alice", "user"); len(names) != 0 {
		t.Fatalf("removed user sees nothing, got %v", names)
	}
	if names := e.projectNames("root", "admin"); len(names) != 1 {
		t.Fatalf("admin always sees every project, got %v", names)
	}

	// Everything an admin did is in the audit log.
	_, _, raw := e.do("root", "admin", "GET", "/api/projects/"+id+"/audit", nil)
	for _, want := range []string{"project.create", "project.update", "project.members.set"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("audit log misses %s: %s", want, raw)
		}
	}
}

func TestRouterDeleteKeepsFilesAndAudit(t *testing.T) {
	e := newPGEnv(t)
	e.do("root", "admin", "GET", "/api/projects", nil)
	_, p, _ := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "Alpha"})
	id, dir := p["id"].(string), p["iac_path"].(string)
	os.MkdirAll(filepath.Join(dir, "common"), 0o755)
	os.WriteFile(filepath.Join(dir, "common", "rg.tf"), []byte("# keep"), 0o644)

	// A changeset waiting for review asks for confirmation.
	csID, err := e.q.CreateChangeset(t.Context(), store.Changeset{ProjectID: id, Env: "dev", UserOID: "root", CanvasJSON: []byte(`{"nodes":[],"edges":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	e.q.PublishChangeset(t.Context(), csID, "t")
	code, out, _ := e.do("root", "admin", "DELETE", "/api/projects/"+id, nil)
	if code != 409 || out["pending_changesets"] != float64(1) {
		t.Fatalf("pending: %d %v", code, out)
	}
	if code, _, _ := e.do("root", "admin", "DELETE", "/api/projects/"+id+"?force=true", nil); code != 204 {
		t.Fatalf("force: %d", code)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "common", "rg.tf")); err != nil || string(b) != "# keep" {
		t.Fatalf("repository files must survive: %v %q", err, b)
	}
	if names := e.projectNames("root", "admin"); len(names) != 0 {
		t.Fatalf("project still listed: %v", names)
	}
	rows, err := e.q.ListAudit(t.Context(), id, 10)
	if err != nil || len(rows) == 0 {
		t.Fatalf("audit must survive the delete: %v %v", rows, err)
	}
}

func TestRouterUnknownUserCannotBeAssigned(t *testing.T) {
	e := newPGEnv(t)
	e.do("root", "admin", "GET", "/api/projects", nil)
	_, p, _ := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "Alpha"})
	code, out, _ := e.do("root", "admin", "PUT", "/api/projects/"+p["id"].(string)+"/members", map[string]any{"user_oids": []string{"never-signed-in"}})
	if code != 422 || !strings.Contains(out["error"].(string), "sign in") {
		t.Fatalf("%d %v", code, out)
	}
}

const pgWidget = `{
  "title": "Widget", "description": "A widget.", "type": "object",
  "x-tf-module": {"source": "widget", "version": "v1", "target_folder": "widgets"},
  "required": ["name"],
  "properties": {"name": {"type": "string", "title": "Name", "description": "d"}}
}`

func (e *pgEnv) catalogNames() []string {
	_, _, raw := e.do("alice", "user", "GET", "/api/catalog", nil)
	var list []map[string]any
	json.Unmarshal(raw, &list)
	out := []string{}
	for _, m := range list {
		out = append(out, m["name"].(string))
	}
	return out
}

func TestRouterCatalogImportAndDeprecate(t *testing.T) {
	e := newPGEnv(t)
	e.do("alice", "user", "GET", "/api/projects", nil)
	body := map[string]any{"name": "widget", "schema": json.RawMessage(pgWidget)}

	if code, _, _ := e.do("alice", "user", "POST", "/api/admin/catalog/modules", body); code != 403 {
		t.Fatalf("user import: %d", code)
	}
	if code, _, _ := e.do("alice", "user", "GET", "/api/admin/catalog/modules", nil); code != 403 {
		t.Fatalf("user list: %d", code)
	}
	if code, out, raw := e.do("root", "admin", "POST", "/api/admin/catalog/modules", body); code != 201 || out["status"] != "imported" {
		t.Fatalf("import: %d %s", code, raw)
	}
	if names := e.catalogNames(); len(names) != 1 || names[0] != "widget" {
		t.Fatalf("developers see the new module: %v", names)
	}
	if code, out, _ := e.do("root", "admin", "POST", "/api/admin/catalog/modules", body); code != 200 || out["status"] != "unchanged" {
		t.Fatalf("re-import: %d %v", code, out)
	}
	if code, _, _ := e.do("root", "admin", "PATCH", "/api/admin/catalog/modules/widget/v1", map[string]string{"status": "deprecated"}); code != 200 {
		t.Fatalf("deprecate: %d", code)
	}
	if names := e.catalogNames(); len(names) != 0 {
		t.Fatalf("deprecated module hidden: %v", names)
	}
	if code, _, _ := e.do("alice", "user", "GET", "/api/catalog/widget/v1", nil); code != 200 {
		t.Fatalf("a deprecated version stays readable for existing work: %d", code)
	}
	_, _, raw := e.do("root", "admin", "GET", "/api/admin/catalog/modules", nil)
	if !strings.Contains(string(raw), `"deprecated"`) || !strings.Contains(string(raw), `"imported_by":"root"`) {
		t.Fatalf("admin list: %s", raw)
	}
}

func TestRouterComponentConfigFollowsProjectAccess(t *testing.T) {
	e := newPGEnv(t)
	e.do("alice", "user", "GET", "/api/projects", nil)
	e.do("mallory", "user", "GET", "/api/projects", nil)
	_, p, _ := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "Alpha"})
	id := p["id"].(string)
	e.do("root", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"alice"}})
	path := "/api/projects/" + id + "/component-config"

	if code, _, raw := e.do("alice", "user", "PUT", path, map[string]string{"address": "common/rg_app", "color": "cyan"}); code != 200 {
		t.Fatalf("member sets a colour: %d %s", code, raw)
	}
	_, _, raw := e.do("alice", "user", "GET", path, nil)
	if !strings.Contains(string(raw), `"common/rg_app"`) || !strings.Contains(string(raw), `"cyan"`) {
		t.Fatalf("list: %s", raw)
	}
	for _, method := range []string{"GET", "PUT"} {
		if code, _, _ := e.do("mallory", "user", method, path, map[string]string{"address": "common/x", "color": "red"}); code != 404 {
			t.Errorf("non-member %s: %d", method, code)
		}
	}
	if code, _, _ := e.do("alice", "user", "PUT", path, map[string]string{"address": "bad", "color": "cyan"}); code != 400 {
		t.Fatalf("bad address: %d", code)
	}
}
