package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func importRouter(iac string) http.Handler {
	r := chi.NewRouter()
	r.Get("/projects/{id}/import", ProjectImportHandler(func(_ context.Context, id string) (store.Project, error) {
		return store.Project{ID: id, IACPath: iac}, nil
	}, LocalRepos("")))
	return r
}

func TestProjectImport_ReturnsExistingNodes(t *testing.T) {
	iac, _ := filepath.Abs("../../../samples/iac/AIH")
	rec := httptest.NewRecorder()
	importRouter(iac).ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/import", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Nodes []struct {
			ID       string `json:"id"`
			Existing *struct {
				Folder, Filename, Label string
			} `json:"existing"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 6 {
		t.Fatalf("want 6 module calls in AIH, got %d", len(out.Nodes))
	}
	for _, n := range out.Nodes {
		if n.Existing == nil || n.Existing.Label == "" {
			t.Errorf("node %s is not linked to its block", n.ID)
		}
	}
}

func TestProjectImport_EmptyFolderIsEmptyList(t *testing.T) {
	rec := httptest.NewRecorder()
	importRouter(t.TempDir()).ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/import", nil))
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) || !strings.Contains(strings.ReplaceAll(rec.Body.String(), " ", ""), `"nodes":[]`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestProjectImport_UnparseableFileIsReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "net", "main.tf"), "module \"x\" {")
	rec := httptest.NewRecorder()
	importRouter(dir).ServeHTTP(rec, httptest.NewRequest("GET", "/projects/p1/import", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d %s", rec.Code, rec.Body)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
