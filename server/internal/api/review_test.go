package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type reviewEnv struct {
	t      *testing.T
	router chi.Router
	mem    *MemChangesetStore
	rs     *MemReviewStore
	dir    string
	role   string
	oid    string
}

func newReviewEnv(t *testing.T, sod bool) *reviewEnv {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ws := t.TempDir()
	dir := filepath.Join(ws, "demo")
	os.MkdirAll(dir, 0o755)

	mem := NewMemChangesetStore()
	rs := NewMemReviewStore(mem)
	e := &reviewEnv{t: t, mem: mem, rs: rs, dir: dir, role: "devops", oid: "reviewer-1"}
	rv := &Reviewer{
		Store: rs, Repos: LocalRepos(ws), SeparationOfDuties: sod,
		Project: func(_ context.Context, id string) (store.Project, error) {
			return store.Project{ID: id, Slug: "demo", IACPath: dir}, nil
		},
	}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			u := &auth.User{OID: e.oid, Roles: []string{e.role}}
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), u)))
		})
	})
	r.Route("/projects/{id}/changesets/{csId}", func(cs chi.Router) {
		cs.Get("/review", rv.Detail())
		cs.Post("/approve", rv.Approve())
		cs.Post("/reject", rv.Reject())
	})
	r.Get("/projects/{id}/audit", rv.Audit())
	e.router = r
	return e
}

func (e *reviewEnv) seed(id string, status store.ChangesetStatus, requester string) {
	canvas, _ := json.Marshal(store.CanvasState{Nodes: []store.CanvasNode{{
		ID: "n1", ModuleName: "storage-account", ModuleVersion: "v1", InstanceName: "sa_demo",
		TargetFolder: "storage", Values: map[string]interface{}{"name": "stdemo01", "account_tier": "Standard"},
	}}})
	e.mem.data[id] = store.Changeset{
		ID: id, ProjectID: "p1", Env: "dev", UserOID: requester, Title: "Add storage",
		Status: status, CanvasJSON: canvas,
	}
}

func (e *reviewEnv) do(method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestApproveCreatesLocalBranchAndAudits(t *testing.T) {
	e := newReviewEnv(t, false)
	e.seed("abcdef12-0000", store.ChangesetPublished, "dev-1")

	rec, out := e.do("GET", "/projects/p1/changesets/abcdef12-0000/review", "")
	if rec.Code != 200 || out["branch"] != "terraflow/demo/dev-abcdef12" {
		t.Fatalf("detail: %d %v", rec.Code, out)
	}

	rec, out = e.do("POST", "/projects/p1/changesets/abcdef12-0000/approve", "")
	if rec.Code != 200 || out["status"] != "approved" {
		t.Fatalf("approve: %d %v", rec.Code, out)
	}
	branch := out["branch"].(string)
	show := exec.Command("git", "show", branch+":storage/main.tf")
	show.Dir = e.dir
	b, err := show.CombinedOutput()
	if err != nil || !strings.Contains(string(b), `module "sa_demo"`) {
		t.Fatalf("branch content: %v %s", err, b)
	}
	cs := e.mem.data["abcdef12-0000"]
	if cs.Status != store.ChangesetApproved || cs.BranchName == nil || *cs.BranchName != branch || cs.ReviewerOID == nil {
		t.Fatalf("changeset not recorded as approved: %+v", cs)
	}

	// Second approve is a conflict, not a second branch.
	if rec, _ = e.do("POST", "/projects/p1/changesets/abcdef12-0000/approve", ""); rec.Code != http.StatusConflict {
		t.Fatalf("double approve: %d", rec.Code)
	}

	list, _ := e.rs.ListAudit(context.Background(), "p1", 10)
	if len(list) != 1 || list[0].Action != "changeset.approve" || list[0].UserOID != "reviewer-1" {
		t.Fatalf("audit: %+v", list)
	}
}

func TestRejectNeedsReasonAndRole(t *testing.T) {
	e := newReviewEnv(t, false)
	e.seed("c1", store.ChangesetPublished, "dev-1")

	if rec, _ := e.do("POST", "/projects/p1/changesets/c1/reject", `{"reason":"  "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank reason: %d", rec.Code)
	}
	e.role = "developer"
	if rec, _ := e.do("POST", "/projects/p1/changesets/c1/reject", `{"reason":"no"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("developer reject: %d", rec.Code)
	}
	if rec, _ := e.do("POST", "/projects/p1/changesets/c1/approve", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("developer approve: %d", rec.Code)
	}
	e.role = "devops"
	if rec, _ := e.do("POST", "/projects/p1/changesets/c1/reject", `{"reason":"wrong region"}`); rec.Code != 200 {
		t.Fatalf("reject: %d", rec.Code)
	}
	cs := e.mem.data["c1"]
	if cs.Status != store.ChangesetRejected || cs.RejectReason == nil || *cs.RejectReason != "wrong region" {
		t.Fatalf("not rejected: %+v", cs)
	}
	if _, err := os.Stat(filepath.Join(e.dir, ".git")); err == nil {
		t.Fatal("reject must not touch the repository")
	}
}

func TestSeparationOfDutiesAndDraftsAndConflicts(t *testing.T) {
	e := newReviewEnv(t, true)
	e.seed("own", store.ChangesetPublished, "reviewer-1")
	if rec, _ := e.do("POST", "/projects/p1/changesets/own/approve", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("self-approve: %d", rec.Code)
	}

	e.seed("draft", store.ChangesetDraft, "dev-1")
	if rec, _ := e.do("POST", "/projects/p1/changesets/draft/approve", ""); rec.Code != http.StatusConflict {
		t.Fatalf("approve draft: %d", rec.Code)
	}

	// A file that already exists is extended, never replaced: the review shows it
	// as "modified" with the hand-written content still in it.
	e.seed("clash", store.ChangesetPublished, "dev-1")
	os.MkdirAll(filepath.Join(e.dir, "storage"), 0o755)
	os.WriteFile(filepath.Join(e.dir, "storage", "main.tf"), []byte("# hand written\n"), 0o644)
	rec, out := e.do("GET", "/projects/p1/changesets/clash/review", "")
	diff, _ := out["hcl_diff"].([]any)
	if rec.Code != 200 || len(diff) != 1 {
		t.Fatalf("review: %d %v", rec.Code, out)
	}
	f := diff[0].(map[string]any)
	if f["status"] != "modified" || !strings.HasPrefix(f["content"].(string), "# hand written\n") {
		t.Fatalf("existing file must be kept and marked modified: %v", f)
	}

	// Wrong project id never resolves.
	if rec, _ := e.do("GET", "/projects/other/changesets/clash/review", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-project access: %d", rec.Code)
	}
}
