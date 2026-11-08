package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

const rgHand = `# hand written resource groups
module "rg_network" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name     = local.rg_names.network
  location = var.location
}

module "rg_app" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name     = local.rg_names.app
  location = var.location
}
`

func (e *reviewEnv) writeRepo(rel, content string) {
	e.t.Helper()
	p := filepath.Join(e.dir, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *reviewEnv) seedCanvas(id string, nodes ...store.CanvasNode) {
	e.mem.data[id] = store.Changeset{
		ID: id, ProjectID: "p1", Env: "dev", UserOID: "dev-1", Title: "change",
		Status: store.ChangesetPublished, CanvasJSON: canvasWith(nodes...),
	}
}

func (e *reviewEnv) show(branch, path string) string {
	e.t.Helper()
	c := exec.Command("git", "show", branch+":"+path)
	c.Dir = e.dir
	b, err := c.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git show %s:%s: %v %s", branch, path, err, b)
	}
	return string(b)
}

func newRGNode(label string) store.CanvasNode {
	return store.CanvasNode{
		ID: "n-" + label, ModuleName: "resource-group", ModuleVersion: "v1.2.0", InstanceName: label,
		TargetFolder: "common", Filename: "rg.tf",
		Values: map[string]interface{}{"name": "test", "location": "southeastasia"},
	}
}

// The incident: adding a resource group to a folder whose rg.tf already holds
// hand-written modules must keep them, in the review and on the branch.
func TestApproveAppendsToExistingFileKeepingHandWrittenModules(t *testing.T) {
	e := newReviewEnv(t, false)
	e.writeRepo("common/rg.tf", rgHand)
	e.seedCanvas("c1", newRGNode("resource_group"))

	rec, out := e.do("GET", "/projects/p1/changesets/c1/review", "")
	diff := out["hcl_diff"].([]any)
	if rec.Code != 200 || len(diff) != 1 || diff[0].(map[string]any)["status"] != "modified" {
		t.Fatalf("review: %d %v", rec.Code, out)
	}

	rec, out = e.do("POST", "/projects/p1/changesets/c1/approve", "")
	if rec.Code != 200 {
		t.Fatalf("approve: %d %v", rec.Code, out)
	}
	got := e.show(out["branch"].(string), "common/rg.tf")
	if !strings.HasPrefix(got, rgHand) {
		t.Fatalf("hand-written content changed on the branch:\n%s", got)
	}
	if !strings.Contains(got, `module "resource_group"`) {
		t.Fatalf("new module missing:\n%s", got)
	}
	// The checked-out working copy is never touched.
	if disk, _ := os.ReadFile(filepath.Join(e.dir, "common", "rg.tf")); string(disk) != rgHand {
		t.Fatal("approve modified the working tree")
	}
}

func TestApproveEditsExistingResourceInPlace(t *testing.T) {
	e := newReviewEnv(t, false)
	e.writeRepo("common/rg.tf", rgHand)
	n := newRGNode("rg_app")
	n.Values = map[string]interface{}{"name": "rg-renamed", "location": "japaneast"}
	n.Existing = &store.ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	e.seedCanvas("c1", n)

	rec, out := e.do("POST", "/projects/p1/changesets/c1/approve", "")
	if rec.Code != 200 {
		t.Fatalf("approve: %d %v", rec.Code, out)
	}
	got := e.show(out["branch"].(string), "common/rg.tf")
	i := strings.Index(rgHand, `module "rg_app"`)
	if !strings.HasPrefix(got, rgHand[:i]) {
		t.Fatalf("content before the edited block changed:\n%s", got)
	}
	if !strings.Contains(got, `"rg-renamed"`) || strings.Contains(got, "local.rg_names.app") {
		t.Fatalf("edit not applied:\n%s", got)
	}
	if !strings.Contains(got, "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0") {
		t.Fatal("the repository's own source line must be kept")
	}
	if !strings.Contains(got, "local.rg_names.network") {
		t.Fatal("untouched module lost")
	}
}

func TestApproveReportsErrorsInsteadOfOverwriting(t *testing.T) {
	e := newReviewEnv(t, false)
	e.writeRepo("common/rg.tf", rgHand)

	// Duplicate label: 422 with a readable reason, repository untouched.
	e.seedCanvas("dup", newRGNode("rg_app"))
	rec, out := e.do("POST", "/projects/p1/changesets/dup/approve", "")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(out["error"].(string), "already exists") {
		t.Fatalf("duplicate label: %d %v", rec.Code, out)
	}
	if rec, _ := e.do("GET", "/projects/p1/changesets/dup/review", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("review of an invalid changeset: %d", rec.Code)
	}

	// Edit whose target was removed from the repo since the draft was made.
	gone := newRGNode("rg_gone")
	gone.Existing = &store.ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_gone"}
	e.seedCanvas("gone", gone)
	if rec, _ := e.do("POST", "/projects/p1/changesets/gone/approve", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing edit target: %d", rec.Code)
	}
	if st := e.mem.data["dup"].Status; st != store.ChangesetPublished {
		t.Fatalf("a failed approve must leave the changeset published, got %s", st)
	}
	if _, err := os.Stat(filepath.Join(e.dir, ".git")); err == nil {
		c := exec.Command("git", "branch", "--list", "terraflow/*")
		c.Dir = e.dir
		if b, _ := c.CombinedOutput(); strings.TrimSpace(string(b)) != "" {
			t.Fatalf("a failed approve created a branch: %s", b)
		}
	}
}
