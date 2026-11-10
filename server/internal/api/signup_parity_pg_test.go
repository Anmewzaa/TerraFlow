package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// Behaviour the approval screen must keep: who may change a status, which changes are
// allowed, that each is audited, and that a person who is not active cannot be invited
// or assigned to a project.

func waitingPerson(t *testing.T, e *pgEnv, oid string) {
	t.Helper()
	_, _, err := e.q.RecordLogin(context.Background(), store.LoginUser{OID: oid, Provider: "github", Name: oid, Email: oid + "@x.test"}, store.UserPending, 0, false)
	if err != nil {
		t.Fatal(err)
	}
}

func statusNow(t *testing.T, e *pgEnv, oid string) string {
	t.Helper()
	s, err := e.q.UserStatus(context.Background(), oid)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func auditAllCount(t *testing.T, e *pgEnv, action string) int {
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

func TestAdminApprovesRejectsSuspendsAndReactivates(t *testing.T) {
	e := newPGEnv(t)
	waitingPerson(t, e, "github:1")
	waitingPerson(t, e, "github:2")
	post := func(oid string, body map[string]string) int {
		code, _, _ := e.do("admin", "admin", "POST", "/api/admin/users/"+oid+"/status", body)
		return code
	}
	if code := post("github:1", map[string]string{"status": "active"}); code != http.StatusOK || statusNow(t, e, "github:1") != "active" {
		t.Fatalf("approve = %d", code)
	}
	if code := post("github:2", map[string]string{"status": "rejected", "reason": "unknown person"}); code != http.StatusOK || statusNow(t, e, "github:2") != "rejected" {
		t.Errorf("reject = %d", code)
	}
	if code := post("github:1", map[string]string{"status": "rejected"}); code == http.StatusOK {
		t.Error("an active person must be suspended, not rejected")
	}
	if code := post("github:2", map[string]string{"status": "suspended"}); code == http.StatusOK {
		t.Error("a rejected person cannot be suspended")
	}
	if code := post("github:1", map[string]string{"status": "suspended", "reason": "left"}); code != http.StatusOK || statusNow(t, e, "github:1") != "suspended" {
		t.Errorf("suspend = %d", code)
	}
	if code := post("github:1", map[string]string{"status": "active"}); code != http.StatusOK || statusNow(t, e, "github:1") != "active" {
		t.Errorf("reactivate = %d", code)
	}
	for _, bad := range []map[string]string{{"status": "pending"}, {"status": "nonsense"}, {}} {
		if code := post("github:1", bad); code < 400 || code >= 500 {
			t.Errorf("%v = %d, want a 4xx", bad, code)
		}
	}
	if code := post("github:404", map[string]string{"status": "active"}); code != http.StatusNotFound {
		t.Errorf("unknown user = %d", code)
	}
	if code := post("admin", map[string]string{"status": "suspended", "reason": "x"}); code < 400 {
		t.Errorf("an admin changing their own account = %d", code)
	}
	for _, a := range []string{"user.approve", "user.reject", "user.suspend", "user.reactivate"} {
		if auditAllCount(t, e, a) < 1 {
			t.Errorf("%s was not audited", a)
		}
	}
}

func TestOnlyAdminsChangeUserStatusAndTheListShowsIt(t *testing.T) {
	e := newPGEnv(t)
	waitingPerson(t, e, "github:5")
	if code, _, _ := e.do("bob", "user", "POST", "/api/admin/users/github:5/status", map[string]string{"status": "active"}); code != http.StatusForbidden {
		t.Fatalf("a non-admin approving = %d", code)
	}
	if statusNow(t, e, "github:5") != "pending" {
		t.Fatal("the status must not change")
	}
	code, _, raw := e.do("admin", "admin", "GET", "/api/admin/users", nil)
	if code != 200 || !strings.Contains(string(raw), `"status":"pending"`) || !strings.Contains(string(raw), `"provider":"github"`) {
		t.Fatalf("list = %d %s", code, raw)
	}
}

func TestPendingPeopleCannotBeInvitedOrAssigned(t *testing.T) {
	e, id := teamEnv(t)
	waitingPerson(t, e, "github:77")
	if code, _, _ := e.do("alice", "user", "POST", "/api/projects/"+id+"/team", map[string]string{"user": "github:77"}); code != http.StatusUnprocessableEntity {
		t.Errorf("inviting a pending person = %d", code)
	}
	if code, _, _ := e.do("admin", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"github:77"}}); code != http.StatusUnprocessableEntity {
		t.Errorf("the admin assigning a pending person = %d", code)
	}
	if err := e.q.SetUserStatus(context.Background(), "github:77", store.UserActive, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := e.do("alice", "user", "POST", "/api/projects/"+id+"/team", map[string]string{"user": "github:77"}); code != http.StatusOK && code != http.StatusCreated {
		t.Errorf("inviting an approved person = %d %s", code, raw)
	}
}
