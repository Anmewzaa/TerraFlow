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
	summaries, err := svc.ListModuleSummaries(ctx, "")
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
	_, err := svc.GetModule(context.Background(), "", "nonexistent", "v1")
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

// D-060: a project sees the global catalog plus its own modules, and a name belongs to one
// scope inside a project.
func TestAProjectSeesGlobalPlusItsOwnModules(t *testing.T) {
	ctx := context.Background()
	st := NewFSStore(t.TempDir())
	add := func(project, name, version string) {
		t.Helper()
		if err := st.InsertModule(ctx, Module{ProjectID: project, Name: name, Version: version, Title: name, SchemaJSON: json.RawMessage(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	add("", "shared", "v1")
	add("", "both", "v1")
	add("", "both", "v2")
	add("p1", "mine", "v1")
	add("p1", "both", "v7")
	add("p2", "other", "v1")
	svc := NewService(st)

	names := func(project string) map[string]ModuleSummary {
		t.Helper()
		list, err := svc.ListModuleSummaries(ctx, project)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]ModuleSummary{}
		for _, s := range list {
			out[s.Name] = s
		}
		return out
	}
	if g := names(""); len(g) != 2 || g["shared"].Scope != ScopeGlobal || len(g["both"].Versions) != 2 {
		t.Fatalf("the global catalog alone: %+v", g)
	}
	p1 := names("p1")
	if len(p1) != 3 || p1["mine"].Scope != ScopeProject || p1["shared"].Scope != ScopeGlobal {
		t.Fatalf("project 1 sees global and its own, never another project's: %+v", p1)
	}
	if b := p1["both"]; b.Scope != ScopeProject || !b.ShadowsGlobal || len(b.Versions) != 1 || b.Versions[0] != "v7" {
		t.Fatalf("a name the project has comes from the project only, and says it hides a global one: %+v", b)
	}
	if p1["mine"].ShadowsGlobal {
		t.Error("a name only the project has hides nothing")
	}
	if _, ok := names("p2")["mine"]; ok {
		t.Error("a project's module leaked into another project")
	}

	if m, err := svc.GetModule(ctx, "p1", "both", "v7"); err != nil || m.ProjectID != "p1" {
		t.Errorf("the project's version: %+v %v", m, err)
	}
	if _, err := svc.GetModule(ctx, "p1", "both", "v1"); err == nil {
		t.Error("a global version of a name the project owns must not be served to the project")
	}
	if m, err := svc.GetModule(ctx, "p1", "shared", "v1"); err != nil || m.ProjectID != "" {
		t.Errorf("a global module is served to a project: %+v %v", m, err)
	}
	if _, err := svc.GetModule(ctx, "", "mine", "v1"); err == nil {
		t.Error("the global catalog must not serve a project's module")
	}
	if _, err := svc.GetModule(ctx, "p2", "mine", "v1"); err == nil {
		t.Error("a project must not read another project's module")
	}
	// The same name and version may exist once per scope.
	if err := st.InsertModule(ctx, Module{ProjectID: "p1", Name: "mine", Version: "v1"}); err != ErrModuleExists {
		t.Errorf("same scope, name and version: %v", err)
	}
	if err := st.InsertModule(ctx, Module{ProjectID: "p2", Name: "mine", Version: "v1"}); err != nil {
		t.Errorf("another project may use the same name: %v", err)
	}
}
