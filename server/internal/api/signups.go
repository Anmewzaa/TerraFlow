package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// SignupStore is what the sign-up approval screen needs (D-050).
type SignupStore interface {
	UserStatus(ctx context.Context, oid string) (string, error)
	SetUserStatus(ctx context.Context, oid, status, reason, by string) error
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// SignupAPI lets an admin approve, reject, suspend and reactivate people.
type SignupAPI struct {
	Store SignupStore
}

const maxStatusReason = 200

// allowed says which change is possible from each status, and what to call it in the
// audit log.
var allowed = map[string]map[string]string{
	store.UserPending:   {store.UserActive: "user.approve", store.UserRejected: "user.reject"},
	store.UserActive:    {store.UserSuspended: "user.suspend"},
	store.UserSuspended: {store.UserActive: "user.reactivate"},
	store.UserRejected:  {store.UserActive: "user.reactivate"},
}

// SetStatus: POST /admin/users/{oid}/status {status, reason}. Rejecting or suspending ends
// the person's sessions at once. An admin cannot change their own status.
func (s *SignupAPI) SetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		oid := chi.URLParam(r, "oid")
		var req struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		reason := strings.TrimSpace(req.Reason)
		if len(reason) > maxStatusReason {
			writeJSONError(w, http.StatusBadRequest, "the reason is too long (max 200 characters)")
			return
		}
		by := actor(r)
		if oid == by {
			writeJSONError(w, http.StatusConflict, "you cannot change your own status")
			return
		}
		cur, err := s.Store.UserStatus(r.Context(), oid)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if cur == "" {
			writeJSONError(w, http.StatusNotFound, "no such user")
			return
		}
		action, ok := allowed[cur][req.Status]
		if !ok {
			writeJSONError(w, http.StatusConflict, "a "+cur+" user cannot become "+req.Status)
			return
		}
		if err := s.Store.SetUserStatus(r.Context(), oid, req.Status, reason, by); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		_ = s.Store.LogAudit(r.Context(), by, action, "user:"+oid, map[string]any{"from": cur, "to": req.Status, "reason": reason})
		writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
	}
}
