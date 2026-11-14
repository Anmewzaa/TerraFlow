package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func TestEnvDiffEndpoint(t *testing.T) {
	const mod = "module \"rg\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name   = \"rg-${var.size}\"\n}\n"
	repo := gitprovider.NewMemory(map[string]string{
		"net/main.tf":                 "variable \"size\" {}\n" + mod,
		"net/env/dev.tfvars":          "size = \"s\"\n",
		"net/env/prod.tfvars":         "size = \"p\"\n",
		"net/extra.tf":                "module \"only\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name = \"o\"\n}\n",
		"net/ignored/README.md":       "x",
		"net/ignored/nothing.tfstate": "x",
	})
	repos := Repos(func(context.Context, store.Project) (gitprovider.Source, error) { return repo, nil })
	project := func(_ context.Context, id string) (store.Project, error) {
		return store.Project{ID: id, Slug: "demo"}, nil
	}
	r := chi.NewRouter()
	r.Get("/projects/{id}/env-diff", EnvDiffHandler(project, repos, SettingsLoader(func(context.Context, string) *hcl.ProjectSettings { return nil })))

	get := func(q string) (int, hcl.EnvDiff) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/projects/p1/env-diff"+q, nil))
		var d hcl.EnvDiff
		json.Unmarshal(w.Body.Bytes(), &d)
		return w.Code, d
	}
	for _, q := range []string{"", "?from=dev", "?from=dev&to=dev", "?from=dev&to=../x"} {
		if code, _ := get(q); code != http.StatusBadRequest {
			t.Errorf("%q = %d, want 400", q, code)
		}
	}
	code, d := get("?from=dev&to=prod")
	if code != 200 {
		t.Fatalf("diff = %d", code)
	}
	if d.Summary.Differs != 1 || d.Summary.Same != 1 {
		t.Fatalf("rg differs by its name, the other block is the same in both: %+v", d.Summary)
	}
}
