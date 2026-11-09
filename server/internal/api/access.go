package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

// isBadReference reports a database error caused by the caller's input: an id
// that is not a UUID (22P02) or that points at a row that does not exist (23503).
func isBadReference(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && (pg.Code == "22P02" || pg.Code == "23503" || pg.Code == "22021")
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ChangesetInProject answers 404 unless the changeset in the URL belongs to the
// project in the URL, and, while it is still a draft or saved version, to the
// signed-in user. ProjectGuard only proves membership of {id}; without this a
// member of one project could read or change another project's changeset by id.
func ChangesetInProject(get func(ctx context.Context, id string) (store.Changeset, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csID := chi.URLParam(r, "csId")
			if !uuidPattern.MatchString(csID) {
				writeJSONError(w, http.StatusNotFound, "changeset not found")
				return
			}
			cs, err := get(r.Context(), csID)
			if err != nil || cs.ProjectID != chi.URLParam(r, "id") {
				writeJSONError(w, http.StatusNotFound, "changeset not found")
				return
			}
			// A draft or saved version belongs to its author (D-044), like the list.
			if cs.Status == store.ChangesetDraft || cs.Status == store.ChangesetSaved {
				if u := auth.UserFromContext(r.Context()); u == nil || u.OID != cs.UserOID {
					writeJSONError(w, http.StatusNotFound, "changeset not found")
					return
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
			if isBadReference(err) {
				// An id that is not even a UUID: same answer as a project that is not there.
				writeJSONError(w, http.StatusNotFound, "project not found")
				return
			}
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
