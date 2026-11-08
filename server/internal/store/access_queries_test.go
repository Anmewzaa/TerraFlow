package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestMigrationsGoDownAndUpAgain(t *testing.T) {
	_, dsn := storetest.New(t)
	if err := store.MigrateDownAll(dsn); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := store.Migrate(dsn); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func newProject(t *testing.T, q *store.Queries, name string) store.Project {
	t.Helper()
	p, err := q.CreateProject(context.Background(), store.Project{Name: name, Slug: name, IACPath: "/ws/" + name, SourceType: "local"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newUser(t *testing.T, q *store.Queries, oid string) {
	t.Helper()
	if err := q.UpsertUser(context.Background(), store.User{OID: oid, Email: oid + "@x.test", Name: "User " + oid}); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertUserKeepsCreatedAtAndRefreshesProfile(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	newUser(t, q, "alice")
	first, _ := q.ListUsers(ctx)
	if err := q.UpsertUser(ctx, store.User{OID: "alice", Email: "new@x.test", Name: "Alice B"}); err != nil {
		t.Fatal(err)
	}
	second, _ := q.ListUsers(ctx)
	if len(second) != 1 || second[0].Email != "new@x.test" || second[0].Name != "Alice B" {
		t.Fatalf("profile not refreshed: %+v", second)
	}
	if !second[0].CreatedAt.Equal(first[0].CreatedAt) || second[0].LastLoginAt.Before(first[0].LastLoginAt) {
		t.Fatal("created_at must stay, last_login_at must move forward")
	}
}

func TestMembersLifecycle(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	a, b := newProject(t, q, "alpha"), newProject(t, q, "beta")
	for _, u := range []string{"alice", "bob", "carol"} {
		newUser(t, q, u)
	}

	if err := q.SetMembers(ctx, a.ID, []string{"alice", "bob"}, "root"); err != nil {
		t.Fatal(err)
	}
	if err := q.SetMembers(ctx, b.ID, []string{"alice"}, "root"); err != nil {
		t.Fatal(err)
	}
	ok, _ := q.IsMember(ctx, a.ID, "bob")
	no, _ := q.IsMember(ctx, b.ID, "bob")
	if !ok || no {
		t.Fatalf("IsMember: %v %v", ok, no)
	}
	mine, _ := q.ListProjectsForUser(ctx, "alice")
	if len(mine) != 2 || mine[0].Name != "alpha" {
		t.Fatalf("alice's projects: %+v", mine)
	}
	if none, err := q.ListProjectsForUser(ctx, "carol"); err != nil || len(none) != 0 {
		t.Fatalf("carol has no projects: %v %v", none, err)
	}
	members, _ := q.ListMembers(ctx, a.ID)
	if len(members) != 2 || members[0].AddedBy != "root" || members[0].Email == "" {
		t.Fatalf("members: %+v", members)
	}
	users, _ := q.ListUsers(ctx)
	counts := map[string]int{}
	for _, u := range users {
		counts[u.OID] = u.ProjectCount
	}
	if counts["alice"] != 2 || counts["bob"] != 1 || counts["carol"] != 0 {
		t.Fatalf("project counts: %v", counts)
	}

	// Replace: bob out, carol in, alice stays (and keeps her original added_by/created_at row).
	if err := q.SetMembers(ctx, a.ID, []string{"alice", "carol"}, "other"); err != nil {
		t.Fatal(err)
	}
	members, _ = q.ListMembers(ctx, a.ID)
	by := map[string]string{}
	for _, m := range members {
		by[m.UserOID] = m.AddedBy
	}
	if len(by) != 2 || by["alice"] != "root" || by["carol"] != "other" {
		t.Fatalf("replace semantics: %v", by)
	}
	if err := q.SetMembers(ctx, a.ID, []string{}, "root"); err != nil {
		t.Fatal(err)
	}
	if members, _ = q.ListMembers(ctx, a.ID); len(members) != 0 {
		t.Fatalf("empty list removes everyone: %+v", members)
	}
}

func TestSetMembersIsAllOrNothing(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	newUser(t, q, "alice")
	newUser(t, q, "bob")
	if err := q.SetMembers(ctx, p.ID, []string{"alice"}, "root"); err != nil {
		t.Fatal(err)
	}

	err := q.SetMembers(ctx, p.ID, []string{"bob", "ghost"}, "root")
	var unknown store.UnknownUserError
	if !errors.As(err, &unknown) || unknown.OID != "ghost" {
		t.Fatalf("want UnknownUserError(ghost), got %v", err)
	}
	if members, _ := q.ListMembers(ctx, p.ID); len(members) != 1 || members[0].UserOID != "alice" {
		t.Fatalf("a failed call must change nothing: %+v", members)
	}
	if err := q.SetMembers(ctx, "00000000-0000-0000-0000-00000000dead", []string{}, "root"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown project: %v", err)
	}
}

func TestUpdateProject(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	name, desc := "Alpha Two", "about"
	got, err := q.UpdateProject(ctx, p.ID, store.ProjectPatch{Name: &name, Description: &desc})
	if err != nil || got.Name != "Alpha Two" || got.Description != "about" || got.Slug != "alpha" || got.IACPath != "/ws/alpha" {
		t.Fatalf("patch: %+v %v", got, err)
	}
	empty := ""
	got, _ = q.UpdateProject(ctx, p.ID, store.ProjectPatch{Description: &empty})
	if got.Description != "" || got.Name != "Alpha Two" {
		t.Fatalf("clearing description must keep the rest: %+v", got)
	}
	if !got.UpdatedAt.After(p.UpdatedAt) {
		t.Fatal("updated_at must move")
	}
	if _, err := q.UpdateProject(ctx, "00000000-0000-0000-0000-00000000dead", store.ProjectPatch{Name: &name}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown project: %v", err)
	}
}

func TestDeleteProjectCascadesMembersAndKeepsAudit(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	newUser(t, q, "alice")
	q.SetMembers(ctx, p.ID, []string{"alice"}, "root")
	if err := q.LogAudit(ctx, "root", "project.create", "project:"+p.ID, map[string]any{"project_id": p.ID}); err != nil {
		t.Fatal(err)
	}

	cs, err := q.CreateChangeset(ctx, store.Changeset{ProjectID: p.ID, Env: "dev", UserOID: "alice", Status: store.ChangesetPublished, CanvasJSON: []byte(`{"nodes":[],"edges":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := q.CountPendingChangesets(ctx, p.ID); n != 0 {
		t.Fatalf("a draft is not waiting for review: %d", n)
	}
	if err := q.PublishChangeset(ctx, cs, "ready"); err != nil {
		t.Fatal(err)
	}
	if n, _ := q.CountPendingChangesets(ctx, p.ID); n != 1 {
		t.Fatalf("a published changeset is waiting for review: %d", n)
	}

	if err := q.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := q.IsMember(ctx, p.ID, "alice"); ok {
		t.Fatal("membership must go with the project")
	}
	if _, err := q.GetProject(ctx, p.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("project still there: %v", err)
	}
	if users, _ := q.ListUsers(ctx); len(users) != 1 {
		t.Fatal("deleting a project must not delete users")
	}
	if rows, err := q.ListAudit(ctx, p.ID, 10); err != nil || len(rows) != 1 {
		t.Fatalf("audit must survive: %v %v", rows, err)
	}
	if err := q.DeleteProject(ctx, p.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second delete: %v", err)
	}
}
