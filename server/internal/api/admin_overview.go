package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// OverviewStore is what the admin project overview reads.
type OverviewStore interface {
	ListProjects(ctx context.Context) ([]store.Project, error)
	GetProjectRepo(ctx context.Context, projectID string) (store.ProjectRepo, error)
	ListMembersWithRoles(ctx context.Context, projectID string) ([]store.RoleMember, error)
	CountPendingChangesets(ctx context.Context, projectID string) (int, error)
	ListConnections(ctx context.Context) ([]store.Connection, error)
}

// AdminOverviewAPI gives an admin the metadata of every project in one call (D-050: an
// admin sees names, owners, repositories and members; opening a project's content is a
// separate, audited step).
type AdminOverviewAPI struct {
	Store OverviewStore
}

type projectOverview struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	IACPath    string `json:"iac_path"`
	CreatedAt  string `json:"created_at"`
	// Owner is the member with the owner role (empty for a project without one).
	Owner   *overviewPerson `json:"owner,omitempty"`
	Members int             `json:"members"`
	Pending int             `json:"pending_changesets"`
	// Repository is set for a GitHub project.
	Repository *overviewRepo `json:"repository,omitempty"`
}

type overviewPerson struct {
	OID   string `json:"oid"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type overviewRepo struct {
	FullName       string `json:"full_name"`
	Folder         string `json:"folder"`
	Branch         string `json:"default_branch"`
	SecondReviewer bool   `json:"second_reviewer"`
	// Connection is the GitHub account the project goes through and its state.
	Account          string `json:"account,omitempty"`
	ConnectionStatus string `json:"connection_status,omitempty"`
	// AccessStatus is "ok" or "needs_attention": whether GitHub still lets the owner
	// reach the repository (D-061).
	AccessStatus string `json:"access_status,omitempty"`
}

// Projects: GET /admin/projects/overview.
func (a *AdminOverviewAPI) Projects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projects, err := a.Store.ListProjects(ctx)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		conns, err := a.Store.ListConnections(ctx)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		byConn := map[string]store.Connection{}
		for _, c := range conns {
			byConn[c.ID] = c
		}
		out := make([]projectOverview, 0, len(projects))
		for _, p := range projects {
			v := projectOverview{ID: p.ID, Name: p.Name, SourceType: p.SourceType, IACPath: p.IACPath, CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")}
			members, err := a.Store.ListMembersWithRoles(ctx, p.ID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			v.Members = len(members)
			for _, m := range members {
				if m.Role == store.RoleOwner {
					v.Owner = &overviewPerson{OID: m.UserOID, Name: m.Name, Email: m.Email}
				}
			}
			v.Pending, _ = a.Store.CountPendingChangesets(ctx, p.ID)
			repo, err := a.Store.GetProjectRepo(ctx, p.ID)
			switch {
			case err == nil:
				c := byConn[repo.ConnectionID]
				v.Repository = &overviewRepo{
					FullName: repo.RepoFullName, Folder: repo.RepoPath, Branch: repo.Branch(),
					SecondReviewer: repo.SecondReviewer, Account: c.AccountLogin, ConnectionStatus: c.Status,
					AccessStatus: repo.AccessStatus,
				}
			case errors.Is(err, pgx.ErrNoRows):
				// A local project has no repository link.
			default:
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			out = append(out, v)
		}
		writeJSON(w, http.StatusOK, out)
	}
}
