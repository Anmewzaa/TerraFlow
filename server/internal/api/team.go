package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// TeamStore is what the project team endpoints need. *store.Queries implements it.
type TeamStore interface {
	GetProject(ctx context.Context, id string) (store.Project, error)
	DeleteProject(ctx context.Context, id string) error
	MemberRole(ctx context.Context, projectID, oid string) (string, error)
	ListMembersWithRoles(ctx context.Context, projectID string) ([]store.RoleMember, error)
	AddMemberRole(ctx context.Context, projectID, oid, role, by string) error
	RemoveMember(ctx context.Context, projectID, oid string) error
	TransferOwnership(ctx context.Context, projectID, from, to string) error
	GetProjectRepo(ctx context.Context, projectID string) (store.ProjectRepo, error)
	SetSecondReviewer(ctx context.Context, projectID string, on bool) error
	ListUsers(ctx context.Context) ([]store.UserSummary, error)
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// TeamAPI is the project's own people management (D-050): the owner and co-owners
// invite and set roles; only the owner transfers or deletes.
type TeamAPI struct {
	Store TeamStore
}

// callerRole is the caller's effective role in the project. The global admin acts as
// an owner for management (the access itself is audited by ProjectGuard).
func (t *TeamAPI) callerRole(r *http.Request, projectID string) (string, error) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return "", nil
	}
	role, err := t.Store.MemberRole(r.Context(), projectID, u.OID)
	if err != nil {
		return "", err
	}
	if role == "" && isDevOps(u) {
		return store.RoleOwner, nil
	}
	return role, nil
}

// need answers 403 unless the caller's role ranks at least min.
func (t *TeamAPI) need(w http.ResponseWriter, r *http.Request, min string) (role string, ok bool) {
	id := chi.URLParam(r, "id")
	role, err := t.callerRole(r, id)
	if isBadReference(err) {
		writeJSONError(w, http.StatusNotFound, "project not found")
		return "", false
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "db error")
		return "", false
	}
	if store.RoleRank(role) < store.RoleRank(min) || role == "" {
		writeJSONError(w, http.StatusForbidden, "this needs the "+strings.ReplaceAll(min, "_", "-")+" role of the project")
		return "", false
	}
	return role, true
}

// Require lets a request through only when the caller's role in the project of the URL ranks
// at least min (403 otherwise). It goes behind ProjectGuard.
func (t *TeamAPI) Require(min string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := t.need(w, r, min); ok {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func (t *TeamAPI) audit(r *http.Request, action, projectID string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	detail["project_id"] = projectID // the project's audit trail finds entries by this
	_ = t.Store.LogAudit(r.Context(), actor(r), action, "project:"+projectID, detail)
}

type teamView struct {
	Members        []store.RoleMember `json:"members"`
	MyRole         string             `json:"my_role"`
	SecondReviewer bool               `json:"second_reviewer"`
	// Repository is set for a project that lives on GitHub.
	Repository string `json:"repository,omitempty"`
	Folder     string `json:"folder,omitempty"`
	// RepositoryAccess is "ok" or "needs_attention" (the owner lost access on GitHub, D-061).
	RepositoryAccess string `json:"repository_access,omitempty"`
}

// Get: GET /projects/{id}/team lists the members and the caller's role.
func (t *TeamAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		role, err := t.callerRole(r, id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		members, err := t.Store.ListMembersWithRoles(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if members == nil {
			members = []store.RoleMember{}
		}
		v := teamView{Members: members, MyRole: role}
		if pr, err := t.Store.GetProjectRepo(r.Context(), id); err == nil {
			v.SecondReviewer, v.Repository, v.Folder = pr.SecondReviewer, pr.RepoFullName, pr.RepoPath
			v.RepositoryAccess = pr.AccessStatus
		}
		writeJSON(w, http.StatusOK, v)
	}
}

type inviteBody struct {
	// User is the person's email or id; they must have signed in once.
	User string `json:"user"`
	Role string `json:"role"`
}

// findUser resolves an email (case-insensitive) or an id to a user who has signed in.
func (t *TeamAPI) findUser(ctx context.Context, ref string) (store.UserSummary, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return store.UserSummary{}, false, nil
	}
	users, err := t.Store.ListUsers(ctx)
	if err != nil {
		return store.UserSummary{}, false, err
	}
	for _, u := range users {
		if u.OID == ref || strings.EqualFold(u.Email, ref) {
			return u, true, nil
		}
	}
	return store.UserSummary{}, false, nil
}

// Invite: POST /projects/{id}/team adds a user who has signed in, or changes their role.
// Owner and co-owner only; the owner role is never given here.
func (t *TeamAPI) Invite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := t.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		var req inviteBody
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Role == "" {
			req.Role = store.RoleContributor
		}
		if !store.ValidRole(req.Role) || req.Role == store.RoleOwner {
			writeJSONError(w, http.StatusBadRequest, "role must be co_owner or contributor")
			return
		}
		u, found, err := t.findUser(r.Context(), req.User)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if !found {
			writeJSONError(w, http.StatusUnprocessableEntity, "that person has not signed in to TerraFlow yet; they must sign in once before they can be invited")
			return
		}
		if u.Status != "" && u.Status != store.UserActive {
			writeJSONError(w, http.StatusUnprocessableEntity, "that person has not been approved yet; an admin has to approve them first")
			return
		}
		t.apply(w, r, id, u.OID, req.Role, "project.member.add")
	}
}

