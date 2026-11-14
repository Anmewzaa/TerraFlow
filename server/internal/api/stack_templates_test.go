package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type memStacks struct {
	mu   sync.Mutex
	list map[string]store.StackTemplate
	n    int
}

func (m *memStacks) CreateStackTemplate(_ context.Context, pid, name, by string, doc json.RawMessage) (store.StackTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.list {
		if t.ProjectID == pid && t.Name == name {
			return store.StackTemplate{}, store.ErrStackTemplateExists
		}
	}
	m.n++
	t := store.StackTemplate{ID: fmt.Sprint("t", m.n), ProjectID: pid, Name: name, CreatedBy: by, CreatedAt: time.Now(), Template: doc}
	m.list[t.ID] = t
	return t, nil
}
func (m *memStacks) ListStackTemplates(_ context.Context, pid string) ([]store.StackTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.StackTemplate
	for _, t := range m.list {
		if t.ProjectID == pid {
			t.Template = nil
			out = append(out, t)
		}
	}
	return out, nil
}
func (m *memStacks) GetStackTemplate(_ context.Context, pid, id string) (store.StackTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.list[id]; ok && t.ProjectID == pid {
		return t, nil
	}
	return store.StackTemplate{}, pgx.ErrNoRows
}
func (m *memStacks) DeleteStackTemplate(_ context.Context, pid, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.list[id]; ok && t.ProjectID == pid {
		delete(m.list, id)
		return nil
	}
	return pgx.ErrNoRows
}

const stackCanvas = `{"nodes":[
 {"id":"v","module_name":"virtual-network","module_version":"v1","instance_name":"vnet","target_folder":"vnet","values":{"name":"vnet-a"}},
 {"id":"s","module_name":"subnet","module_version":"v1","instance_name":"sn","target_folder":"vnet","parent_id":"v","values":{"name":"snet-app","size":"24"}},
 {"id":"p","module_name":"private-endpoint","module_version":"v1","instance_name":"pe","target_folder":"vnet","parent_id":"s","values":{"name":"pe-kv","private_connection_resource_id":{"$expr":"module.kv.id"}}}
],"edges":[{"id":"e","from":"s","to":"p","kind":"nesting"}]}`

func TestStackTemplatesFromSelectionToApplication(t *testing.T) {
	st := &memStacks{list: map[string]store.StackTemplate{}}
	team := &TeamAPI{Store: &bbStore{roles: map[string]string{"owner": store.RoleOwner, "contrib": store.RoleContributor}}}
	a := &StackAPI{Team: team, Store: st}
	r := chi.NewRouter()
	r.Get("/projects/{id}/stack-templates", a.List())
	r.Post("/projects/{id}/stack-templates", a.Create())
	r.Get("/projects/{id}/stack-templates/{tid}", a.Get())
	r.Delete("/projects/{id}/stack-templates/{tid}", a.Delete())
	r.Post("/projects/{id}/stack-templates/{tid}/apply", a.Apply())
	do := func(who, method, target, body string) (int, map[string]any) {
		req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{OID: who}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var m map[string]any
		json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}
	create := `{"name":"Private endpoint group","canvas":` + stackCanvas + `,"selected":["s","p"],"params":[{"name":"subnet_size","node":"sn","field":"size"}]}`

	if code, _ := do("contrib", "POST", "/projects/p/stack-templates", create); code != http.StatusForbidden {
		t.Fatalf("a contributor does not make templates: %d", code)
	}
	code, m := do("owner", "POST", "/projects/p/stack-templates", create)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, m)
	}
	id := m["id"].(string)
	if code, _ := do("owner", "POST", "/projects/p/stack-templates", create); code != http.StatusConflict {
		t.Fatalf("a duplicate name = %d", code)
	}
	if code, m := do("owner", "POST", "/projects/p/stack-templates", `{"name":"x","canvas":`+stackCanvas+`,"selected":["s"],"params":[{"name":"A b","node":"sn","field":"size"}]}`); code != http.StatusUnprocessableEntity || m["problems"] == nil {
		t.Fatalf("a bad parameter lists its problems: %d %v", code, m)
	}
	wl := httptest.NewRecorder()
	rq := httptest.NewRequest(http.MethodGet, "/projects/p/stack-templates", nil)
	r.ServeHTTP(wl, rq.WithContext(auth.WithUser(rq.Context(), &auth.User{OID: "contrib"})))
	var listed []map[string]any
	if json.Unmarshal(wl.Body.Bytes(), &listed); len(listed) != 1 || listed[0]["name"] != "Private endpoint group" {
		t.Fatalf("a member lists templates: %s", wl.Body)
	}

	apply := func(body string) (int, map[string]any) {
		return do("contrib", "POST", "/projects/p/stack-templates/"+id+"/apply", body)
	}
	// Slots must be filled: the parent of the subnet, and the key vault the endpoint pointed at.
	if code, m := apply(`{"env":"dev"}`); code != http.StatusUnprocessableEntity || len(m["problems"].([]any)) != 2 {
		t.Fatalf("empty slots: %d %v", code, m)
	}
	good := `{"env":"prod","prefix":"app-","slots":{"parent of sn":"vnet-existing","module kv":"kv_main"},"values":{"subnet_size":{"prod":"22"}},"offset_x":300}`
	code, m = apply(good)
	if code != 200 || len(m["nodes"].([]any)) != 2 {
		t.Fatalf("apply = %d %v", code, m)
	}
	vars := m["variables"].([]any)[0].(map[string]any)
	if vars["name"] != "subnet_size" || vars["values"].(map[string]any)["prod"] != "22" {
		t.Fatalf("the parameter comes back as a project variable for this environment: %v", vars)
	}
	if code, _ := apply(`{"env":"prod","prefix":"app-","slots":{"parent of sn":"v","module kv":"k"},"draft_labels":["vnet/app-sn"]}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("a name that clashes with the draft = %d", code)
	}
	if code, _ := do("contrib", "DELETE", "/projects/p/stack-templates/"+id, ""); code != http.StatusForbidden {
		t.Fatalf("a contributor does not delete: %d", code)
	}
	if code, _ := do("owner", "DELETE", "/projects/p/stack-templates/"+id, ""); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if code, _ := do("owner", "GET", "/projects/p/stack-templates/"+id, ""); code != http.StatusNotFound {
		t.Fatalf("gone = %d", code)
	}
}
