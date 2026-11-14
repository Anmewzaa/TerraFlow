package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/validate"
)

type fakeModules struct{}

func (fakeModules) VisibleModules(context.Context, string) ([]catalog.Module, map[string]bool, error) {
	schema := `{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"lock_level":{"type":"string","enum":["CanNotDelete","ReadOnly"]}}}`
	return []catalog.Module{{Name: "resource-group", Version: "v1", SchemaJSON: json.RawMessage(schema)}}, nil, nil
}

type validationEnv struct {
	h   http.Handler
	mem *MemChangesetStore
}

func newValidationEnv() *validationEnv {
	mem := NewMemChangesetStore()
	r := chi.NewRouter()
	r.Post("/projects/{id}/validate", ValidateCanvas(fakeModules{}))
	r.Route("/projects/{id}/changesets/{csId}", func(cs chi.Router) {
		cs.Put("/", MemChangesetSaveHandler(mem))
		cs.With(ValidChangeset(mem.GetChangeset, fakeModules{})).Post("/publish", MemChangesetPublishHandler(mem))
	})
	return &validationEnv{h: r, mem: mem}
}

func (e *validationEnv) do(method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return w
}

func findingsOf(t *testing.T, w *httptest.ResponseRecorder) []validate.Finding {
	t.Helper()
	var out findingsBody
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("not a findings body: %v: %s", err, w.Body)
	}
	return out.Findings
}

func oneGroupCanvas(name string) string {
	return `{"nodes":[{"id":"n1","module_name":"resource-group","module_version":"v1","instance_name":"rg","target_folder":"common","values":{"name":` + name + `}}],"edges":[]}`
}

func TestSaveRefusesUnsafeInputButKeepsUnfinishedWork(t *testing.T) {
	e := newValidationEnv()
	id := e.mem.Seed(store.Changeset{ProjectID: "p", Env: "dev", UserOID: "u", Status: store.ChangesetDraft, CanvasJSON: json.RawMessage(`{"nodes":[],"edges":[]}`)})

	// Unfinished work (a required argument empty) is saved: a draft may be incomplete.
	if w := e.do("PUT", "/projects/p/changesets/"+id+"/", oneGroupCanvas(`""`)); w.Code != http.StatusOK {
		t.Fatalf("an incomplete draft must be saved: %d %s", w.Code, w.Body)
	}
	// Unsafe input is refused and not stored.
	evil := `{"nodes":[{"id":"n1","instance_name":"rg","target_folder":"../etc"}],"edges":[]}`
	w := e.do("PUT", "/projects/p/changesets/"+id+"/", evil)
	if w.Code != http.StatusUnprocessableEntity || !validate.HasStructural(findingsOf(t, w)) {
		t.Fatalf("unsafe input must be refused: %d %s", w.Code, w.Body)
	}
	cs, _ := e.mem.GetChangeset(context.Background(), id)
	if strings.Contains(string(cs.CanvasJSON), "../etc") {
		t.Fatal("the refused canvas was stored")
	}
	// Not JSON, wrong shape, too big.
	if w := e.do("PUT", "/projects/p/changesets/"+id+"/", `not json`); w.Code != http.StatusBadRequest {
		t.Errorf("not json = %d", w.Code)
	}
	if w := e.do("PUT", "/projects/p/changesets/"+id+"/", `{"nodes":"no"}`); w.Code != http.StatusBadRequest {
		t.Errorf("wrong shape = %d", w.Code)
	}
	big := `{"nodes":[],"edges":[],"pad":"` + strings.Repeat("a", MaxCanvasBytes) + `"}`
	if w := e.do("PUT", "/projects/p/changesets/"+id+"/", big); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("too big = %d", w.Code)
	}
}

func TestPublishRunsEveryRuleOnTheServer(t *testing.T) {
	e := newValidationEnv()
	bad := e.mem.Seed(store.Changeset{ProjectID: "p", Env: "dev", UserOID: "u", Status: store.ChangesetSaved, CanvasJSON: json.RawMessage(oneGroupCanvas(`""`))})
	// A client that skips the screens and calls publish directly still meets the rules.
	w := e.do("POST", "/projects/p/changesets/"+bad+"/publish", `{"title":"x"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("publish of an incomplete canvas = %d %s", w.Code, w.Body)
	}
	fs := findingsOf(t, w)
	if len(fs) == 0 || fs[0].Rule != "required" || fs[0].Node != "n1" {
		t.Fatalf("findings: %+v", fs)
	}
	if cs, _ := e.mem.GetChangeset(context.Background(), bad); cs.Status != store.ChangesetSaved {
		t.Fatalf("a refused publish must not change the status: %s", cs.Status)
	}

	good := e.mem.Seed(store.Changeset{ProjectID: "p2", Env: "dev", UserOID: "u", Status: store.ChangesetSaved, CanvasJSON: json.RawMessage(oneGroupCanvas(`"rg-app"`))})
	if w := e.do("POST", "/projects/p2/changesets/"+good+"/publish", `{"title":"x"}`); w.Code != http.StatusOK {
		t.Fatalf("a valid canvas publishes: %d %s", w.Code, w.Body)
	}
}

func TestValidateEndpointAnswersWithoutChangingAnything(t *testing.T) {
	e := newValidationEnv()
	w := e.do("POST", "/projects/p/validate", oneGroupCanvas(`"rg"`)[:0]+`{"nodes":[{"id":"a","module_name":"resource-group","module_version":"v1","instance_name":"x","values":{"name":"n","lock_level":"Banana"}}],"edges":[]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("validate = %d %s", w.Code, w.Body)
	}
	fs := findingsOf(t, w)
	if len(fs) != 1 || fs[0].Rule != "schema" || fs[0].Field != "lock_level" {
		t.Fatalf("findings: %+v", fs)
	}
	if w := e.do("POST", "/projects/p/validate", `{"nodes":[{"id":"a","instance_name":"x","values":{"v":{"$expr":"a } b"}}}]}`); !validate.HasStructural(findingsOf(t, w)) {
		t.Fatal("an expression that would close the block must be reported")
	}
}

func TestExistingBlocksAreOnlyWarnedAboutTheSchema(t *testing.T) {
	e := newValidationEnv()
	// A block already in the repository may lack an argument the schema now requires.
	c := `{"nodes":[{"id":"a","module_name":"resource-group","module_version":"v1","instance_name":"x","values":{},"existing":{"folder":"common","filename":"rg.tf","label":"x"}}],"edges":[]}`
	id := e.mem.Seed(store.Changeset{ProjectID: "p", Env: "dev", UserOID: "u", Status: store.ChangesetSaved, CanvasJSON: json.RawMessage(c)})
	if w := e.do("POST", "/projects/p/changesets/"+id+"/publish", `{"title":"x"}`); w.Code != http.StatusOK {
		t.Fatalf("editing an existing block must not be blocked by the schema: %d %s", w.Code, w.Body)
	}
}

// Seed puts a changeset straight into the in-memory store and returns its id.
func (s *MemChangesetStore) Seed(cs store.Changeset) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs.ID = "cs-" + string(rune('a'+len(s.data)))
	s.data[cs.ID] = cs
	return cs.ID
}
