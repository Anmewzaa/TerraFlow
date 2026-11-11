package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func widgetSchema(version string) json.RawMessage {
	return json.RawMessage(`{
  "title": "Widget", "description": "A widget.", "type": "object",
  "x-tf-module": {"source": "git::https://example.invalid/mods.git//widget", "version": "` + version + `", "target_folder": "widgets"},
  "required": ["name"],
  "properties": {"name": {"type": "string", "title": "Name", "description": "d"}}
}`)
}

func imp(name, version string) map[string]any {
	return map[string]any{"name": name, "schema": widgetSchema(version)}
}

// roleOf: root is the admin of these tests, everyone else a plain user.
func roleOf(user string) string {
	if user == "root" {
		return "admin"
	}
	return "user"
}

// secondProject makes another project next to the one of teamEnv.
func (e *pgEnv) secondProject() string {
	e.t.Helper()
	code, p, raw := e.do("root", "admin", "POST", "/api/projects", map[string]string{"name": "Second"})
	if code != http.StatusCreated {
		e.t.Fatalf("create project: %d %s", code, raw)
	}
	return p["id"].(string)
}

// palette returns "name" -> summary of what a project (or, for "", everyone) sees.
func (e *pgEnv) palette(user, projectID string) map[string]map[string]any {
	e.t.Helper()
	path := "/api/catalog"
	if projectID != "" {
		path = "/api/projects/" + projectID + "/catalog"
	}
	code, _, raw := e.do(user, roleOf(user), "GET", path, nil)
	if code != http.StatusOK {
		e.t.Fatalf("GET %s as %s = %d %s", path, user, code, raw)
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		e.t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, m := range list {
		out[m["name"].(string)] = m
	}
	return out
}

// D-060: the owner and co-owners manage a project's own catalog; contributors use it; nobody
// outside the project sees it.
func TestProjectCatalogBelongsToItsProject(t *testing.T) {
	e, id := teamEnv(t)
	other := e.secondProject()
	base := "/api/projects/" + id + "/catalog"

	if code, _, raw := e.do("bob", "user", "POST", base+"/modules", imp("team-widget", "v1")); code != http.StatusForbidden {
		t.Fatalf("a contributor importing = %d %s, want 403", code, raw)
	}
	if code, _, _ := e.do("bob", "user", "GET", base+"/modules", nil); code != http.StatusForbidden {
		t.Fatalf("a contributor reading the management list = %d, want 403", code)
	}
	if code, _, _ := e.do("mallory", "user", "POST", base+"/modules", imp("team-widget", "v1")); code != http.StatusNotFound {
		t.Fatalf("an outsider importing = %d, want 404", code)
	}
	if code, res, raw := e.do("alice", "user", "POST", base+"/validate", imp("team-widget", "v1")); code != http.StatusOK || res["exists"] != "none" {
		t.Fatalf("validate = %d %s", code, raw)
	}
	if len(e.palette("bob", id)) != 0 {
		t.Fatal("validating must not save anything")
	}
	if code, _, raw := e.do("alice", "user", "POST", base+"/modules", imp("team-widget", "v1")); code != http.StatusCreated {
		t.Fatalf("the owner importing = %d %s, want 201", code, raw)
	}
	if code, _, raw := e.do("carol", "user", "POST", base+"/modules", imp("team-widget", "v2")); code != http.StatusCreated {
		t.Fatalf("a co-owner importing = %d %s, want 201", code, raw)
	}

	// The contributor sees it in the project, marked as the project's own.
	got := e.palette("bob", id)["team-widget"]
	if got == nil || got["scope"] != "project" || len(got["versions"].([]any)) != 2 {
		t.Fatalf("the project's palette: %+v", got)
	}
	if code, m, raw := e.do("bob", "user", "GET", base+"/team-widget/v1", nil); code != http.StatusOK || m["name"] != "team-widget" {
		t.Fatalf("a member reading the schema = %d %s", code, raw)
	}
	// Nobody else does.
	if _, ok := e.palette("bob", "")["team-widget"]; ok {
		t.Fatal("a project's module must not be in the global catalog")
	}
	if code, _, _ := e.do("bob", "user", "GET", "/api/catalog/team-widget/v1", nil); code != http.StatusNotFound {
		t.Fatalf("the global catalog serving a project's module = %d", code)
	}
	if _, ok := e.palette("root", other)["team-widget"]; ok {
		t.Fatal("a project's module leaked into another project")
	}
	if code, _, _ := e.do("mallory", "user", "GET", base, nil); code != http.StatusNotFound {
		t.Fatalf("an outsider reading the project's palette = %d, want 404", code)
	}

	// Deprecating takes a version out of the palette, and only this project's.
	if code, _, raw := e.do("carol", "user", "PATCH", base+"/modules/team-widget/v2", map[string]string{"status": "deprecated"}); code != http.StatusOK {
		t.Fatalf("a co-owner deprecating = %d %s", code, raw)
	}
	if vs := e.palette("bob", id)["team-widget"]["versions"].([]any); len(vs) != 1 || vs[0] != "v1" {
		t.Fatalf("after deprecating v2: %v", vs)
	}
	if code, _, _ := e.do("root", "admin", "PATCH", "/api/admin/catalog/modules/team-widget/v1", map[string]string{"status": "deprecated"}); code != http.StatusNotFound {
		t.Fatalf("the admin catalog must not reach a project's module: %d", code)
	}

	// The project's audit trail has the imports.
	_, _, raw := e.do("alice", "user", "GET", "/api/projects/"+id+"/audit", nil)
	if !strings.Contains(string(raw), "catalog.import") || !strings.Contains(string(raw), "catalog.deprecated") {
		t.Fatalf("the project's audit must record catalog changes: %s", raw)
	}

	// Deleting the project takes its catalog with it.
	if code, _, raw := e.do("root", "admin", "DELETE", "/api/projects/"+id, nil); code >= 300 {
		t.Fatalf("delete project = %d %s", code, raw)
	}
	if mods, err := e.q.ListCatalogModulesIn(t.Context(), id); err != nil || len(mods) != 0 {
		t.Fatalf("a deleted project's modules must be gone: %d %v", len(mods), err)
	}
}

func TestProjectModuleNeverShadowsAGlobalOneSilently(t *testing.T) {
	e, id := teamEnv(t)
	base := "/api/projects/" + id + "/catalog"
	if code, _, raw := e.do("root", "admin", "POST", "/api/admin/catalog/modules", imp("widget", "v1")); code != http.StatusCreated {
		t.Fatalf("global import = %d %s", code, raw)
	}

	// A project may not take a global name.
	code, res, raw := e.do("alice", "user", "POST", base+"/modules", imp("widget", "v5"))
	if code != http.StatusUnprocessableEntity || !strings.Contains(string(raw), "global_name") {
		t.Fatalf("taking a global name = %d %s, want 422 with global_name", code, raw)
	}
	_ = res
	if code, v, _ := e.do("alice", "user", "POST", base+"/validate", imp("widget", "v5")); code != http.StatusOK || v["report"].(map[string]any)["valid"] != false {
		t.Fatalf("the live check must say so before saving: %d %+v", code, v)
	}
	if got := e.palette("bob", id)["widget"]; got["scope"] != "global" || len(got["versions"].([]any)) != 1 {
		t.Fatalf("the global module is unchanged in the project: %+v", got)
	}

	// The other way round: the project had the name first, the admin adds it later. The project
	// keeps its own module (its canvases do not change meaning) and the palette says so.
	if code, _, raw := e.do("alice", "user", "POST", base+"/modules", imp("team-widget", "v1")); code != http.StatusCreated {
		t.Fatalf("project import = %d %s", code, raw)
	}
	if code, _, raw := e.do("root", "admin", "POST", "/api/admin/catalog/modules", imp("team-widget", "v9")); code != http.StatusCreated {
		t.Fatalf("a project's name must not block the admin: %d %s", code, raw)
	}
	got := e.palette("bob", id)["team-widget"]
	if got["scope"] != "project" || got["shadows_global"] != true || len(got["versions"].([]any)) != 1 {
		t.Fatalf("the project's module wins and is marked: %+v", got)
	}
	if code, _, _ := e.do("bob", "user", "GET", base+"/team-widget/v9", nil); code != http.StatusNotFound {
		t.Fatalf("the hidden global version must not be served to the project: %d", code)
	}
	if code, _, raw := e.do("alice", "user", "POST", base+"/modules", imp("team-widget", "v2")); code != http.StatusCreated {
		t.Fatalf("the project can still version a name it owns: %d %s", code, raw)
	}
	if got := e.palette("bob", "")["team-widget"]; got["scope"] != "global" {
		t.Fatalf("outside the project the global one is used: %+v", got)
	}
}

func TestProjectRawResourcesFollowTheSameScopes(t *testing.T) {
	e, id := teamEnv(t)
	other := e.secondProject()
	base := "/api/projects/" + id + "/catalog/resources"
	tpl := func(name string) map[string]any {
		return map[string]any{"name": name, "title": "T " + name, "resource_type": "azurerm_resource_group"}
	}
	list := func(user, path string) map[string]map[string]any {
		t.Helper()
		code, _, raw := e.do(user, roleOf(user), "GET", path, nil)
		if code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, code, raw)
		}
		var l []map[string]any
		json.Unmarshal(raw, &l)
		out := map[string]map[string]any{}
		for _, r := range l {
			out[r["name"].(string)] = r
		}
		return out
	}
	if code, _, raw := e.do("root", "admin", "POST", "/api/admin/resources", tpl("shared-rg")); code != http.StatusCreated {
		t.Fatalf("global template = %d %s", code, raw)
	}
	if code, _, _ := e.do("bob", "user", "POST", base, tpl("team-rg")); code != http.StatusForbidden {
		t.Fatalf("a contributor creating = %d, want 403", code)
	}
	if code, _, raw := e.do("alice", "user", "POST", base, tpl("team-rg")); code != http.StatusCreated {
		t.Fatalf("the owner creating = %d %s", code, raw)
	}
	if code, _, raw := e.do("alice", "user", "POST", base, tpl("shared-rg")); code != http.StatusConflict {
		t.Fatalf("taking a global name = %d %s, want 409", code, raw)
	}
	mine := list("bob", base)
	if len(mine) != 2 || mine["team-rg"]["scope"] != "project" || mine["shared-rg"]["scope"] != "global" {
		t.Fatalf("the project's templates: %+v", mine)
	}
	if _, ok := list("bob", "/api/catalog/resources")["team-rg"]; ok {
		t.Fatal("a project's template must not be in the global list")
	}
	if _, ok := list("root", "/api/projects/"+other+"/catalog/resources")["team-rg"]; ok {
		t.Fatal("a project's template leaked into another project")
	}
	upd := tpl("team-rg")
	upd["title"] = "Renamed"
	if code, _, raw := e.do("carol", "user", "PUT", base+"/team-rg", upd); code != http.StatusOK {
		t.Fatalf("a co-owner updating = %d %s", code, raw)
	}
	if code, _, _ := e.do("carol", "user", "PUT", base+"/shared-rg", tpl("shared-rg")); code != http.StatusNotFound {
		t.Fatalf("a project must not change a global template: %d", code)
	}
	if code, _, _ := e.do("root", "admin", "DELETE", "/api/admin/resources/team-rg", nil); code != http.StatusNotFound {
		t.Fatalf("the admin routes must not reach a project's template: %d", code)
	}
	if code, _, _ := e.do("bob", "user", "DELETE", base+"/team-rg", nil); code != http.StatusForbidden {
		t.Fatalf("a contributor deleting = %d, want 403", code)
	}
	if code, _, raw := e.do("alice", "user", "DELETE", base+"/team-rg", nil); code != http.StatusNoContent {
		t.Fatalf("the owner deleting = %d %s", code, raw)
	}
	if _, ok := list("bob", base)["shared-rg"]; !ok || len(list("bob", base)) != 1 {
		t.Fatal("the global template must survive")
	}
}