// SetRole: PATCH /projects/{id}/team/{oid} changes a member's role.
func (t *TeamAPI) SetRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := t.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		var req struct {
			Role string `json:"role"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if !store.ValidRole(req.Role) || req.Role == store.RoleOwner {
			writeJSONError(w, http.StatusBadRequest, "role must be co_owner or contributor")
			return
		}
		id, oid := chi.URLParam(r, "id"), chi.URLParam(r, "oid")
		if cur, err := t.Store.MemberRole(r.Context(), id, oid); err != nil || cur == "" {
			writeJSONError(w, http.StatusNotFound, "that person is not a member of this project")
			return
		}
		t.apply(w, r, id, oid, req.Role, "project.member.role")
	}
}

func (t *TeamAPI) apply(w http.ResponseWriter, r *http.Request, id, oid, role, action string) {
	err := t.Store.AddMemberRole(r.Context(), id, oid, role, actor(r))
	switch {
	case errors.Is(err, store.ErrOwnerChange):
		writeJSONError(w, http.StatusConflict, "the owner's role cannot be changed; transfer ownership instead")
		return
	case isBadReference(err):
		writeJSONError(w, http.StatusNotFound, "project not found")
		return
	case err != nil:
		writeJSONError(w, http.StatusInternalServerError, "db error")
		return
	}
	t.audit(r, action, id, map[string]any{"user_oid": oid, "role": role})
	t.Get()(w, r)
}

// Remove: DELETE /projects/{id}/team/{oid}. The owner cannot be removed.
func (t *TeamAPI) Remove() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := t.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id, oid := chi.URLParam(r, "id"), chi.URLParam(r, "oid")
		err := t.Store.RemoveMember(r.Context(), id, oid)
		switch {
		case errors.Is(err, store.ErrOwnerChange):
			writeJSONError(w, http.StatusConflict, "the owner cannot be removed; transfer ownership first")
			return
		case errors.Is(err, store.ErrNotMember):
			writeJSONError(w, http.StatusNotFound, "that person is not a member of this project")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		t.audit(r, "project.member.remove", id, map[string]any{"user_oid": oid})
		w.WriteHeader(http.StatusNoContent)
	}
}

// Settings: PUT /projects/{id}/settings sets the per-project second-reviewer switch.
func (t *TeamAPI) Settings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := t.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		var req struct {
			SecondReviewer bool `json:"second_reviewer"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		id := chi.URLParam(r, "id")
		err := t.Store.SetSecondReviewer(r.Context(), id, req.SecondReviewer)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusConflict, "this project has no GitHub repository to set this for")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		t.audit(r, "project.settings", id, map[string]any{"second_reviewer": req.SecondReviewer})
		t.Get()(w, r)
	}
}

// Transfer: POST /projects/{id}/transfer {"to": oid} (owner only). The old owner
// becomes a co-owner; the new owner must already be a member.
func (t *TeamAPI) Transfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := t.need(w, r, store.RoleOwner); !ok {
			return
		}
		var req struct {
			To string `json:"to"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		id := chi.URLParam(r, "id")
		from, err := t.Store.ListMembersWithRoles(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		owner := ""
		for _, m := range from {
			if m.Role == store.RoleOwner {
				owner = m.UserOID
			}
		}
		err = t.Store.TransferOwnership(r.Context(), id, owner, strings.TrimSpace(req.To))
		switch {
		case errors.Is(err, store.ErrNotMember):
			writeJSONError(w, http.StatusUnprocessableEntity, "the new owner must already be a member of the project")
			return
		case err != nil:
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		t.audit(r, "project.transfer", id, map[string]any{"from": owner, "to": req.To})
		t.Get()(w, r)
	}
}

// OwnerDelete is DELETE /projects/{id} for a project's owner (a GitHub project; the
// repository itself is never touched). Admins use AdminAPI.Delete.
func (t *TeamAPI) OwnerDelete(admin http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isDevOps(auth.UserFromContext(r.Context())) {
			admin(w, r)
			return
		}
		if _, ok := t.need(w, r, store.RoleOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		p, err := t.Store.GetProject(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		if p.SourceType != "github" {
			writeJSONError(w, http.StatusForbidden, "only an admin can delete a local project")
			return
		}
		if err := t.Store.DeleteProject(r.Context(), id); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		t.audit(r, "project.delete", id, nil)
		w.WriteHeader(http.StatusNoContent)
	}
}
