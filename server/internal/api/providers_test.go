package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type memProviders struct {
	items map[string]store.Provider
	inUse map[string]bool
	audit []string
}

func (m *memProviders) ListProviders(context.Context) ([]store.Provider, error) {
	out := []store.Provider{}
	for _, p := range m.items {
		out = append(out, p)
	}
	return out, nil
}
func (m *memProviders) CreateProvider(_ context.Context, p store.Provider) error {
	if _, ok := m.items[p.ID]; ok {
		return store.ErrProviderExists
	}
	m.items[p.ID] = p
	return nil
}
func (m *memProviders) UpdateProvider(_ context.Context, p store.Provider) error {
	if _, ok := m.items[p.ID]; !ok {
		return pgx.ErrNoRows
	}
	m.items[p.ID] = p
	return nil
}
func (m *memProviders) DeleteProvider(_ context.Context, id string) error {
	if m.inUse[id] {
		return store.ErrProviderInUse
	}
	if _, ok := m.items[id]; !ok {
		return pgx.ErrNoRows
	}
	delete(m.items, id)
	return nil
}
func (m *memProviders) LogAudit(_ context.Context, _, action, _ string, _ any) error {
	m.audit = append(m.audit, action)
	return nil
}

func providerRouter(m *memProviders) http.Handler {
	a := &ProviderAPI{Store: m}
	r := chi.NewRouter()
	r.Get("/api/catalog/providers", a.List())
	r.Get("/api/catalog/meta", Meta())
	r.Post("/api/admin/providers", a.Create())
	r.Put("/api/admin/providers/{id}", a.Update())
	r.Delete("/api/admin/providers/{id}", a.Delete())
	return r
}

func doJSON(h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, rd))
	return rec
}

func TestProviderRegistryValidatesAndIsAudited(t *testing.T) {
	m := &memProviders{items: map[string]store.Provider{}, inUse: map[string]bool{}}
	h := providerRouter(m)
	svg := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg xmlns='http://www.w3.org/2000/svg'/>"))
	good := map[string]any{"id": "aws", "name": "Amazon Web Services", "source": "hashicorp/aws", "logo": svg, "versions": []string{"~> 5.0"}, "block": "provider \"aws\" {\n  region = \"us-east-1\"\n}"}

	if rec := doJSON(h, "POST", "/api/admin/providers", good); rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(h, "POST", "/api/admin/providers", good); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d", rec.Code)
	}
	for name, mod := range map[string]func(map[string]any){
		"id":       func(b map[string]any) { b["id"] = "AWS" },
		"source":   func(b map[string]any) { b["source"] = "no-slash" },
		"logo url": func(b map[string]any) { b["logo"] = "https://evil.test/x.svg" },
		"logo html": func(b map[string]any) {
			b["logo"] = "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte("<script>"))
		},
		"version": func(b map[string]any) { b["versions"] = []string{"latest"} },
		"name":    func(b map[string]any) { b["name"] = " " },
	} {
		bad := map[string]any{}
		for k, v := range good {
			bad[k] = v
		}
		bad["id"] = "other"
		mod(bad)
		if rec := doJSON(h, "POST", "/api/admin/providers", bad); rec.Code != http.StatusBadRequest {
			t.Errorf("%s accepted: %d", name, rec.Code)
		}
	}
	// Update never renames; delete is refused while raw resources use it.
	good["name"] = "AWS"
	if rec := doJSON(h, "PUT", "/api/admin/providers/aws", good); rec.Code != http.StatusOK || m.items["aws"].Name != "AWS" {
		t.Fatalf("update: %d", rec.Code)
	}
	if rec := doJSON(h, "PUT", "/api/admin/providers/missing", good); rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("update missing: %d", rec.Code)
	}
	m.inUse["aws"] = true
	if rec := doJSON(h, "DELETE", "/api/admin/providers/aws", nil); rec.Code != http.StatusConflict {
		t.Fatalf("delete in use: %d", rec.Code)
	}
	m.inUse["aws"] = false
	if rec := doJSON(h, "DELETE", "/api/admin/providers/aws", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	want := []string{"provider.create", "provider.update", "provider.delete"}
	if len(m.audit) < 3 || m.audit[0] != want[0] || m.audit[1] != want[1] || m.audit[len(m.audit)-1] != want[2] {
		t.Fatalf("audit = %v", m.audit)
	}
}

func TestMetaServesTheCategories(t *testing.T) {
	rec := doJSON(providerRouter(&memProviders{}), "GET", "/api/catalog/meta", nil)
	var out struct{ Categories []Category }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Categories) < 5 || out.Categories[0].ID != "compute" {
		t.Fatalf("%v %s", err, rec.Body)
	}
	if !ValidCategory("") || !ValidCategory("network") || ValidCategory("nonsense") {
		t.Error("ValidCategory")
	}
}

func TestRawResourceProviderCategoryAndVersionRange(t *testing.T) {
	base := rawBody{Name: "rg", Title: "RG", ResourceType: "azurerm_resource_group"}
	if msg := base.check(); msg != "" {
		t.Fatalf("a plain template: %s", msg)
	}
	if base.provider() != "azurerm" {
		t.Error("the provider is taken from the resource type")
	}
	ok := base
	ok.Provider, ok.Category, ok.MinVersion, ok.MaxVersion = "azurerm", "compute", "3.0", "4.9"
	if msg := ok.check(); msg != "" {
		t.Fatalf("a full template: %s", msg)
	}
	for name, mod := range map[string]func(*rawBody){
		"other provider": func(b *rawBody) { b.Provider = "aws" },
		"category":       func(b *rawBody) { b.Category = "nonsense" },
		"bound":          func(b *rawBody) { b.MinVersion = "3" },
		"order":          func(b *rawBody) { b.MinVersion, b.MaxVersion = "4.0", "3.0" },
	} {
		bad := ok
		mod(&bad)
		if bad.check() == "" {
			t.Errorf("%s accepted", name)
		}
	}
}
