package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type memGallery struct {
	mu      sync.Mutex
	list    map[string]store.GalleryTemplate
	n       int
	imports []string
	audits  []string
}

func (m *memGallery) CreateGalleryTemplate(_ context.Context, g store.GalleryTemplate) (store.GalleryTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.list {
		if o.Author == g.Author && o.Name == g.Name {
			return store.GalleryTemplate{}, store.ErrGalleryExists
		}
	}
	m.n++
	g.ID, g.Version, g.CreatedAt, g.UpdatedAt = fmt.Sprint("g", m.n), 1, time.Now(), time.Now()
	m.list[g.ID] = g
	return g, nil
}
func (m *memGallery) ListGalleryTemplates(_ context.Context, f store.GalleryFilter) ([]store.GalleryTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []store.GalleryTemplate{}
	for _, g := range m.list {
		vis := f.Admin || g.Author == f.Viewer || (g.Scope == store.GalleryPublic && g.Status == store.GalleryApproved)
		if f.Mine && g.Author != f.Viewer || !vis || f.Status != "" && g.Status != f.Status {
			continue
		}
		if f.Query != "" && !strings.Contains(strings.ToLower(g.Name), strings.ToLower(f.Query)) {
			continue
		}
		g.Template = nil
		out = append(out, g)
	}
	return out, nil
}
func (m *memGallery) GetGalleryTemplate(_ context.Context, id string) (store.GalleryTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.list[id]; ok {
		return g, nil
	}
	return store.GalleryTemplate{}, pgx.ErrNoRows
}
func (m *memGallery) UpdateGalleryTemplate(_ context.Context, id string, n store.GalleryTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.list[id]
	if !ok {
		return pgx.ErrNoRows
	}
	for _, o := range m.list {
		if o.ID != id && o.Author == g.Author && o.Name == n.Name {
			return store.ErrGalleryExists
		}
	}
	g.Name, g.Description, g.Category, g.Scope, g.Status, g.Template, g.Modules = n.Name, n.Description, n.Category, n.Scope, n.Status, n.Template, n.Modules
	g.Version++
	m.list[id] = g
	return nil
}
func (m *memGallery) SetGalleryStatus(_ context.Context, id, status, note, by string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.list[id]
	if !ok {
		return pgx.ErrNoRows
	}
	g.Status, g.ReviewNote, g.ReviewedBy = status, note, by
	m.list[id] = g
	return nil
}
func (m *memGallery) DeleteGalleryTemplate(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.list[id]; !ok {
		return pgx.ErrNoRows
	}
	delete(m.list, id)
	return nil
}
func (m *memGallery) RecordGalleryImport(_ context.Context, id, pid, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.imports = append(m.imports, id+"@"+pid)
	return nil
}
func (m *memGallery) LogAudit(_ context.Context, _, action, _ string, _ any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audits = append(m.audits, action)
	return nil
}

// stubCatalog answers VisibleModules with a fixed list.
type stubCatalog struct{ names []string }

func (s stubCatalog) VisibleModules(context.Context, string) ([]catalog.Module, map[string]bool, error) {
	var out []catalog.Module
	for _, n := range s.names {
		out = append(out, catalog.Module{Name: n, Version: "v1", SchemaJSON: json.RawMessage(`{}`)})
	}
	return out, nil, nil
}

type galleryRig struct {
	t   *testing.T
	g   *memGallery
	st  *memStacks
	rt  chi.Router
	api *GalleryAPI
}

func newGalleryRig(t *testing.T, modules ...string) *galleryRig {
	g := &memGallery{list: map[string]store.GalleryTemplate{}}
	st := &memStacks{list: map[string]store.StackTemplate{}}
	team := &TeamAPI{Store: &bbStore{roles: map[string]string{"alice": store.RoleOwner, "contrib": store.RoleContributor}}}
	a := &GalleryAPI{Store: g, Team: team, Stacks: st, Catalog: stubCatalog{modules}}
	r := chi.NewRouter()
	r.Get("/templates", a.List())
	r.Post("/templates", a.Create())
	r.Get("/templates/{gid}", a.Get())
	r.Put("/templates/{gid}", a.Update())
	r.Delete("/templates/{gid}", a.Delete())
	r.Post("/templates/{gid}/withdraw", a.Withdraw())
	r.Post("/templates/{gid}/fork", a.Fork())
	r.Get("/templates/{gid}/export", a.Export())
	r.Get("/admin/templates", a.AdminList())
	r.Post("/admin/templates/{gid}/review", a.Review())
	r.Post("/projects/{id}/stack-templates/import", a.Import())
	r.Get("/projects/{id}/stack-templates/{tid}/export", a.ExportProject())
	return &galleryRig{t: t, g: g, st: st, rt: r, api: a}
}

