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

func TestEnvCopyProposesANewNodeAndRefusesACollision(t *testing.T) {
	const mod = "module \"rg\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name   = \"rg-dev\"\n}\n"
	repo := gitprovider.NewMemory(map[string]string{
		"net/dev/main.tf":  mod,
		"net/prod/main.tf": "module \"other\" {\n  source = \"git::x//modules/resource-group?ref=v1\"\n  name = \"o\"\n}\n",
		"odd/x.tf":         "# not in the layout\n",
	})
	repos := Repos(func(context.Context, store.Project) (gitprovider.Source, error) { return repo, nil })
	project := func(_ context.Context, id string) (store.Project, error) {
		return store.Project{ID: id, Slug: "demo"}, nil
	}
	st := SettingsLoader(func(context.Context, string) *hcl.ProjectSettings {
		return &hcl.ProjectSettings{Layout: hcl.Layout{Pattern: "{stack}/{env}"}, Environments: []string{"dev", "prod"}}
	})
	r := chi.NewRouter()
	r.Get("/projects/{id}/env-copy", EnvCopyHandler(project, repos, st))
	r.Get("/projects/{id}/layout/check", LayoutCheckHandler(project, repos, st))
	get := func(target string) (int, map[string]any) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		var b map[string]any
		json.Unmarshal(w.Body.Bytes(), &b)
		return w.Code, b
	}
	code, b := get("/projects/p/env-copy?from=dev&to=prod&id=net.module.rg")
	if code != 200 {
		t.Fatalf("copy = %d %v", code, b)
	}
	n := b["node"].(map[string]any)
	if n["existing"] != nil || n["values"].(map[string]any)["name"] != "rg-prod" || n["target_folder"] != "net" {
		t.Fatalf("node: %v", n)
	}
	if code, _ := get("/projects/p/env-copy?from=prod&to=dev&id=net.module.other"); code != 200 {
		t.Fatalf("the reverse copy = %d", code)
	}
	if code, _ := get("/projects/p/env-copy?from=dev&to=prod&id=net.module.nope"); code != http.StatusNotFound {
		t.Fatalf("unknown resource = %d", code)
	}
	if code, _ := get("/projects/p/env-copy?from=dev&to=dev&id=x"); code != http.StatusBadRequest {
		t.Fatalf("same environment = %d", code)
	}
	// A block already in the target is refused.
	repo2 := gitprovider.NewMemory(map[string]string{"net/dev/main.tf": mod, "net/prod/main.tf": mod})
	repos2 := Repos(func(context.Context, store.Project) (gitprovider.Source, error) { return repo2, nil })
	r2 := chi.NewRouter()
	r2.Get("/projects/{id}/env-copy", EnvCopyHandler(project, repos2, st))
	w := httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/projects/p/env-copy?from=dev&to=prod&id=net.module.rg", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("collision = %d, want 409", w.Code)
	}
	// The check lists the folder that does not fit.
	code, b = get("/projects/p/layout/check?pattern=%7Bstack%7D/%7Benv%7D")
	if code != 200 || b["misfits"].(float64) != 1 {
		t.Fatalf("check = %d %v", code, b)
	}
	if code, _ = get("/projects/p/layout/check?pattern=../x"); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad pattern = %d", code)
	}
}
