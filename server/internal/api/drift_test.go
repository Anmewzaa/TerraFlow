package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type memDrift struct {
	mu        sync.Mutex
	baseline  *store.DriftBaseline
	dismissed map[string]store.Dismissal
}

func (m *memDrift) SetDriftBaseline(_ context.Context, _ string, files map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseline = &store.DriftBaseline{Files: files}
	return nil
}
func (m *memDrift) GetDriftBaseline(context.Context, string) (*store.DriftBaseline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.baseline, nil
}
func (m *memDrift) DismissDrift(_ context.Context, _, key, note, by string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dismissed[key] = store.Dismissal{Key: key, Note: note, By: by}
	return nil
}
func (m *memDrift) RestoreDrift(_ context.Context, _, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.dismissed, key)
	return nil
}
func (m *memDrift) ListDriftDismissals(context.Context, string) ([]store.Dismissal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.Dismissal
	for _, d := range m.dismissed {
		out = append(out, d)
	}
	return out, nil
}

const driftMain = "module \"many\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  for_each = toset([\"a\"])\n  name = \"n\"\n}\n" +
	"module \"rg\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name = \"rg\"\n  location = \"japaneast\"\n}\n"

func TestRepositoryDrift(t *testing.T) {
	repoNow := gitprovider.NewMemory(map[string]string{"net/main.tf": driftMain})
	repos := Repos(func(context.Context, store.Project) (gitprovider.Source, error) { return repoNow, nil })
	st := &memDrift{dismissed: map[string]store.Dismissal{}}
	team := &TeamAPI{Store: &bbStore{roles: map[string]string{"owner": store.RoleOwner, "contrib": store.RoleContributor}}}
	settings := SettingsLoader(func(context.Context, string) *hcl.ProjectSettings {
		return &hcl.ProjectSettings{Variables: []hcl.ProjectVariable{{Name: "location", Values: map[string]string{"dev": "southeastasia"}}}}
	})
	d := &DriftAPI{Team: team, Store: st, Repos: repos, Settings: settings,
		Project: func(_ context.Context, id string) (store.Project, error) {
			return store.Project{ID: id, Slug: "demo"}, nil
		}}
	r := chi.NewRouter()
	r.Get("/projects/{id}/drift", d.List())
	r.Post("/projects/{id}/drift/dismiss", d.Dismiss())
	r.Delete("/projects/{id}/drift/dismiss", d.Dismiss())
	r.Post("/projects/{id}/drift/baseline", d.Baseline())
	do := func(who, method, target, body string) (int, map[string]any) {
		req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{OID: who}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var m map[string]any
		json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}
	kinds := func(m map[string]any, field string) map[string]int {
		out := map[string]int{}
		for _, x := range m[field].([]any) {
			out[x.(map[string]any)["kind"].(string)]++
		}
		return out
	}

	code, m := do("owner", "GET", "/projects/p/drift?env=dev", "")
	if code != 200 || m["has_baseline"] != false {
		t.Fatalf("%d %v", code, m)
	}
	if k := kinds(m, "items"); k["unrepresented"] != 1 || k["variable_differs"] != 1 || len(k) != 2 {
		t.Fatalf("a for_each block and a value that differs from the variable, and no file drift without a baseline: %v", k)
	}

	// Accept the state as it is; then a commit lands on the base branch.
	if code, _ := do("contrib", "POST", "/projects/p/drift/baseline?env=dev", ""); code != http.StatusForbidden {
		t.Fatalf("a contributor cannot take a baseline: %d", code)
	}
	if code, _ := do("owner", "POST", "/projects/p/drift/baseline?env=dev", ""); code != http.StatusNoContent {
		t.Fatalf("baseline = %d", code)
	}
	if _, m = do("owner", "GET", "/projects/p/drift?env=dev", ""); m["has_baseline"] != true || kinds(m, "items")["file_changed"] != 0 {
		t.Fatalf("nothing changed since the baseline: %v", m)
	}
	repoNow = gitprovider.NewMemory(map[string]string{"net/main.tf": driftMain + "# edited\n", "net/new.tf": "# new\n"})
	_, m = do("owner", "GET", "/projects/p/drift?env=dev", "")
	if k := kinds(m, "items"); k["file_changed"] != 1 || k["file_added"] != 1 {
		t.Fatalf("a commit that landed after the baseline: %v", k)
	}

	// Dismiss one item with a note; it moves to the dismissed list. A contributor cannot.
	var key string
	for _, x := range m["items"].([]any) {
		if x.(map[string]any)["kind"] == "unrepresented" {
			key = x.(map[string]any)["key"].(string)
		}
	}
	body, _ := json.Marshal(map[string]string{"key": key, "note": "managed by hand on purpose"})
	if code, _ := do("contrib", "POST", "/projects/p/drift/dismiss", string(body)); code != http.StatusForbidden {
		t.Fatalf("a contributor cannot dismiss: %d", code)
	}
	if code, _ := do("owner", "POST", "/projects/p/drift/dismiss", string(body)); code != http.StatusNoContent {
		t.Fatalf("dismiss = %d", code)
	}
	_, m = do("owner", "GET", "/projects/p/drift?env=dev", "")
	if kinds(m, "items")["unrepresented"] != 0 || kinds(m, "dismissed")["unrepresented"] != 1 {
		t.Fatalf("a dismissed item is listed apart: %v", m)
	}
	if d := m["dismissed"].([]any)[0].(map[string]any); d["note"] != "managed by hand on purpose" {
		t.Fatalf("the note is kept: %v", d)
	}
	if code, _ := do("owner", "DELETE", "/projects/p/drift/dismiss?key="+key, ""); code != http.StatusNoContent {
		t.Fatalf("restore = %d", code)
	}
	if _, m = do("owner", "GET", "/projects/p/drift?env=dev", ""); kinds(m, "items")["unrepresented"] != 1 {
		t.Fatalf("a restored item shows again: %v", m)
	}
}
