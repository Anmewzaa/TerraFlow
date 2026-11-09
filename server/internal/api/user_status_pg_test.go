package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// The admin's approval screen: approve, reject, suspend and restore, with the audit
// trail and the rules about who may do what.

func pendingUser(t *testing.T, e *pgEnv, oid string) {
	t.Helper()
	if _, err := e.q.EnsureUser(context.Background(), oid, oid+"@x.test", oid, store.StatusPending, 0); err != nil {
		t.Fatal(err)
	}
}

func statusOf(t *testing.T, e *pgEnv, oid string) string {
	t.Helper()
	st, err := e.q.GetUserStatus(context.Background(), oid)
	if err != nil {
		t.Fatal(err)
	}
	return st.Status
}

func TestAdminApprovesRejectsSuspendsAndRestores(t *testing.T) {
	e := newPGEnv(t)
	pendingUser(t, e, "github:1")
	pendingUser(t, e, "github:2")
	post := func(oid string, body map[string]string) int {
		code, _, _ := e.do("admin", "admin", "POST", "/api/admin/users/"+oid+"/status", body)
		return code
	}
	if code := post("github:1", map[string]string{"status": "active"}); code != http.StatusOK || statusOf(t, e, "github:1") != "active" {
		t.Fatalf("approve = %d, status %s", code, statusOf(t, e, "github:1"))
	}
	if code := post("github:2", map[string]string{"status": "rejected"}); code != http.StatusUnprocessableEntity {
		t.Errorf("reject without a reason = %d", code)
	}
	if code := post("github:2", map[string]string{"status": "rejected", "reason": "unknown person"}); code != http.StatusOK || statusOf(t, e, "github:2") != "rejected" {
		t.Errorf("reject = %d", code)
	}
	if code := post("github:1", map[string]string{"status": "rejected", "reason": "x"}); code != http.StatusUnprocessableEntity {
		t.Errorf("rejecting an active user must say to suspend = %d", code)
	}
	if code := post("github:2", map[string]string{"status": "suspended", "reason": "x"}); code != http.StatusUnprocessableEntity {
		t.Errorf("suspending a rejected user = %d", code)
	}
	if code := post("github:1", map[string]string{"status": "suspended", "reason": "left"}); code != http.StatusOK || statusOf(t, e, "github:1") != "suspended" {
		t.Errorf("suspend = %d", code)
	}
	if code := post("github:1", map[string]string{"status": "active"}); code != http.StatusOK || statusOf(t, e, "github:1") != "active" {
		t.Errorf("restore = %d", code)
	}
	for _, bad := range []map[string]string{{"status": "pending"}, {"status": "nonsense"}, {}} {
		if code := post("github:1", bad); code != http.StatusBadRequest {
			t.Errorf("%v = %d, want 400", bad, code)
		}
	}
	if code := post("github:404", map[string]string{"status": "active"}); code != http.StatusNotFound {
		t.Errorf("unknown user = %d", code)
	}
	if code := post("admin", map[string]string{"status": "suspended", "reason": "x"}); code != http.StatusUnprocessableEntity {
		t.Errorf("an admin changing their own account = %d", code)
	}
	if n := auditAll(t, e, "user.approve"); n != 1 {
		t.Errorf("user.approve entries = %d, want 1", n)
	}
	if auditAll(t, e, "user.reject") != 1 || auditAll(t, e, "user.suspend") != 1 || auditAll(t, e, "user.restore") != 1 {
		t.Error("every change must be audited")
	}
}

func TestOnlyAdminsChangeUserStatusAndTheListShowsIt(t *testing.T) {
	e := newPGEnv(t)
	pendingUser(t, e, "github:5")
	if code, _, _ := e.do("bob", "user", "POST", "/api/admin/users/github:5/status", map[string]string{"status": "active"}); code != http.StatusForbidden {
		t.Fatalf("a non-admin approving = %d", code)
	}
	if statusOf(t, e, "github:5") != "pending" {
		t.Fatal("the status must not change")
	}
	code, _, raw := e.do("admin", "admin", "GET", "/api/admin/users", nil)
	if code != 200 || !contains(string(raw), `"status":"pending"`) || !contains(string(raw), `"provider":"github"`) {
		t.Fatalf("list = %d %s", code, raw)
	}
}

func TestPendingUsersCannotBeInvitedOrAssigned(t *testing.T) {
	e, id := teamEnv(t)
	pendingUser(t, e, "github:77")
	if code, _, _ := e.do("alice", "user", "POST", "/api/projects/"+id+"/team", map[string]string{"user": "github:77"}); code != http.StatusUnprocessableEntity {
		t.Errorf("inviting a pending user = %d", code)
	}
	if code, _, _ := e.do("admin", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"github:77"}}); code != http.StatusUnprocessableEntity {
		t.Errorf("the admin assigning a pending user = %d", code)
	}
	if err := e.q.SetUserStatus(context.Background(), "github:77", store.StatusActive, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := e.do("alice", "user", "POST", "/api/projects/"+id+"/team", map[string]string{"user": "github:77"}); code != http.StatusOK && code != http.StatusCreated {
		t.Errorf("inviting an approved user = %d %s", code, raw)
	}
}

func contains(s, sub string) bool { return len(sub) > 0 && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func auditAll(t *testing.T, e *pgEnv, action string) int {
	t.Helper()
	entries, err := e.q.ListAuditAll(context.Background(), 500)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range entries {
		if a.Action == action {
			n++
		}
	}
	return n
}
