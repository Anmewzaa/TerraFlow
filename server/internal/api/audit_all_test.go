package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type fakeAuditAll struct{ gotLimit int }

func (f *fakeAuditAll) ListAuditAll(_ context.Context, limit int) ([]store.AuditEntry, error) {
	f.gotLimit = limit
	return nil, nil
}

func TestAuditAll_EmptyIsAListAndLimitIsClamped(t *testing.T) {
	f := &fakeAuditAll{}
	h := AuditAllHandler(f)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/audit", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" || f.gotLimit != 200 {
		t.Fatalf("%d %q limit=%d", rec.Code, rec.Body, f.gotLimit)
	}
	for q, want := range map[string]int{"?limit=50": 50, "?limit=9999": 200, "?limit=-3": 200, "?limit=abc": 200} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/admin/audit"+q, nil))
		if f.gotLimit != want {
			t.Errorf("%s: limit %d, want %d", q, f.gotLimit, want)
		}
	}
}
