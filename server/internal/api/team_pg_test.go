package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// teamEnv makes a project with an owner (alice), a co-owner (carol), a contributor
// (bob) and an outsider (mallory), all of whom have signed in.
func teamEnv(t *testing.T) (*pgEnv, string) {
	t.Helper()
	e := newPGEnv(t)
	id := e.newProject()
	for _, u := range []string{"alice", "bob", "carol", "mallory", "dave"} {
		e.do(u, "user", "GET", "/api/projects", nil)
	}
	ctx := context.Background()
	if err := e.q.CreateOwner(ctx, id, "alice"); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.q.AddMemberRole(ctx, id, "carol", store.RoleCoOwner, "alice"))
	must(e.q.AddMemberRole(ctx, id, "bob", store.RoleContributor, "alice"))
	return e, id
}

func TestOnlyOwnersAndCoOwnersMayReview(t *testing.T) {
	e, id := teamEnv(t)
	code, cs, raw := e.do("bob", "user", "POST", "/api/projects/"+id+"/changesets/", map[string]string{"env": "dev"})
	if code != 201 && code != 200 {
		t.Fatalf("create: %d %s", code, raw)
	}
	base := "/api/projects/" + id + "/changesets/" + cs["id"].(string)
	if code, _, raw := e.do("bob", "user", "POST", base+"/publish", map[string]string{"title": "t"}); code != 200 {
		t.Fatalf("publish: %d %s", code, raw)
	}
	for _, who := range []string{"bob", "mallory"} {
		if code, _, _ := e.do(who, "user", "POST", base+"/reject", map[string]string{"reason": "x"}); code == http.StatusOK {
			t.Fatalf("%s must not review", who)
		}
	}
	if code, _, _ := e.do("bob", "user", "POST", base+"/reject", map[string]string{"reason": "x"}); code != http.StatusForbidden {
		t.Fatalf("a contributor reviewing = %d, want 403", code)
	}
	if code, _, raw := e.do("carol", "user", "POST", base+"/reject", map[string]string{"reason": "not now"}); code != http.StatusOK {
		t.Fatalf("a co-owner reviewing = %d %s, want 200", code, raw)
	}
}

