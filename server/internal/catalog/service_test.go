package catalog

import (
	"context"
	"encoding/json"
	"testing"
)

func TestListModuleSummaries(t *testing.T) {
	store := NewFSStore(t.TempDir())
	ctx := context.Background()

	store.UpsertModule(ctx, Module{
		Name: "mod-a", Version: "v1", Title: "Module A", Description: "First",
		Icon: "A", SchemaJSON: json.RawMessage(`{}`), Checksum: "aaa",
	})
	store.UpsertModule(ctx, Module{
		Name: "mod-a", Version: "v2", Title: "Module A", Description: "First",
		Icon: "A", SchemaJSON: json.RawMessage(`{}`), Checksum: "bbb",
	})
	store.UpsertModule(ctx, Module{
		Name: "mod-b", Version: "v1", Title: "Module B", Description: "Second",
		Icon: "B", SchemaJSON: json.RawMessage(`{}`), Checksum: "ccc",
	})

	svc := NewService(store)
	summaries, err := svc.ListModuleSummaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	a := summaries[0]
	if a.Name != "mod-a" {
		t.Errorf("name = %q, want %q", a.Name, "mod-a")
	}
	if len(a.Versions) != 2 {
		t.Errorf("versions = %v, want 2", a.Versions)
	}
	if a.LatestVersion != "v2" {
		t.Errorf("latest = %q, want %q", a.LatestVersion, "v2")
	}
}

func TestGetModuleNotFound(t *testing.T) {
	store := NewFSStore(t.TempDir())
	svc := NewService(store)
	_, err := svc.GetModule(context.Background(), "nonexistent", "v1")
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
}

func TestDraftRoundTrip(t *testing.T) {
	store := NewFSStore(t.TempDir())
	svc := NewService(store)
	ctx := context.Background()

	d := Draft{
		ProjectID:  "proj-1",
		ModuleID:   "mod-1",
		UserOID:    "user-1",
		ValuesJSON: json.RawMessage(`{"name":"test"}`),
	}
	if err := svc.SaveDraft(ctx, d); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetDraft(ctx, "proj-1", "mod-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	var vals map[string]string
	json.Unmarshal(got.ValuesJSON, &vals)
	if vals["name"] != "test" {
		t.Errorf("draft value = %q, want %q", vals["name"], "test")
	}

	drafts, err := svc.ListDrafts(ctx, "proj-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}
}
