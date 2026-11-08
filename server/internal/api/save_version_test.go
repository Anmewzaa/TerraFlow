package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func TestMemSaveVersion_SnapshotsTheDraftAndPublishesFromTheSavedCopy(t *testing.T) {
	s := NewMemChangesetStore()
	r := chi.NewRouter()
	r.Post("/projects/{id}/changesets", MemChangesetCreateHandler(s))
	r.Put("/projects/{id}/changesets/{csId}", MemChangesetSaveHandler(s))
	r.Post("/projects/{id}/changesets/{csId}/versions", MemChangesetSaveVersionHandler(s))
	r.Post("/projects/{id}/changesets/{csId}/publish", MemChangesetPublishHandler(s))

	do := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}
	var created struct{ ID string }
	_ = json.Unmarshal(do("POST", "/projects/p1/changesets", `{"env":"dev"}`).Body.Bytes(), &created)
	canvas := `{"nodes":[{"id":"n1"}],"edges":[]}`
	do("PUT", "/projects/p1/changesets/"+created.ID, canvas)

	var v1, v2 struct{ ID, Title string }
	rec := do("POST", "/projects/p1/changesets/"+created.ID+"/versions", `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("save version: %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &v1)
	_ = json.Unmarshal(do("POST", "/projects/p1/changesets/"+created.ID+"/versions", `{}`).Body.Bytes(), &v2)
	if v1.Title != "Version 1" || v2.Title != "Version 2" || v1.ID == v2.ID || v1.ID == created.ID {
		t.Fatalf("each save is a new numbered copy: %+v %+v", v1, v2)
	}

	if d := s.data[created.ID]; d.Status != store.ChangesetDraft {
		t.Fatalf("the working draft must stay a draft, got %s", d.Status)
	}
	if c := s.data[v1.ID]; c.Status != store.ChangesetSaved || string(c.CanvasJSON) != canvas {
		t.Fatalf("snapshot must be saved with the draft's canvas: %s %s", c.Status, c.CanvasJSON)
	}
	// A saved version cannot itself be snapshotted: only the draft can.
	if rec := do("POST", "/projects/p1/changesets/"+v1.ID+"/versions", `{}`); rec.Code != http.StatusNotFound {
		t.Fatalf("snapshotting a saved version must be 404, got %d", rec.Code)
	}
	if rec := do("POST", "/projects/p1/changesets/"+v1.ID+"/publish", `{"title":"go"}`); rec.Code != http.StatusOK {
		t.Fatalf("publish saved version: %d %s", rec.Code, rec.Body)
	}
	if s.data[v1.ID].Status != store.ChangesetPublished {
		t.Fatal("published version must be awaiting review")
	}
}
