package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

const moduleV1 = `{
  "title": "Widget", "description": "A widget.", "type": "object",
  "x-tf-module": {"source": "widget", "version": "v1", "target_folder": "widgets"},
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "title": "Name", "description": "d"},
    "size": {"type": "string", "title": "Size", "description": "d", "enum": ["s", "m"], "default": "s"}
  }
}`

// v2 drops the "m" size and adds a required input: two breaking changes.
const moduleV2 = `{
  "title": "Widget", "description": "A widget.", "type": "object",
  "x-tf-module": {"source": "widget", "version": "v2", "target_folder": "widgets"},
  "required": ["name", "owner"],
  "properties": {
    "name": {"type": "string", "title": "Name", "description": "d"},
    "owner": {"type": "string", "title": "Owner", "description": "d"},
    "size": {"type": "string", "title": "Size", "description": "d", "enum": ["s"], "default": "s"}
  }
}`

type auditSink struct {
	mu   sync.Mutex
	rows []string
}

func (a *auditSink) LogAudit(_ context.Context, _, action, resource string, _ any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, action+" "+resource)
	return nil
}

type catEnv struct {
	t      *testing.T
	svc    *catalog.Service
	router chi.Router
	audit  *auditSink
}

func newCatEnv(t *testing.T) *catEnv {
	t.Helper()
	dir := t.TempDir() // an empty file-backed catalog
	e := &catEnv{t: t, svc: catalog.NewService(catalog.NewFSStore(dir)), audit: &auditSink{}}
	ca := &CatalogAdmin{Svc: e.svc, Audit: e.audit}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			roles := []string{"developer"}
			if req.Header.Get("X-Role") == "admin" {
				roles = []string{"devops"}
			}
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), &auth.User{OID: "root", Roles: roles})))
		})
	})
	admin := r.With(RequireAdmin)
	admin.Get("/admin/catalog/modules", ca.List())
	admin.Post("/admin/catalog/validate", ca.Validate())
	admin.Post("/admin/catalog/modules", ca.Import())
	admin.Patch("/admin/catalog/modules/{name}/{version}", ca.SetStatus())
	r.Get("/catalog", CatalogListHandler(e.svc))
	r.Get("/catalog/{name}/{version}", CatalogGetHandler(e.svc))
	e.router = r
	return e
}

