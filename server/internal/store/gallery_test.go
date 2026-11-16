package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func mkGallery(name, author, scope, status string, modules ...string) store.GalleryTemplate {
	return store.GalleryTemplate{Name: name, Description: "d " + name, Category: "Network", Author: author, AuthorName: author + " name",
		Scope: scope, Status: status, Template: json.RawMessage(`{"name":"` + name + `"}`), Modules: modules}
}

func TestGalleryVisibilityIsDecidedInTheQuery(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	mustCreate := func(g store.GalleryTemplate) store.GalleryTemplate {
		c, err := q.CreateGalleryTemplate(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	pub := mustCreate(mkGallery("public ok", "alice", store.GalleryPublic, store.GalleryApproved, "vnet"))
	mustCreate(mkGallery("public waiting", "alice", store.GalleryPublic, store.GalleryPending, "vnet"))
	mustCreate(mkGallery("alice private", "alice", store.GalleryPrivate, store.GalleryApproved))
	mustCreate(mkGallery("bob private", "bob", store.GalleryPrivate, store.GalleryApproved))
	mustCreate(mkGallery("hidden", "bob", store.GalleryPublic, store.GalleryHidden))

	names := func(f store.GalleryFilter) map[string]bool {
		l, err := q.ListGalleryTemplates(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, g := range l {
			m[g.Name] = true
			if len(g.Template) != 0 {
				t.Fatal("a list carries no documents")
			}
		}
		return m
	}
	if got := names(store.GalleryFilter{Viewer: "carol"}); len(got) != 1 || !got["public ok"] {
		t.Fatalf("a stranger sees only approved public templates: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "alice"}); len(got) != 3 || !got["public waiting"] || !got["alice private"] || got["bob private"] {
		t.Fatalf("an author also sees their own: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "alice", Mine: true}); len(got) != 3 {
		t.Fatalf("mine: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "root", Admin: true}); len(got) != 5 {
		t.Fatalf("an admin sees all: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "root", Admin: true, Status: store.GalleryPending}); len(got) != 1 || !got["public waiting"] {
		t.Fatalf("the review queue: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "carol", Query: "VNET"}); len(got) != 1 {
		t.Fatalf("search matches module names without case: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "carol", Query: "%"}); len(got) != 0 {
		t.Fatalf("a percent sign is not a wildcard: %v", got)
	}
	if got := names(store.GalleryFilter{Viewer: "alice", Category: "Other"}); len(got) != 0 {
		t.Fatalf("category: %v", got)
	}

	if _, err := q.CreateGalleryTemplate(ctx, mkGallery("public ok", "alice", store.GalleryPrivate, store.GalleryApproved)); !errors.Is(err, store.ErrGalleryExists) {
		t.Fatalf("a name is unique per author: %v", err)
	}
	if _, err := q.CreateGalleryTemplate(ctx, mkGallery("public ok", "bob", store.GalleryPrivate, store.GalleryApproved)); err != nil {
		t.Fatalf("another author may use the same name: %v", err)
	}

	got, err := q.GetGalleryTemplate(ctx, pub.ID)
	if err != nil || string(got.Template) != `{"name": "public ok"}` || got.Version != 1 || len(got.Modules) != 1 {
		t.Fatalf("get: %+v %v", got, err)
	}
}

func TestGalleryEditReviewImportAndDelete(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	proj, err := q.CreateProject(ctx, store.Project{Name: "P", Slug: "p"})
	if err != nil {
		t.Fatal(err)
	}
	g, err := q.CreateGalleryTemplate(ctx, mkGallery("t", "alice", store.GalleryPublic, store.GalleryPending))
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetGalleryStatus(ctx, g.ID, store.GalleryRejected, "too specific", "root"); err != nil {
		t.Fatal(err)
	}
	g2, _ := q.GetGalleryTemplate(ctx, g.ID)
	if g2.Status != store.GalleryRejected || g2.ReviewNote != "too specific" || g2.ReviewedBy != "root" {
		t.Fatalf("review: %+v", g2)
	}
	edit := mkGallery("t2", "alice", store.GalleryPublic, store.GalleryPending, "subnet")
	if err := q.UpdateGalleryTemplate(ctx, g.ID, edit); err != nil {
		t.Fatal(err)
	}
	g3, _ := q.GetGalleryTemplate(ctx, g.ID)
	if g3.Version != 2 || g3.Status != store.GalleryPending || g3.ReviewNote != "" || g3.Name != "t2" || g3.Author != "alice" {
		t.Fatalf("an edit raises the version, clears the review and keeps the author: %+v", g3)
	}

	for _, by := range []string{"u1", "u2"} {
		if err := q.RecordGalleryImport(ctx, g.ID, proj.ID, by); err != nil {
			t.Fatal(err)
		}
	}
	g4, _ := q.GetGalleryTemplate(ctx, g.ID)
	if g4.Imports != 2 || g4.Projects != 1 {
		t.Fatalf("imported twice into one project: %+v", g4)
	}

	if err := q.DeleteGalleryTemplate(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetGalleryTemplate(ctx, g.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("deleted: %v", err)
	}
	if err := q.DeleteGalleryTemplate(ctx, g.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("deleting again: %v", err)
	}
	if err := q.SetGalleryStatus(ctx, g.ID, store.GalleryHidden, "", ""); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("status of a missing template: %v", err)
	}
}
