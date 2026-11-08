package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func mod(name, version, checksum string) store.CatalogModule {
	return store.CatalogModule{
		Name: name, Version: version, Title: name, TargetFolder: "f",
		SchemaJSON: []byte(`{"title":"` + name + `"}`), Checksum: checksum,
	}
}

func TestInsertCatalogModuleNeverOverwrites(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()

	m := mod("widget", "v1", "aaa")
	m.ImportedBy = "root"
	if err := q.InsertCatalogModule(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetCatalogModule(ctx, "widget", "v1")
	if err != nil || got.Status != "published" || got.ImportedBy != "root" {
		t.Fatalf("a new module is published and remembers who imported it: %+v %v", got, err)
	}
	if err := q.InsertCatalogModule(ctx, mod("widget", "v1", "bbb")); !errors.Is(err, store.ErrModuleExists) {
		t.Fatalf("same name+version: %v", err)
	}
	if again, _ := q.GetCatalogModule(ctx, "widget", "v1"); again.Checksum != "aaa" {
		t.Fatal("a refused insert must not change the stored schema")
	}
	if err := q.InsertCatalogModule(ctx, mod("widget", "v2", "ccc")); err != nil {
		t.Fatalf("another version of the same module: %v", err)
	}
}

func TestCatalogStatusSurvivesSeedRefresh(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	if err := q.UpsertCatalogModule(ctx, mod("widget", "v1", "aaa")); err != nil {
		t.Fatal(err)
	}
	if err := q.SetCatalogModuleStatus(ctx, "widget", "v1", "deprecated"); err != nil {
		t.Fatal(err)
	}
	// The app re-seeds catalog/modules on every start; that must not undo an admin's decision.
	if err := q.UpsertCatalogModule(ctx, mod("widget", "v1", "bbb")); err != nil {
		t.Fatal(err)
	}
	got, _ := q.GetCatalogModule(ctx, "widget", "v1")
	if got.Status != "deprecated" || got.Checksum != "bbb" {
		t.Fatalf("seed refresh keeps the status and updates the schema: %+v", got)
	}

	if err := q.SetCatalogModuleStatus(ctx, "widget", "v1", "published"); err != nil {
		t.Fatal(err)
	}
	if got, _ = q.GetCatalogModule(ctx, "widget", "v1"); got.Status != "published" {
		t.Fatalf("republish: %+v", got)
	}
	if err := q.SetCatalogModuleStatus(ctx, "widget", "v9", "deprecated"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown version: %v", err)
	}
	if err := q.SetCatalogModuleStatus(ctx, "widget", "v1", "archived"); err == nil {
		t.Fatal("the database refuses a status other than published or deprecated")
	}
}

func TestComponentConfig(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	p := newProject(t, q, "alpha")
	other := newProject(t, q, "beta")

	if list, err := q.ListComponentConfig(ctx, p.ID); err != nil || list == nil || len(list) != 0 {
		t.Fatalf("empty list must be non-nil: %#v %v", list, err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(q.SetComponentConfig(ctx, p.ID, "common/rg_app", "cyan", "alice"))
	must(q.SetComponentConfig(ctx, p.ID, "common/rg_data", "red", "alice"))
	must(q.SetComponentConfig(ctx, other.ID, "common/rg_app", "green", "bob"))

	must(q.SetComponentConfig(ctx, p.ID, "common/rg_app", "blue", "carol")) // overwrite
	list, _ := q.ListComponentConfig(ctx, p.ID)
	if len(list) != 2 || list[0].Address != "common/rg_app" || list[0].Color != "blue" || list[0].UpdatedBy != "carol" || list[1].Color != "red" {
		t.Fatalf("list: %+v", list)
	}
	if o, _ := q.ListComponentConfig(ctx, other.ID); len(o) != 1 || o[0].Color != "green" {
		t.Fatalf("projects must not see each other's settings: %+v", o)
	}

	must(q.SetComponentConfig(ctx, p.ID, "common/rg_data", "", "alice")) // empty colour removes
	if list, _ = q.ListComponentConfig(ctx, p.ID); len(list) != 1 {
		t.Fatalf("remove: %+v", list)
	}
	must(q.SetComponentConfig(ctx, p.ID, "common/never_set", "", "alice")) // removing nothing is fine

	must(q.DeleteProject(ctx, p.ID))
	if rows, err := q.ListComponentConfig(ctx, p.ID); err != nil || len(rows) != 0 {
		t.Fatalf("settings go with the project: %v %v", rows, err)
	}
	if o, _ := q.ListComponentConfig(ctx, other.ID); len(o) != 1 {
		t.Fatal("deleting one project must not touch another's settings")
	}
}