func (e *catEnv) do(role, method, path string, body any) (int, map[string]any, []byte) {
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		json.NewEncoder(&buf).Encode(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("X-Role", role)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec.Body.Bytes()
}

func imp(name, schema string) map[string]any {
	return map[string]any{"name": name, "schema": json.RawMessage(schema)}
}

func (e *catEnv) catalogNames() []string {
	_, _, raw := e.do("user", "GET", "/catalog", nil)
	var list []map[string]any
	json.Unmarshal(raw, &list)
	out := []string{}
	for _, m := range list {
		vs := []string{}
		for _, v := range m["versions"].([]any) {
			vs = append(vs, v.(string))
		}
		out = append(out, m["name"].(string)+":"+strings.Join(vs, "+"))
	}
	return out
}

func TestCatalogAdminNeedsTheAdminRole(t *testing.T) {
	e := newCatEnv(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/admin/catalog/modules"}, {"POST", "/admin/catalog/validate"},
		{"POST", "/admin/catalog/modules"}, {"PATCH", "/admin/catalog/modules/widget/v1"},
	} {
		if code, _, _ := e.do("user", tc.method, tc.path, imp("widget", moduleV1)); code != 403 {
			t.Errorf("%s %s as a user: %d", tc.method, tc.path, code)
		}
	}
	if len(e.catalogNames()) != 0 || len(e.audit.rows) != 0 {
		t.Fatal("a rejected request must change nothing")
	}
}

func TestValidateReportsWithoutSaving(t *testing.T) {
	e := newCatEnv(t)

	code, out, _ := e.do("admin", "POST", "/admin/catalog/validate", imp("widget", moduleV1))
	res := out
	if code != 200 || res["exists"] != "none" || res["report"].(map[string]any)["valid"] != true {
		t.Fatalf("good schema: %d %v", code, out)
	}
	if len(e.catalogNames()) != 0 {
		t.Fatal("validate must not import anything")
	}

	bad := strings.Replace(moduleV1, `"default": "s"`, `"default": "xl"`, 1)
	code, out, _ = e.do("admin", "POST", "/admin/catalog/validate", imp("widget", bad))
	rep := out["report"].(map[string]any)
	if code != 200 || rep["valid"] != false || len(rep["errors"].([]any)) == 0 {
		t.Fatalf("a bad schema is reported (200), not rejected: %d %v", code, out)
	}

	for name, body := range map[string]any{
		"not json": "{", "no schema": map[string]any{"name": "widget"}, "null schema": `{"name":"widget","schema":null}`,
	} {
		if code, _, _ := e.do("admin", "POST", "/admin/catalog/validate", body); code != 400 {
			t.Errorf("%s: %d", name, code)
		}
	}
	huge := `{"name":"widget","schema":{"x":"` + strings.Repeat("a", maxSchemaBytes) + `"}}`
	if code, _, _ := e.do("admin", "POST", "/admin/catalog/validate", huge); code != 400 {
		t.Errorf("oversized body: %d", code)
	}
}

func TestImportPublishesAndIsImmutable(t *testing.T) {
	e := newCatEnv(t)

	code, out, _ := e.do("admin", "POST", "/admin/catalog/modules", imp("widget", moduleV1))
	if code != 201 || out["status"] != "imported" {
		t.Fatalf("import: %d %v", code, out)
	}
	if got := e.catalogNames(); len(got) != 1 || got[0] != "widget:v1" {
		t.Fatalf("catalog after import: %v", got)
	}
	if code, _, _ := e.do("user", "GET", "/catalog/widget/v1", nil); code != 200 {
		t.Fatalf("developers can open the module: %d", code)
	}

	// The same bytes again are a no-op; different bytes under the same version are refused.
	if code, out, _ := e.do("admin", "POST", "/admin/catalog/modules", imp("widget", moduleV1)); code != 200 || out["status"] != "unchanged" {
		t.Fatalf("re-import of identical schema: %d %v", code, out)
	}
	changed := strings.Replace(moduleV1, `"A widget."`, `"Changed."`, 1)
	code, out, _ = e.do("admin", "POST", "/admin/catalog/modules", imp("widget", changed))
	if code != 409 || !strings.Contains(out["error"].(string), "new version") {
		t.Fatalf("published versions are immutable: %d %v", code, out)
	}
	_, got, _ := e.do("user", "GET", "/catalog/widget/v1", nil)
	if strings.Contains(string(mustJSON(got)), "Changed.") {
		t.Fatal("the published schema was replaced")
	}

	// An invalid schema is refused with the report and stores nothing.
	broken := strings.Replace(moduleV1, `"source": "widget", `, ``, 1)
	code, out, _ = e.do("admin", "POST", "/admin/catalog/modules", imp("broken", broken))
	if code != 422 || out["result"] == nil {
		t.Fatalf("invalid import: %d %v", code, out)
	}
	if len(e.catalogNames()) != 1 {
		t.Fatal("an invalid schema must not be stored")
	}

	if want := []string{"catalog.import module:widget/v1"}; strings.Join(e.audit.rows, "|") != strings.Join(want, "|") {
		t.Fatalf("audit: %v", e.audit.rows)
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func TestNewVersionShowsWhatBreaks(t *testing.T) {
	e := newCatEnv(t)
	e.do("admin", "POST", "/admin/catalog/modules", imp("widget", moduleV1))

	code, out, _ := e.do("admin", "POST", "/admin/catalog/validate", imp("widget", moduleV2))
	if code != 200 || out["compared_to"] != "v1" || out["exists"] != "none" {
		t.Fatalf("validate v2: %d %v", code, out)
	}
	if n := out["breaking_changes"]; n != float64(2) {
		t.Fatalf("v2 drops an enum value and adds a required input: breaking=%v changes=%v", n, out["changes"])
	}

	if code, _, _ := e.do("admin", "POST", "/admin/catalog/modules", imp("widget", moduleV2)); code != 201 {
		t.Fatalf("a breaking version is still allowed, it is a new version: %d", code)
	}
	if got := e.catalogNames(); len(got) != 1 || got[0] != "widget:v1+v2" {
		t.Fatalf("both versions listed: %v", got)
	}
	last := e.audit.rows[len(e.audit.rows)-1]
	if last != "catalog.import module:widget/v2" {
		t.Fatalf("audit: %v", e.audit.rows)
	}
}

func TestDeprecateHidesFromDevelopersButNotFromAdmins(t *testing.T) {
	e := newCatEnv(t)
	e.do("admin", "POST", "/admin/catalog/modules", imp("widget", moduleV1))
	e.do("admin", "POST", "/admin/catalog/modules", imp("widget", moduleV2))

	code, _, _ := e.do("admin", "PATCH", "/admin/catalog/modules/widget/v1", map[string]string{"status": "deprecated"})
	if code != 200 {
		t.Fatalf("deprecate: %d", code)
	}
	if got := e.catalogNames(); len(got) != 1 || got[0] != "widget:v2" {
		t.Fatalf("developers only see the remaining version: %v", got)
	}
	// Existing drafts and changesets still resolve the retired version.
	if code, _, _ := e.do("user", "GET", "/catalog/widget/v1", nil); code != 200 {
		t.Fatalf("a deprecated version stays readable: %d", code)
	}
	_, _, raw := e.do("admin", "GET", "/admin/catalog/modules", nil)
	var rows []map[string]any
	json.Unmarshal(raw, &rows)
	if len(rows) != 2 || rows[0]["version"] != "v2" || rows[1]["version"] != "v1" || rows[1]["status"] != "deprecated" || rows[0]["status"] != "published" {
		t.Fatalf("admin list shows everything, newest first: %v", rows)
	}
	if _, ok := rows[0]["schema"]; ok {
		t.Fatal("the list must not carry whole schemas")
	}

	e.do("admin", "PATCH", "/admin/catalog/modules/widget/v2", map[string]string{"status": "deprecated"})
	if len(e.catalogNames()) != 0 {
		t.Fatal("a module with every version deprecated disappears from the catalog")
	}
	e.do("admin", "PATCH", "/admin/catalog/modules/widget/v1", map[string]string{"status": "published"})
	if got := e.catalogNames(); len(got) != 1 || got[0] != "widget:v1" {
		t.Fatalf("republish brings it back: %v", got)
	}

	for _, tc := range []struct {
		path string
		body any
		want int
	}{
		{"/admin/catalog/modules/widget/v1", map[string]string{"status": "draft"}, 400},
		{"/admin/catalog/modules/widget/v1", `{"status":"published","x":1}`, 400},
		{"/admin/catalog/modules/widget/v9", map[string]string{"status": "deprecated"}, 404},
		{"/admin/catalog/modules/nope/v1", map[string]string{"status": "deprecated"}, 404},
	} {
		if code, _, _ := e.do("admin", "PATCH", tc.path, tc.body); code != tc.want {
			t.Errorf("%s %v: got %d, want %d", tc.path, tc.body, code, tc.want)
		}
	}
	want := "catalog.import module:widget/v1|catalog.import module:widget/v2|catalog.deprecated module:widget/v1|catalog.deprecated module:widget/v2|catalog.published module:widget/v1"
	if got := strings.Join(e.audit.rows, "|"); got != want {
		t.Fatalf("every status change is audited and rejected ones are not:\n got %s\nwant %s", got, want)
	}
}

func TestVersionOrder(t *testing.T) {
	for _, tc := range []struct{ a, b string }{
		{"v1", "v2"}, {"v1", "v1.1"}, {"v1.2", "v1.10"}, {"v1.9.9", "v2"}, {"v1.0.1", "v1.0.2"},
	} {
		if !versionLess(tc.a, tc.b) || versionLess(tc.b, tc.a) {
			t.Errorf("%s must sort before %s", tc.a, tc.b)
		}
	}
	if versionLess("v1", "v1.0") || versionLess("v1.0", "v1") {
		t.Error("v1 and v1.0 are the same version")
	}
}

// --- component config -----------------------------------------------------------

type fakeConfig struct {
	items map[string]string
}

func (f *fakeConfig) ListComponentConfig(context.Context, string) ([]store.ComponentConfig, error) {
	out := []store.ComponentConfig{}
	for a, c := range f.items {
		out = append(out, store.ComponentConfig{Address: a, Color: c})
	}
	return out, nil
}

func (f *fakeConfig) SetComponentConfig(_ context.Context, _ string, address, color, _ string) error {
	if color == "" {
		delete(f.items, address)
		return nil
	}
	f.items[address] = color
	return nil
}

func TestComponentConfig(t *testing.T) {
	fc := &fakeConfig{items: map[string]string{}}
	r := chi.NewRouter()
	r.Get("/projects/{id}/component-config", ComponentConfigList(fc))
	r.Put("/projects/{id}/component-config", ComponentConfigSet(fc))
	put := func(body any) int {
		var buf bytes.Buffer
		switch b := body.(type) {
		case string:
			buf.WriteString(b)
		default:
			json.NewEncoder(&buf).Encode(b)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("PUT", "/projects/p1/component-config", &buf))
		return rec.Code
	}

	if put(map[string]string{"address": "common/rg_app", "color": "cyan"}) != 200 || fc.items["common/rg_app"] != "cyan" {
		t.Fatal("set colour")
	}
	if put(map[string]string{"address": "common/rg_app", "color": ""}) != 200 || len(fc.items) != 0 {
		t.Fatal("an empty colour removes the setting")
	}
	for _, body := range []any{
		map[string]string{"address": "rg_app", "color": "cyan"},
		map[string]string{"address": "../x/rg", "color": "cyan"},
		map[string]string{"address": "common/rg app", "color": "cyan"},
		map[string]string{"address": "common/rg_app", "color": "#ff0000"},
		map[string]string{"address": "common/rg_app", "color": "Cyan"},
		`{"address":"common/rg_app","color":"cyan","extra":1}`, `nope`,
	} {
		if code := put(body); code != 400 {
			t.Errorf("%v: got %d, want 400", body, code)
		}
	}
	if len(fc.items) != 0 {
		t.Fatal("rejected requests must store nothing")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/component-config", nil))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list must be []: %d %q", rec.Code, rec.Body.String())
	}
}
