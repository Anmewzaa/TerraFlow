package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// AccessStore is what the access and admin handlers need from persistence.
// *store.Queries implements it; tests use an in-memory fake.
type AccessStore interface {
	UpsertUser(ctx context.Context, u store.User) error
	ListUsers(ctx context.Context) ([]store.UserSummary, error)

	ListProjects(ctx context.Context) ([]store.Project, error)
	ListProjectsForUser(ctx context.Context, oid string) ([]store.Project, error)
	GetProject(ctx context.Context, id string) (store.Project, error)
	CreateProject(ctx context.Context, p store.Project) (store.Project, error)
	UpdateProject(ctx context.Context, id string, patch store.ProjectPatch) (store.Project, error)
	DeleteProject(ctx context.Context, id string) error
	CountPendingChangesets(ctx context.Context, projectID string) (int, error)

	IsMember(ctx context.Context, projectID, oid string) (bool, error)
	ListMembers(ctx context.Context, projectID string) ([]store.Member, error)
	SetMembers(ctx context.Context, projectID string, oids []string, by string) error

	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// RequireAdmin lets only the admin role (Entra "devops" group) through.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isDevOps(auth.UserFromContext(r.Context())) {
			writeJSONError(w, http.StatusForbidden, "this action needs the admin role")
			return
		}
		next.ServeHTTP(w, r)
	})
}

const userSyncEvery = 10 * time.Minute

// UserSync records the signed-in user so an admin can assign them to projects.
// It writes at most once per user per userSyncEvery, and never blocks a request
// when the write fails.
func UserSync(s interface {
	UpsertUser(ctx context.Context, u store.User) error
}, logger *slog.Logger) func(http.Handler) http.Handler {
	var seen sync.Map // oid -> time.Time
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u := auth.UserFromContext(r.Context()); u != nil && u.OID != "" {
				last, ok := seen.Load(u.OID)
				if !ok || time.Since(last.(time.Time)) > userSyncEvery {
					err := s.UpsertUser(r.Context(), store.User{OID: u.OID, Email: u.Email, Name: u.Name})
					if err != nil {
						if logger != nil {
							logger.Warn("record user", "err", err)
						}
					} else {
						seen.Store(u.OID, time.Now())
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ProjectGuard limits /projects/{id}/... to admins and the project's members.
// Anyone else gets 404, so a project's existence is not revealed.
func ProjectGuard(s interface {
	IsMember(ctx context.Context, projectID, oid string) (bool, error)
}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := auth.UserFromContext(r.Context())
			if u == nil {
				writeJSONError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			if isDevOps(u) {
				next.ServeHTTP(w, r)
				return
			}
			ok, err := s.IsMember(r.Context(), chi.URLParam(r, "id"), u.OID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "could not check project access")
				return
			}
			if !ok {
				writeJSONError(w, http.StatusNotFound, "project not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
