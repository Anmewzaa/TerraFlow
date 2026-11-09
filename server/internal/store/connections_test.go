package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestConnectionsUpsertIsIdempotentAndKeepsTheCreator(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	first, err := q.UpsertConnection(ctx, store.Connection{Provider: "github", ExternalID: "777", AccountLogin: "alice", AccountType: "User", CreatedBy: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != store.ConnActive || first.ID == "" {
		t.Fatalf("first: %+v", first)
	}
	if err := q.SetConnectionStatus(ctx, first.ID, store.ConnNeedsAttention); err != nil {
		t.Fatal(err)
	}
	// Reconnecting (another person finishes the installation flow) reactivates the
	// connection and renames the account, but the creator does not change.
	again, err := q.UpsertConnection(ctx, store.Connection{Provider: "github", ExternalID: "777", AccountLogin: "alice-renamed", AccountType: "Organization", CreatedBy: "u2"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || again.CreatedBy != "u1" || again.AccountLogin != "alice-renamed" || again.Status != store.ConnActive {
		t.Fatalf("again: %+v", again)
	}
	list, _ := q.ListConnections(ctx)
	if len(list) != 1 {
		t.Fatalf("one installation is one connection, got %d", len(list))
	}
	// The same external id under another provider is a different connection.
	other, err := q.UpsertConnection(ctx, store.Connection{Provider: "azurerepos", ExternalID: "777", AccountLogin: "org", CreatedBy: "u1"})
	if err != nil || other.ID == first.ID {
		t.Fatalf("other provider: %+v %v", other, err)
	}
}

func TestConnectionValidation(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	for name, c := range map[string]store.Connection{
		"no provider": {ExternalID: "1", AccountLogin: "a", CreatedBy: "u"},
		"no id":       {Provider: "github", AccountLogin: "a", CreatedBy: "u"},
		"no account":  {Provider: "github", ExternalID: "1", CreatedBy: "u"},
		"no creator":  {Provider: "github", ExternalID: "1", AccountLogin: "a"},
	} {
		if _, err := q.UpsertConnection(ctx, c); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	c, _ := q.UpsertConnection(ctx, store.Connection{Provider: "github", ExternalID: "1", AccountLogin: "a", CreatedBy: "u"})
	if err := q.SetConnectionStatus(ctx, c.ID, "bogus"); err == nil {
		t.Error("an unknown status must be refused")
	}
	if err := q.SetConnectionStatus(ctx, "00000000-0000-0000-0000-000000000000", store.ConnActive); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown connection: %v", err)
	}
	if _, err := q.GetConnection(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("get unknown: %v", err)
	}
}

func TestProjectRepoLinkLifecycle(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	c, _ := q.UpsertConnection(ctx, store.Connection{Provider: "github", ExternalID: "1", AccountLogin: "alice", CreatedBy: "u1"})

	if _, err := q.GetProjectRepo(ctx, p.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("a local project has no repository link: %v", err)
	}
	link := store.ProjectRepo{ProjectID: p.ID, ConnectionID: c.ID, RepoExternalID: "42", RepoFullName: "alice/infra", DefaultBranch: "main", RepoPath: "iac", ConnectedBy: "u1"}
	if err := q.SetProjectRepo(ctx, link); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetProjectRepo(ctx, p.ID)
	if err != nil || got.RepoFullName != "alice/infra" || got.RepoPath != "iac" || got.SecondReviewer {
		t.Fatalf("link: %+v %v", got, err)
	}
	if n, _ := q.ProjectsOfConnection(ctx, c.ID); n != 1 {
		t.Errorf("projects of the connection: %d", n)
	}

	if err := q.SetSecondReviewer(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	// Reconnecting to another repository replaces the link and keeps the switch.
	link.RepoFullName, link.RepoExternalID = "alice/other", "43"
	if err := q.SetProjectRepo(ctx, link); err != nil {
		t.Fatal(err)
	}
	got, _ = q.GetProjectRepo(ctx, p.ID)
	if got.RepoFullName != "alice/other" || !got.SecondReviewer {
		t.Errorf("replaced link: %+v", got)
	}

	for name, bad := range map[string]store.ProjectRepo{
		"no connection": {ProjectID: p.ID, RepoExternalID: "1", RepoFullName: "a/b", DefaultBranch: "main", ConnectedBy: "u"},
		"no branch":     {ProjectID: p.ID, ConnectionID: c.ID, RepoExternalID: "1", RepoFullName: "a/b", ConnectedBy: "u"},
		"no one":        {ProjectID: p.ID, ConnectionID: c.ID, RepoExternalID: "1", RepoFullName: "a/b", DefaultBranch: "main"},
	} {
		if err := q.SetProjectRepo(ctx, bad); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if err := q.SetSecondReviewer(ctx, "00000000-0000-0000-0000-000000000000", true); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown project: %v", err)
	}

	if err := q.DeleteProjectRepo(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetProjectRepo(ctx, p.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("after delete: %v", err)
	}
	// Deleting the project removes its link too.
	if err := q.SetProjectRepo(ctx, link); err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := q.ProjectsOfConnection(ctx, c.ID); n != 0 {
		t.Errorf("the link must go with the project: %d", n)
	}
}

func TestExistingMembersAreContributors(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	newUser(t, q, "alice")
	// The old API adds members without a role.
	if err := q.AddMember(ctx, p.ID, "alice", "admin"); err != nil {
		t.Fatal(err)
	}
	if role, _ := q.MemberRole(ctx, p.ID, "alice"); role != store.RoleContributor {
		t.Errorf("role %q", role)
	}
	if role, _ := q.MemberRole(ctx, p.ID, "nobody"); role != "" {
		t.Errorf("a non-member has no role, got %q", role)
	}
}

func TestOwnershipRules(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	for _, u := range []string{"owner", "carol", "bob", "dave"} {
		newUser(t, q, u)
	}
	if err := q.CreateOwner(ctx, p.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	if err := q.AddMemberRole(ctx, p.ID, "carol", store.RoleCoOwner, "owner"); err != nil {
		t.Fatal(err)
	}
	if err := q.AddMemberRole(ctx, p.ID, "bob", store.RoleContributor, "owner"); err != nil {
		t.Fatal(err)
	}

	// Nothing can create a second owner or change the owner's role.
	if err := q.AddMemberRole(ctx, p.ID, "dave", store.RoleOwner, "owner"); err == nil {
		t.Error("AddMemberRole must not make an owner")
	}
	if err := q.AddMemberRole(ctx, p.ID, "owner", store.RoleContributor, "owner"); !errors.Is(err, store.ErrOwnerChange) {
		t.Errorf("demoting the owner: %v", err)
	}
	if err := q.RemoveMember(ctx, p.ID, "owner"); !errors.Is(err, store.ErrOwnerChange) {
		t.Errorf("removing the owner: %v", err)
	}
	if err := q.AddMemberRole(ctx, p.ID, "dave", "admin", "owner"); err == nil {
		t.Error("an unknown role must be refused")
	}
	// A role can change and a member can be removed.
	if err := q.AddMemberRole(ctx, p.ID, "bob", store.RoleCoOwner, "owner"); err != nil {
		t.Fatal(err)
	}
	if role, _ := q.MemberRole(ctx, p.ID, "bob"); role != store.RoleCoOwner {
		t.Errorf("bob %q", role)
	}
	if err := q.RemoveMember(ctx, p.ID, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := q.RemoveMember(ctx, p.ID, "bob"); !errors.Is(err, store.ErrNotMember) {
		t.Errorf("removing twice: %v", err)
	}

	// Transfer: the owner steps down to co-owner; only a member can receive it.
	if err := q.TransferOwnership(ctx, p.ID, "owner", "dave"); !errors.Is(err, store.ErrNotMember) {
		t.Errorf("transfer to a non-member: %v", err)
	}
	if err := q.TransferOwnership(ctx, p.ID, "carol", "owner"); err == nil {
		t.Error("only the owner can transfer")
	}
	if err := q.TransferOwnership(ctx, p.ID, "owner", "carol"); err != nil {
		t.Fatal(err)
	}
	if r, _ := q.MemberRole(ctx, p.ID, "carol"); r != store.RoleOwner {
		t.Errorf("carol %q", r)
	}
	if r, _ := q.MemberRole(ctx, p.ID, "owner"); r != store.RoleCoOwner {
		t.Errorf("the old owner %q", r)
	}

	list, err := q.ListMembersWithRoles(ctx, p.ID)
	if err != nil || len(list) != 2 || list[0].Role != store.RoleOwner || list[1].Role != store.RoleCoOwner {
		t.Fatalf("members, owner first: %+v %v", list, err)
	}
}

func TestOnlyOneOwnerPerProjectInTheDatabase(t *testing.T) {
	q, dsn := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	newUser(t, q, "a")
	newUser(t, q, "b")
	if err := q.CreateOwner(ctx, p.ID, "a"); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO project_members (project_id, user_oid, role) VALUES ($1, 'b', 'owner')`, p.ID)
	if err == nil {
		t.Fatal("the unique index must refuse a second owner")
	}
	if _, err := conn.Exec(ctx, `INSERT INTO project_members (project_id, user_oid, role) VALUES ($1, 'b', 'admin')`, p.ID); err == nil {
		t.Fatal("the check constraint must refuse an unknown role")
	}
}

func TestRoleHelpers(t *testing.T) {
	if !store.CanApprove(store.RoleOwner) || !store.CanApprove(store.RoleCoOwner) || store.CanApprove(store.RoleContributor) || store.CanApprove("") {
		t.Error("only owner and co-owner approve")
	}
	if !(store.RoleRank(store.RoleOwner) > store.RoleRank(store.RoleCoOwner) && store.RoleRank(store.RoleCoOwner) > store.RoleRank(store.RoleContributor) && store.RoleRank(store.RoleContributor) > store.RoleRank("")) {
		t.Error("rank order")
	}
	for _, r := range []string{"owner", "co_owner", "contributor"} {
		if !store.ValidRole(r) {
			t.Errorf("%s", r)
		}
	}
	for _, r := range []string{"", "admin", "Owner"} {
		if store.ValidRole(r) {
			t.Errorf("%q must be invalid", r)
		}
	}
}
