package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func canvasWith(nodes ...store.CanvasNode) json.RawMessage {
	b, _ := json.Marshal(store.CanvasState{Nodes: nodes})
	return b
}

func TestReviseCopiesCanvasAndPreviewDiffsAgainstBase(t *testing.T) {
	mem := NewMemChangesetStore()
	r := chi.NewRouter()
	r.Route("/projects/{id}/changesets/{csId}", func(cs chi.Router) {
		cs.Get("/preview", ChangesetPreviewHandler(mem, nil, LocalRepos(""), nil))
		cs.Post("/revise", ChangesetReviseHandler(mem))
	})
	do := func(method, path string) (*httptest.ResponseRecorder, map[string]any) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewBuffer(nil)))
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec, out
	}

	sa := store.CanvasNode{ID: "n1", ModuleName: "storage-account", ModuleVersion: "v1", InstanceName: "sa", TargetFolder: "storage",
		Values: map[string]interface{}{"name": "st1", "account_tier": "Standard"}}
	mem.data["old"] = store.Changeset{ID: "old", ProjectID: "p1", Env: "dev", Title: "first", Status: store.ChangesetPublished, CanvasJSON: canvasWith(sa)}
	mem.data["draft"] = store.Changeset{ID: "draft", ProjectID: "p1", Env: "dev", Status: store.ChangesetDraft, CanvasJSON: canvasWith(sa)}

	// A draft is already editable; another project cannot reach the changeset.
	if rec, _ := do("POST", "/projects/p1/changesets/draft/revise"); rec.Code != http.StatusConflict {
		t.Fatalf("revise draft: %d", rec.Code)
	}
	if rec, _ := do("POST", "/projects/other/changesets/old/revise"); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-project revise: %d", rec.Code)
	}

	rec, out := do("POST", "/projects/p1/changesets/old/revise")
	if rec.Code != http.StatusCreated {
		t.Fatalf("revise: %d %s", rec.Code, rec.Body.String())
	}
	id := out["id"].(string)
	rev := mem.data[id]
	if rev.Status != store.ChangesetDraft || rev.Title != "first" || string(rev.BaseCanvas) != string(rev.CanvasJSON) {
		t.Fatalf("revision not a draft copy with a base: %+v", rev)
	}
	if mem.data["old"].Status != store.ChangesetPublished {
		t.Fatal("the original must be left alone")
	}

	// Edit the revision: change a value, add a node. Preview returns both versions.
	sa2 := sa
	sa2.Values = map[string]interface{}{"name": "st1", "account_tier": "Premium"}
	kv := store.CanvasNode{ID: "n2", ModuleName: "key-vault", ModuleVersion: "v1", InstanceName: "kv", TargetFolder: "keyvault",
		Values: map[string]interface{}{"name": "kv1"}}
	rev.CanvasJSON = canvasWith(sa2, kv)
	mem.data[id] = rev

	rec, out = do("GET", "/projects/p1/changesets/"+id+"/preview")
	if rec.Code != 200 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	files, base := out["files"].([]any), out["base_files"].([]any)
	if len(files) != 2 || len(base) != 1 {
		t.Fatalf("want 2 current files and 1 base file, got %d / %d", len(files), len(base))
	}
	if c := base[0].(map[string]any)["content"].(string); !bytes.Contains([]byte(c), []byte("Standard")) {
		t.Fatalf("base must be generated from the base canvas: %s", c)
	}
}

func TestPreviewReportsGenerationErrors(t *testing.T) {
	mem := NewMemChangesetStore()
	orphan := store.CanvasNode{ID: "s", ModuleName: "subnet", InstanceName: "snet", ParentID: "gone", NestField: "subnets",
		Values: map[string]interface{}{"name": "x"}}
	mem.data["c"] = store.Changeset{ID: "c", ProjectID: "p1", Status: store.ChangesetDraft, CanvasJSON: canvasWith(orphan)}
	r := chi.NewRouter()
	r.Get("/projects/{id}/changesets/{csId}/preview", ChangesetPreviewHandler(mem, nil, LocalRepos(""), nil))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/changesets/c/preview", nil))
	if rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte("no parent")) {
		t.Fatalf("want 422 explaining the orphan, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPreviewIncludesExistingRepoFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "vnet", "env"), 0o755)
	os.WriteFile(filepath.Join(dir, "vnet", "vnet.tf"), []byte(`module "vnet" {}`), 0o644)
	os.WriteFile(filepath.Join(dir, "vnet", "notes.txt"), []byte("not terraform"), 0o644)
	os.WriteFile(filepath.Join(dir, "root.tf"), []byte("# root-level files are not part of the folder layout"), 0o644)

	mem := NewMemChangesetStore()
	mem.data["c"] = store.Changeset{ID: "c", ProjectID: "p1", Status: store.ChangesetDraft, CanvasJSON: canvasWith()}
	project := func(_ context.Context, id string) (store.Project, error) {
		return store.Project{ID: id, IACPath: dir}, nil
	}
	r := chi.NewRouter()
	r.Get("/projects/{id}/changesets/{csId}/preview", ChangesetPreviewHandler(mem, project, LocalRepos(""), nil))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/changesets/c/preview", nil))
	var out struct {
		RepoFiles []struct{ Folder, Filename, Content string } `json:"repo_files"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || len(out.RepoFiles) != 1 || out.RepoFiles[0].Folder != "vnet" || out.RepoFiles[0].Filename != "vnet.tf" {
		t.Fatalf("want only vnet/vnet.tf, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestProjectFilesWithoutAnyChangeset(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "kv"), 0o755)
	os.WriteFile(filepath.Join(dir, "kv", "kv.tf"), []byte("# vault"), 0o644)
	project := func(_ context.Context, id string) (store.Project, error) {
		if id != "p1" {
			return store.Project{}, os.ErrNotExist
		}
		return store.Project{ID: id, IACPath: dir}, nil
	}
	r := chi.NewRouter()
	r.Get("/projects/{id}/files", ProjectFilesHandler(project, LocalRepos("")))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/files", nil))
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"kv.tf"`)) {
		t.Fatalf("want kv/kv.tf, got %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/nope/files", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown project: %d", rec.Code)
	}
}
