package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type fakeRaw struct {
	items map[string]store.RawResource
	audit []string
}

// The fake keeps a template under "<project id>/<name>" ("" is the global scope).
func (f *fakeRaw) ListRawResourcesIn(_ context.Context, projectID string) ([]store.RawResource, error) {
	out := []store.RawResource{}
	for _, v := range f.items {
		if v.ProjectID == projectID {
			out = append(out, v)
		}
	}
	return out, nil
}
func (f *fakeRaw) CreateRawResource(_ context.Context, r store.RawResource) error {
	if _, ok := f.items[r.ProjectID+"/"+r.Name]; ok {
		return store.ErrRawResourceExists
	}
	f.items[r.ProjectID+"/"+r.Name] = r
	return nil
}
func (f *fakeRaw) UpdateRawResource(_ context.Context, r store.RawResource) error {
	if _, ok := f.items[r.ProjectID+"/"+r.Name]; !ok {
		return pgx.ErrNoRows
	}
	f.items[r.ProjectID+"/"+r.Name] = r
	return nil
}
func (f *fakeRaw) DeleteRawResourceIn(_ context.Context, projectID, n string) error {
	if _, ok := f.items[projectID+"/"+n]; !ok {
		return pgx.ErrNoRows
	}
	delete(f.items, projectID+"/"+n)
	return nil
}
func (f *fakeRaw) LogAudit(_ context.Context, _ string, action, res string, _ any) error {
	f.audit = append(f.audit, action+" "+res)
	return nil
}

func rawRouter(f *fakeRaw) http.Handler {
	a := &RawResourceAPI{Store: f}
	r := chi.NewRouter()
	r.Get("/catalog/resources", a.List())
	r.Post("/admin/resources", a.Create())
	r.Put("/admin/resources/{name}", a.Update())
	r.Delete("/admin/resources/{name}", a.Delete())
	p := &RawResourceAPI{Store: f, Project: true}
	r.Get("/projects/{id}/catalog/resources", p.List())
	r.Post("/projects/{id}/catalog/resources", p.Create())
	r.Put("/projects/{id}/catalog/resources/{name}", p.Update())
	r.Delete("/projects/{id}/catalog/resources/{name}", p.Delete())
	return r
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

const goodRaw = `{"name":"resource-group","title":"Resource group","resource_type":"azurerm_resource_group","default_values":{"location":"southeastasia"}}`

func TestRawResources_CreateListUpdateDelete(t *testing.T) {
	f := &fakeRaw{items: map[string]store.RawResource{}}
	h := rawRouter(f)

	if rec := do(h, "POST", "/admin/resources", goodRaw); rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/admin/resources", goodRaw); rec.Code != http.StatusConflict {
		t.Fatalf("a second create with the same name must be 409, got %d", rec.Code)
	}
	rec := do(h, "GET", "/catalog/resources", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "azurerm_resource_group") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	upd := `{"title":"RG","resource_type":"azurerm_resource_group","description":"x"}`
	if rec := do(h, "PUT", "/admin/resources/resource-group", upd); rec.Code != 200 || f.items["/resource-group"].Title != "RG" {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if f.items["/resource-group"].DefaultValues == nil {
		t.Fatal("default values must never be nil")
	}
	if rec := do(h, "PUT", "/admin/resources/nope", upd); rec.Code != http.StatusNotFound {
		t.Fatalf("update of an unknown name: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/admin/resources/resource-group", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/admin/resources/resource-group", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d", rec.Code)
	}
	if len(f.audit) != 3 {
		t.Fatalf("every change is audited, got %v", f.audit)
	}
}

func TestRawResources_Validation(t *testing.T) {
	h := rawRouter(&fakeRaw{items: map[string]store.RawResource{}})
	for name, body := range map[string]string{
		"bad name":      `{"name":"Bad Name","title":"t","resource_type":"azurerm_x"}`,
		"no title":      `{"name":"ok-name","title":" ","resource_type":"azurerm_x"}`,
		"module-ish":    `{"name":"ok-name","title":"t","resource_type":"Resource"}`,
		"unknown field": `{"name":"ok-name","title":"t","resource_type":"azurerm_x","nope":1}`,
		"not json":      `{`,
		"bad folder":    `{"name":"ok-name","title":"t","resource_type":"azurerm_x","target_folder":"../x"}`,
	} {
		if rec := do(h, "POST", "/admin/resources", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d %s", name, rec.Code, rec.Body)
		}
	}
}