func (g *galleryRig) do(who, method, target, body string) (int, map[string]any, []byte) {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	u := &auth.User{OID: who, Name: who}
	if who == "root" {
		u.Roles = []string{"devops"}
	}
	req = req.WithContext(auth.WithUser(req.Context(), u))
	w := httptest.NewRecorder()
	g.rt.ServeHTTP(w, req)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m, w.Body.Bytes()
}

const galleryDoc = `{"name":"x","nodes":[
 {"id":"s","module_name":"subnet","module_version":"v1","instance_name":"sn","target_folder":"vnet","values":{"name":"snet-app"}},
 {"id":"p","module_name":"private-endpoint","module_version":"v1","instance_name":"pe","target_folder":"vnet","parent_id":"s","values":{"name":"pe-kv"}}
],"edges":[{"id":"e","from":"s","to":"p","kind":"nesting"}],"slots":[],"params":[]}`

func (g *galleryRig) share(who, scope, name string) (int, map[string]any) {
	c, m, _ := g.do(who, "POST", "/templates", fmt.Sprintf(`{"name":%q,"description":"d","category":"Network","scope":%q,"template":%s}`, name, scope, galleryDoc))
	return c, m
}

func TestGalleryVisibilityPerScope(t *testing.T) {
	g := newGalleryRig(t)
	c, priv := g.share("alice", "private", "mine")
	if c != 201 || priv["status"] != "approved" || priv["scope"] != "private" {
		t.Fatalf("a private template is live at once: %d %v", c, priv)
	}
	c, pub := g.share("alice", "public", "shared")
	if c != 201 || pub["status"] != "pending" {
		t.Fatalf("a public template waits for an admin: %d %v", c, pub)
	}
	privID, pubID := priv["id"].(string), pub["id"].(string)

	listed := func(who string) int {
		_, _, raw := g.do(who, "GET", "/templates", "")
		var l []map[string]any
		_ = json.Unmarshal(raw, &l)
		return len(l)
	}
	if listed("alice") != 2 || listed("bob") != 0 {
		t.Fatalf("the author sees both, a stranger neither: %d %d", listed("alice"), listed("bob"))
	}
	for _, id := range []string{privID, pubID} {
		if c, _, _ := g.do("bob", "GET", "/templates/"+id, ""); c != 404 {
			t.Fatalf("a stranger gets 404, not 403, so ids are not confirmed: %d", c)
		}
		if c, _, _ := g.do("bob", "GET", "/templates/"+id+"/export", ""); c != 404 {
			t.Fatalf("export follows the same rule: %d", c)
		}
		if c, _, _ := g.do("root", "GET", "/templates/"+id, ""); c != 200 {
			t.Fatalf("an admin sees any template: %d", c)
		}
	}

	// Review: approve the public one.
	if c, _, _ := g.do("bob", "POST", "/admin/templates/"+pubID+"/review", `{"action":"approve"}`); c != 404 {
		t.Fatalf("a stranger cannot even see it: %d", c)
	}
	if c, m, _ := g.do("root", "POST", "/admin/templates/"+pubID+"/review", `{"action":"approve"}`); c != 200 || m["status"] != "approved" {
		t.Fatalf("approve: %d %v", c, m)
	}
	if listed("bob") != 1 {
		t.Fatal("an approved public template is on every signed-in person's shelf")
	}
	if c, _, _ := g.do("bob", "GET", "/templates/"+pubID, ""); c != 200 {
		t.Fatalf("%d", c)
	}
	if c, _, _ := g.do("bob", "GET", "/templates/"+privID, ""); c != 404 {
		t.Fatalf("private stays private: %d", c)
	}
	// Hide, with a reason.
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+pubID+"/review", `{"action":"hide"}`); c != 400 {
		t.Fatalf("a hide needs a note: %d", c)
	}
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+pubID+"/review", `{"action":"hide","note":"contains a customer name"}`); c != 200 {
		t.Fatalf("hide: %d", c)
	}
	if listed("bob") != 0 {
		t.Fatal("a hidden template leaves the shelf")
	}
}