func TestSchemaDraftFromVariables(t *testing.T) {
	e, id := teamEnv(t)
	body := map[string]string{"name": "team-widget", "variables": "variable \"name\" {\n  type = string\n}\n"}
	if code, _, _ := e.do("bob", "user", "POST", "/api/projects/"+id+"/catalog/draft", body); code != http.StatusForbidden {
		t.Fatalf("a contributor drafting = %d, want 403", code)
	}
	code, res, raw := e.do("alice", "user", "POST", "/api/projects/"+id+"/catalog/draft", body)
	if code != http.StatusOK || res["schema"] == nil {
		t.Fatalf("draft = %d %s", code, raw)
	}
	// The draft is importable as it is.
	if code, v, raw := e.do("alice", "user", "POST", "/api/projects/"+id+"/catalog/validate", map[string]any{"name": "team-widget", "schema": res["schema"]}); code != http.StatusOK || v["report"].(map[string]any)["valid"] != true {
		t.Fatalf("the draft must pass the check: %d %s", code, raw)
	}
	body["variables"] = "variable \"x\" {"
	if code, _, _ := e.do("alice", "user", "POST", "/api/projects/"+id+"/catalog/draft", body); code != http.StatusBadRequest {
		t.Fatalf("broken Terraform = %d, want 400", code)
	}
}
