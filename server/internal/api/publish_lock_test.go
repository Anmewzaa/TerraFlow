package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMemPublish_SecondIsBlockedUntilTheFirstIsDecided(t *testing.T) {
	s := NewMemChangesetStore()
	r := chi.NewRouter()
	r.Post("/projects/{id}/changesets", MemChangesetCreateHandler(s))
	r.Post("/projects/{id}/changesets/{csId}/publish", MemChangesetPublishHandler(s))

	create := func() string {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("POST", "/projects/p1/changesets", strings.NewReader(`{"env":"dev"}`)))
		var out struct{ ID string }
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
			t.Fatalf("create: %s", rec.Body)
		}
		return out.ID
	}
	publish := func(id, title string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("POST", "/projects/p1/changesets/"+id+"/publish", strings.NewReader(`{"title":"`+title+`"}`)))
		return rec
	}

	a, b := create(), create()
	if rec := publish(a, "first"); rec.Code != http.StatusOK {
		t.Fatalf("first publish: %d %s", rec.Code, rec.Body)
	}
	rec := publish(b, "second")
	if rec.Code != http.StatusConflict {
		t.Fatalf("second publish must be 409, got %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Error   string
		Pending struct{ ID, Title string }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Pending.ID != a || body.Pending.Title != "first" || !strings.Contains(body.Error, "first") {
		t.Fatalf("the 409 must name the changeset in the way: %+v", body)
	}
}