func TestSecondReviewerSwitchBlocksSelfApproval(t *testing.T) {
	e, id := teamEnv(t)
	ctx := context.Background()
	conn, err := e.q.UpsertConnection(ctx, store.Connection{Provider: "github", ExternalID: "5", AccountLogin: "x", CreatedBy: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.q.SetProjectRepo(ctx, store.ProjectRepo{ProjectID: id, ConnectionID: conn.ID, RepoExternalID: "1", RepoFullName: "x/y", DefaultBranch: "main", ConnectedBy: "alice"}); err != nil {
		t.Fatal(err)
	}
	newPublished := func(user string) string {
		_, cs, _ := e.do(user, "user", "POST", "/api/projects/"+id+"/changesets/", map[string]string{"env": "dev"})
		base := "/api/projects/" + id + "/changesets/" + cs["id"].(string)
		e.do(user, "user", "POST", base+"/publish", map[string]string{"title": "t"})
		return base
	}

	// Off (the default): the owner may decide on their own change.
	if code, _, raw := e.do("alice", "user", "POST", newPublished("alice")+"/reject", map[string]string{"reason": "x"}); code != http.StatusOK {
		t.Fatalf("owner reviewing their own change with the switch off = %d %s", code, raw)
	}
	// On: the same owner is refused, another co-owner is not.
	if code, _, raw := e.do("alice", "user", "PUT", "/api/projects/"+id+"/settings", map[string]bool{"second_reviewer": true}); code != http.StatusOK {
		t.Fatalf("switch on = %d %s", code, raw)
	}
	base := newPublished("alice")
	if code, _, _ := e.do("alice", "user", "POST", base+"/reject", map[string]string{"reason": "x"}); code != http.StatusForbidden {
		t.Fatalf("owner reviewing their own change with the switch on = %d, want 403", code)
	}
	if code, _, raw := e.do("carol", "user", "POST", base+"/reject", map[string]string{"reason": "x"}); code != http.StatusOK {
		t.Fatalf("a second reviewer = %d %s, want 200", code, raw)
	}
	// A contributor cannot flip the switch.
	if code, _, _ := e.do("bob", "user", "PUT", "/api/projects/"+id+"/settings", map[string]bool{"second_reviewer": false}); code != http.StatusForbidden {
		t.Fatalf("contributor changing settings = %d, want 403", code)
	}
}

func TestInviteRolesAndRemoval(t *testing.T) {
	e, id := teamEnv(t)
	team := "/api/projects/" + id + "/team"

	// A contributor cannot invite; a stranger cannot even see the project.
	if code, _, _ := e.do("bob", "user", "POST", team, map[string]string{"user": "dave"}); code != http.StatusForbidden {
		t.Fatalf("contributor invite = %d, want 403", code)
	}
	if code, _, _ := e.do("mallory", "user", "POST", team, map[string]string{"user": "dave"}); code != http.StatusNotFound {
		t.Fatalf("stranger invite = %d, want 404", code)
	}
	// A co-owner invites by email; the default role is contributor.
	if code, _, raw := e.do("carol", "user", "POST", team, map[string]string{"user": "dave@x.test"}); code != http.StatusOK {
		t.Fatalf("co-owner invite = %d %s", code, raw)
	}
	if role, _ := e.q.MemberRole(context.Background(), id, "dave"); role != store.RoleContributor {
		t.Fatalf("dave's role = %q", role)
	}
	// Someone who never signed in cannot be invited, and the answer says why.
	if code, _, _ := e.do("alice", "user", "POST", team, map[string]string{"user": "ghost@x.test"}); code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown user = %d, want 422", code)
	}
	// Nobody can be made owner, and the owner cannot be changed or removed.
	if code, _, _ := e.do("alice", "user", "POST", team, map[string]string{"user": "bob", "role": "owner"}); code != http.StatusBadRequest {
		t.Fatalf("make owner = %d, want 400", code)
	}
	if code, _, _ := e.do("carol", "user", "PATCH", team+"/alice", map[string]string{"role": "contributor"}); code != http.StatusConflict {
		t.Fatalf("demote owner = %d, want 409", code)
	}
	if code, _, _ := e.do("carol", "user", "DELETE", team+"/alice", nil); code != http.StatusConflict {
		t.Fatalf("remove owner = %d, want 409", code)
	}
	// A co-owner promotes and removes.
	if code, _, _ := e.do("carol", "user", "PATCH", team+"/bob", map[string]string{"role": "co_owner"}); code != http.StatusOK {
		t.Fatalf("promote = %d", code)
	}
	if code, _, _ := e.do("carol", "user", "DELETE", team+"/dave", nil); code != http.StatusNoContent {
		t.Fatalf("remove = %d", code)
	}
	if code, _, _ := e.do("dave", "user", "GET", "/api/projects/"+id+"/graph", nil); code != http.StatusNotFound {
		t.Fatalf("a removed member still sees the project (%d)", code)
	}
}

func TestOnlyTheOwnerTransfersAndDeletes(t *testing.T) {
	e, id := teamEnv(t)
	if code, _, _ := e.do("carol", "user", "POST", "/api/projects/"+id+"/transfer", map[string]string{"to": "bob"}); code != http.StatusForbidden {
		t.Fatalf("co-owner transfer = %d, want 403", code)
	}
	if code, _, _ := e.do("alice", "user", "POST", "/api/projects/"+id+"/transfer", map[string]string{"to": "mallory"}); code != http.StatusUnprocessableEntity {
		t.Fatalf("transfer to a non-member = %d, want 422", code)
	}
	if code, _, raw := e.do("alice", "user", "POST", "/api/projects/"+id+"/transfer", map[string]string{"to": "bob"}); code != http.StatusOK {
		t.Fatalf("transfer = %d %s", code, raw)
	}
	ctx := context.Background()
	if r, _ := e.q.MemberRole(ctx, id, "bob"); r != store.RoleOwner {
		t.Fatalf("bob = %q, want owner", r)
	}
	if r, _ := e.q.MemberRole(ctx, id, "alice"); r != store.RoleCoOwner {
		t.Fatalf("alice = %q, want co_owner after stepping down", r)
	}
	// The project is a local one, so even its owner cannot delete it (an admin does).
	if code, _, _ := e.do("bob", "user", "DELETE", "/api/projects/"+id, nil); code != http.StatusForbidden {
		t.Fatalf("owner deleting a local project = %d, want 403", code)
	}
}

func TestAdminSetMembersNeverRemovesTheOwner(t *testing.T) {
	e, id := teamEnv(t)
	if code, _, raw := e.do("root", "admin", "PUT", "/api/projects/"+id+"/members", map[string]any{"user_oids": []string{"bob"}}); code != http.StatusOK {
		t.Fatalf("set members = %d %s", code, raw)
	}
	if r, _ := e.q.MemberRole(context.Background(), id, "alice"); r != store.RoleOwner {
		t.Fatalf("the owner lost the project: role %q", r)
	}
}

func TestSupportAccessByAnAdminIsAudited(t *testing.T) {
	e, id := teamEnv(t)
	e.do("root", "admin", "GET", "/api/projects", nil)
	before := auditCount(t, e, id, "support.access")
	if code, _, _ := e.do("root", "admin", "GET", "/api/projects/"+id+"/files", nil); code == http.StatusNotFound || code == http.StatusForbidden {
		t.Fatalf("admin support access = %d", code)
	}
	if after := auditCount(t, e, id, "support.access"); after != before+1 {
		t.Fatalf("support.access entries %d -> %d, want one more", before, after)
	}
	// A member who is also an admin is not "support": it is their own project.
	e.q.AddMemberRole(context.Background(), id, "root", store.RoleContributor, "alice")
	e.do("root", "admin", "GET", "/api/projects/"+id+"/files", nil)
	if again := auditCount(t, e, id, "support.access"); again != before+1 {
		t.Fatalf("a member admin was audited as support access (%d)", again)
	}
}

func auditCount(t *testing.T, e *pgEnv, projectID, action string) int {
	t.Helper()
	entries, err := e.q.ListAudit(context.Background(), projectID, 200)
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
