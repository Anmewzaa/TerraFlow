package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type memOverview struct{}

func (memOverview) ListProjects(context.Context) ([]store.Project, error) {
	t := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	return []store.Project{
		{ID: "p1", Name: "Platform", SourceType: "github", IACPath: "iac", CreatedAt: t},
		{ID: "p2", Name: "Local", SourceType: "local", IACPath: "./samples", CreatedAt: t},
	}, nil
}
func (memOverview) GetProjectRepo(_ context.Context, id string) (store.ProjectRepo, error) {
	if id != "p1" {
		return store.ProjectRepo{}, pgx.ErrNoRows
	}
	return store.ProjectRepo{ProjectID: id, ConnectionID: "c1", RepoFullName: "alice/infra", RepoPath: "iac", DefaultBranch: "main", SecondReviewer: true}, nil
}
func (memOverview) ListMembersWithRoles(_ context.Context, id string) ([]store.RoleMember, error) {
	if id == "p1" {
		return []store.RoleMember{{UserOID: "u1", Name: "Alice", Email: "a@x.test", Role: store.RoleOwner}, {UserOID: "u2", Role: store.RoleContributor}}, nil
	}
	return nil, nil
}
func (memOverview) CountPendingChangesets(_ context.Context, id string) (int, error) {
	if id == "p1" {
		return 3, nil
	}
	return 0, nil
}
func (memOverview) ListConnections(context.Context) ([]store.Connection, error) {
	return []store.Connection{{ID: "c1", AccountLogin: "alice", Status: store.ConnNeedsAttention}}, nil
}

func TestAdminOverviewShowsOwnerRepositoryConnectionAndPendingWorkWithoutContent(t *testing.T) {
	rec := httptest.NewRecorder()
	(&AdminOverviewAPI{Store: memOverview{}}).Projects()(rec, httptest.NewRequest("GET", "/api/admin/projects/overview", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out []projectOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out) != 2 {
		t.Fatalf("%v %s", err, rec.Body)
	}
	gh, local := out[0], out[1]
	if gh.Owner == nil || gh.Owner.Name != "Alice" || gh.Members != 2 || gh.Pending != 3 {
		t.Fatalf("github project: %+v", gh)
	}
	if gh.Repository == nil || gh.Repository.FullName != "alice/infra" || gh.Repository.Account != "alice" ||
		gh.Repository.ConnectionStatus != store.ConnNeedsAttention || !gh.Repository.SecondReviewer {
		t.Fatalf("repository: %+v", gh.Repository)
	}
	if local.Repository != nil || local.Owner != nil || local.Members != 0 {
		t.Fatalf("local project: %+v", local)
	}
}
