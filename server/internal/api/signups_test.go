package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
)

type memSignups struct {
	status map[string]string
	audit  []string
	ended  []string
}

func (m *memSignups) UserStatus(_ context.Context, oid string) (string, error) {
	return m.status[oid], nil
}
func (m *memSignups) SetUserStatus(_ context.Context, oid, status, _, _ string) error {
	m.status[oid] = status
	if status == "rejected" || status == "suspended" {
		m.ended = append(m.ended, oid)
	}
	return nil
}
func (m *memSignups) LogAudit(_ context.Context, _, action, _ string, _ any) error {
	m.audit = append(m.audit, action)
	return nil
}

func TestApprovalChangesFollowTheAllowedTransitionsAndAreAudited(t *testing.T) {
	m := &memSignups{status: map[string]string{"github:1": "pending", "github:2": "active", "github:3": "rejected", "admin": "active"}}
	r := chi.NewRouter()
	r.Post("/api/admin/users/{oid}/status", (&SignupAPI{Store: m}).SetStatus())
	do := func(oid, status, reason string) int {
		b, _ := json.Marshal(map[string]string{"status": status, "reason": reason})
		req := httptest.NewRequest("POST", "/api/admin/users/"+oid+"/status", bytes.NewReader(b))
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{OID: "admin", Roles: []string{"devops"}}))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, c := range []struct {
		name, oid, to string
		want          int
	}{
		{"approve a pending person", "github:1", "active", 200},
		{"suspend an active person", "github:2", "suspended", 200},
		{"lift a rejection", "github:3", "active", 200},
		{"cannot skip pending to suspended", "github:9", "suspended", 404},
		{"cannot approve an active person again", "github:3", "active", 409},
		{"cannot change your own status", "admin", "suspended", 409},
		{"unknown status", "github:1", "banned", 409},
	} {
		if got := do(c.oid, c.to, ""); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	if got := do("github:1", "suspended", string(bytes.Repeat([]byte("x"), 201))); got != http.StatusBadRequest {
		t.Errorf("a long reason: %d, want 400", got)
	}
	want := []string{"user.approve", "user.suspend", "user.reactivate"}
	if len(m.audit) != len(want) {
		t.Fatalf("audit = %v, want %v", m.audit, want)
	}
	for i := range want {
		if m.audit[i] != want[i] {
			t.Fatalf("audit = %v, want %v", m.audit, want)
		}
	}
	if len(m.ended) != 1 || m.ended[0] != "github:2" {
		t.Fatalf("sessions ended for %v, want only the suspended person", m.ended)
	}
}