func TestGalleryReviewRules(t *testing.T) {
	g := newGalleryRig(t)
	_, pub := g.share("alice", "public", "shared")
	id := pub["id"].(string)
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"reject"}`); c != 400 {
		t.Fatalf("a rejection needs a note: %d", c)
	}
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"hide","note":"x"}`); c != 409 {
		t.Fatalf("only an approved template is hidden: %d", c)
	}
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"explode"}`); c != 400 {
		t.Fatalf("unknown action: %d", c)
	}
	if c, m, _ := g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"reject","note":"too specific"}`); c != 200 || m["review_note"] != "too specific" {
		t.Fatalf("reject: %d %v", c, m)
	}
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"approve"}`); c != 409 {
		t.Fatalf("a rejected template is edited and resubmitted, not approved: %d", c)
	}
	// The author edits it: back to pending, version up.
	c, m, _ := g.do("alice", "PUT", "/templates/"+id, `{"description":"better"}`)
	if c != 200 || m["status"] != "pending" || m["version"].(float64) != 2 {
		t.Fatalf("edit: %d %v", c, m)
	}
	if c, _, _ := g.do("root", "PUT", "/templates/"+id, `{"description":"mine now"}`); c != 403 {
		t.Fatalf("an admin reviews but does not edit another person's template: %d", c)
	}
	// Private templates are not reviewed.
	_, priv := g.share("alice", "private", "own")
	if c, _, _ := g.do("root", "POST", "/admin/templates/"+priv["id"].(string)+"/review", `{"action":"approve"}`); c != 409 {
		t.Fatalf("%d", c)
	}
	// An admin's own public template is live.
	if c, m := g.share("root", "public", "admin shared"); c != 201 || m["status"] != "approved" {
		t.Fatalf("%d %v", c, m)
	}
	// The queue.
	_, _, raw := g.do("root", "GET", "/admin/templates?status=pending", "")
	var q []map[string]any
	_ = json.Unmarshal(raw, &q)
	if len(q) != 1 {
		t.Fatalf("queue: %s", raw)
	}
}

func TestGalleryRefusesHostileAndLeakingTemplates(t *testing.T) {
	g := newGalleryRig(t)
	post := func(scope, doc string) (int, map[string]any) {
		c, m, _ := g.do("alice", "POST", "/templates", fmt.Sprintf(`{"name":"t","scope":%q,"template":%s}`, scope, doc))
		return c, m
	}
	if c, m := post("private", `{"nodes":[]}`); c != 422 || m["problems"] == nil {
		t.Fatalf("an empty template: %d %v", c, m)
	}
	hostile := strings.Replace(galleryDoc, `"snet-app"`, `{"$expr":"var.a }\nresource \"null_resource\" \"x\" {"}`, 1)
	if c, _ := post("private", hostile); c != 422 {
		t.Fatalf("an expression that closes its block: %d", c)
	}
	traversal := strings.Replace(galleryDoc, `"target_folder":"vnet"`, `"target_folder":"../../etc"`, 1)
	if c, _ := post("private", traversal); c != 422 {
		t.Fatalf("a folder outside the project: %d", c)
	}
	if c, _ := post("private", `not json`); c != 400 {
		t.Fatalf("not json: %d", c)
	}
	secret := strings.Replace(galleryDoc, `"name":"snet-app"`, `"name":"snet-app","admin_password":"hunter2hunter2"`, 1)
	c, m := post("private", secret)
	if c != 422 || m["leaks"] == nil || strings.Contains(fmt.Sprint(m), "hunter2") {
		t.Fatalf("a secret is refused even in private and never echoed: %d %v", c, m)
	}
	withID := strings.Replace(galleryDoc, `"name":"snet-app"`, `"name":"snet-app","scope":"/subscriptions/00000000-1111-2222-3333-444444444444"`, 1)
	if c, _ := post("private", withID); c != 201 {
		t.Fatalf("an id is only a warning for a private template: %d", c)
	}
	g2 := newGalleryRig(t)
	if c, _, _ := g2.do("alice", "POST", "/templates", fmt.Sprintf(`{"name":"t","scope":"public","template":%s}`, withID)); c != 422 {
		t.Fatalf("a public template holds no subscription id: %d", c)
	}
	if c, _, _ := g.do("alice", "POST", "/templates", `{"name":"t","scope":"world","template":`+galleryDoc+`}`); c != 400 {
		t.Fatalf("scope: %d", c)
	}
	if c, _, _ := g.do("alice", "POST", "/templates", `{"name":"t","category":"<script>","template":`+galleryDoc+`}`); c != 400 {
		t.Fatalf("category: %d", c)
	}
	if c, _ := post("private", galleryDoc); c != 409 {
		t.Fatalf("the same author and name: %d", c)
	}
}

func TestGalleryShareFromProjectNeedsCoOwner(t *testing.T) {
	g := newGalleryRig(t)
	if _, err := g.st.CreateStackTemplate(context.Background(), "p1", "from project", "alice", json.RawMessage(galleryDoc)); err != nil {
		t.Fatal(err)
	}
	body := `{"scope":"private","project_id":"p1","stack_template_id":"t1"}`
	if c, _, _ := g.do("contrib", "POST", "/templates", body); c != 403 {
		t.Fatalf("a contributor does not share a project's template: %d", c)
	}
	c, m, _ := g.do("alice", "POST", "/templates", body)
	if c != 201 || m["name"] != "from project" {
		t.Fatalf("share: %d %v", c, m)
	}
	if c, _, _ := g.do("alice", "POST", "/templates", `{"scope":"private","project_id":"p1","stack_template_id":"nope"}`); c != 404 {
		t.Fatalf("%d", c)
	}
}

func TestGalleryWithdrawForkDelete(t *testing.T) {
	g := newGalleryRig(t)
	_, pub := g.share("alice", "public", "shared")
	id := pub["id"].(string)
	g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"approve"}`)

	c, fork, _ := g.do("bob", "POST", "/templates/"+id+"/fork", "")
	if c != 201 || fork["name"] != "shared (copy)" || fork["scope"] != "private" || fork["author"] != "bob" {
		t.Fatalf("fork: %d %v", c, fork)
	}
	if c, f2, _ := g.do("bob", "POST", "/templates/"+id+"/fork", ""); c != 201 || f2["name"] != "shared (copy 2)" {
		t.Fatalf("a second fork gets its own name: %d %v", c, f2)
	}
	if c, _, _ := g.do("bob", "POST", "/templates/"+id+"/withdraw", ""); c != 403 {
		t.Fatalf("only the author withdraws: %d", c)
	}
	if c, _, _ := g.do("alice", "POST", "/templates/"+id+"/withdraw", ""); c != 204 {
		t.Fatalf("withdraw: %d", c)
	}
	if c, _, _ := g.do("bob", "GET", "/templates/"+id, ""); c != 404 {
		t.Fatalf("a withdrawn template is gone for others: %d", c)
	}
	if c, _, _ := g.do("bob", "POST", "/templates/"+id+"/fork", ""); c != 404 {
		t.Fatalf("and cannot be forked: %d", c)
	}
	if c, _, _ := g.do("bob", "DELETE", "/templates/"+id, ""); c != 404 {
		t.Fatalf("%d", c)
	}
	if c, _, _ := g.do("root", "DELETE", "/templates/"+id, ""); c != 204 {
		t.Fatalf("an admin removes any template: %d", c)
	}
	if c, _, _ := g.do("alice", "DELETE", "/templates/"+fork["id"].(string), ""); c != 404 {
		t.Fatalf("someone else's private fork is not alice's to delete: %d", c)
	}
	if !contains(g.g.audits, "gallery.template.withdraw") || !contains(g.g.audits, "gallery.template.delete") || !contains(g.g.audits, "gallery.template.approve") {
		t.Fatalf("publish, review, withdraw and delete are audited: %v", g.g.audits)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func TestImportFromGalleryAndFile(t *testing.T) {
	g := newGalleryRig(t, "subnet", "private-endpoint")
	_, pub := g.share("alice", "public", "shared")
	id := pub["id"].(string)
	g.do("root", "POST", "/admin/templates/"+id+"/review", `{"action":"approve"}`)

	imp := func(who, body string) (int, map[string]any) {
		c, m, _ := g.do(who, "POST", "/projects/p2/stack-templates/import", body)
		return c, m
	}
	if c, _ := imp("contrib", `{"gallery_id":"`+id+`"}`); c != 403 {
		t.Fatalf("a contributor does not import: %d", c)
	}
	if c, _ := imp("alice", `{}`); c != 400 {
		t.Fatalf("nothing to import: %d", c)
	}
	if c, _ := imp("alice", `{"gallery_id":"`+id+`","document":{"format":"x"}}`); c != 400 {
		t.Fatalf("one source only: %d", c)
	}
	c, m := imp("alice", `{"gallery_id":"`+id+`"}`)
	if c != 201 || m["name"] != "shared" {
		t.Fatalf("import: %d %v", c, m)
	}
	if len(g.g.imports) != 1 || g.g.imports[0] != id+"@p2" {
		t.Fatalf("the import is counted: %v", g.g.imports)
	}
	if c, _ := imp("alice", `{"gallery_id":"`+id+`"}`); c != 409 {
		t.Fatalf("the name is taken in the project: %d", c)
	}
	if c, m := imp("alice", `{"gallery_id":"`+id+`","name":"shared two"}`); c != 201 || m["name"] != "shared two" {
		t.Fatalf("a new name: %d %v", c, m)
	}
	// A private template of someone else cannot be imported.
	_, priv := g.share("bob", "private", "bobs")
	if c, _ := imp("alice", `{"gallery_id":"`+priv["id"].(string)+`"}`); c != 404 {
		t.Fatalf("%d", c)
	}

	// A file: export and import it into another project.
	_, _, file := g.do("alice", "GET", "/templates/"+id+"/export", "")
	var f map[string]any
	if json.Unmarshal(file, &f) != nil || f["format"] != ExportFormat {
		t.Fatalf("export: %s", file)
	}
	if c, _ := imp("alice", `{"document":`+string(file)+`,"name":"from file"}`); c != 201 {
		t.Fatalf("file import: %d", c)
	}
	if c, _ := imp("alice", `{"document":{"format":"other","version":1,"template":{}}}`); c != 400 {
		t.Fatalf("a file of another kind: %d", c)
	}
	// A file that was edited by hand into something hostile is held to the gate.
	bad := strings.Replace(string(file), `"snet-app"`, `{"$expr":"var.a }\nresource \"null_resource\" \"x\" {"}`, 1)
	if c, _ := imp("alice", `{"document":`+bad+`,"name":"bad one"}`); c != 422 {
		t.Fatalf("a hostile file: %d", c)
	}
	leak := strings.Replace(string(file), `"snet-app"`, `"snet-app","client_secret":"literal-secret"`, 1)
	if c, _ := imp("alice", `{"document":`+leak+`,"name":"leaky"}`); c != 422 {
		t.Fatalf("a file with a secret: %d", c)
	}
	// Project export is the same file format.
	if c, _, raw := g.do("alice", "GET", "/projects/p2/stack-templates/t1/export", ""); c != 200 || !strings.Contains(string(raw), ExportFormat) {
		t.Fatalf("project export: %d %s", c, raw)
	}
}

func TestImportListsMissingModulesAndStoresNothing(t *testing.T) {
	g := newGalleryRig(t, "subnet") // private-endpoint is not in this project's catalog
	_, pub := g.share("alice", "private", "needs pe")
	c, m, _ := g.do("alice", "POST", "/projects/p2/stack-templates/import", `{"gallery_id":"`+pub["id"].(string)+`"}`)
	missing, _ := m["missing"].([]any)
	if c != 422 || len(missing) != 1 || missing[0] != "private-endpoint@v1" {
		t.Fatalf("the answer names the missing module: %d %v", c, m)
	}
	if l, _ := g.st.ListStackTemplates(context.Background(), "p2"); len(l) != 0 {
		t.Fatal("nothing is stored")
	}
	if len(g.g.imports) != 0 {
		t.Fatal("a refused import is not counted")
	}
}

func TestGalleryBodyLimits(t *testing.T) {
	g := newGalleryRig(t)
	huge := `{"name":"t","template":{"name":"t","nodes":[{"id":"a","instance_name":"a","values":{"x":"` + strings.Repeat("a", MaxCanvasBytes+10) + `"}}]}}`
	if c, _, _ := g.do("alice", "POST", "/templates", huge); c == http.StatusCreated {
		t.Fatalf("a body over the limit is refused: %d", c)
	}
}
