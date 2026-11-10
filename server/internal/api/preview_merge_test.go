package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// The preview must show the existing file with the new module added, so the
// diff marks only additions instead of the whole file as deleted.
func TestPreviewMergesNewResourceIntoExistingFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "common"), 0o755)
	os.WriteFile(filepath.Join(dir, "common", "rg.tf"), []byte(rgHand), 0o644)

	mem := NewMemChangesetStore()
	mem.data["c"] = store.Changeset{ID: "c", ProjectID: "p1", Status: store.ChangesetDraft,
		CanvasJSON: canvasWith(newRGNode("resource_group"))}
	project := func(_ context.Context, id string) (store.Project, error) {
		return store.Project{ID: id, IACPath: dir}, nil
	}
	r := chi.NewRouter()
	r.Get("/projects/{id}/changesets/{csId}/preview", ChangesetPreviewHandler(mem, project, LocalRepos(""), nil))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/changesets/c/preview", nil))

	var out struct {
		Files     []struct{ Folder, Filename, Content string } `json:"files"`
		RepoFiles []struct{ Folder, Filename, Content string } `json:"repo_files"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || len(out.Files) != 1 || len(out.RepoFiles) != 1 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(out.Files[0].Content, rgHand) || !strings.Contains(out.Files[0].Content, `module "resource_group"`) {
		t.Fatalf("preview must keep the existing code and add the new module:\n%s", out.Files[0].Content)
	}
}

func TestPreviewReportsDuplicateLabel(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "common"), 0o755)
	os.WriteFile(filepath.Join(dir, "common", "rg.tf"), []byte(rgHand), 0o644)
	mem := NewMemChangesetStore()
	mem.data["c"] = store.Changeset{ID: "c", ProjectID: "p1", Status: store.ChangesetDraft,
		CanvasJSON: canvasWith(newRGNode("rg_app"))}
	project := func(_ context.Context, id string) (store.Project, error) {
		return store.Project{ID: id, IACPath: dir}, nil
	}
	r := chi.NewRouter()
	r.Get("/projects/{id}/changesets/{csId}/preview", ChangesetPreviewHandler(mem, project, LocalRepos(""), nil))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/changesets/c/preview", nil))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "already exists") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
