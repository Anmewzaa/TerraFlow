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

// userStatusStore is the part of the store that approves, rejects and suspends users.
type userStatusStore interface {
	GetUserStatus(ctx context.Context, oid string) (store.UserStatus, error)
	SetUserStatus(ctx context.Context, oid, status, reason, by string) error
}

const maxReasonLen = 500

// SetUserStatus: POST /admin/users/{oid}/status {"status": "...", "reason": "..."}.
// An admin approves a pending user (active), rejects one (rejected, with a reason),
// suspends an active user (suspended, with a reason; their sessions end at once) or
// restores a rejected or suspended one (active). Every change is audited.
func (a *AdminAPI) SetUserStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, ok := a.Store.(userStatusStore)
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "user approval needs the database")
			return
		}
		admin := auth.UserFromContext(r.Context())
		oid := chi.URLParam(r, "oid")
		var req struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		if len(req.Reason) > maxReasonLen {
			writeJSONError(w, http.StatusBadRequest, "the reason is too long")
			return
		}
		if admin != nil && admin.OID == oid {
			writeJSONError(w, http.StatusUnprocessableEntity, "you cannot change your own account")
			return
		}
		action := map[string]string{store.StatusActive: "user.approve", store.StatusRejected: "user.reject", store.StatusSuspended: "user.suspend"}[req.Status]
		if action == "" {
			writeJSONError(w, http.StatusBadRequest, "status must be active, rejected or suspended")
			return
		}
		if (req.Status == store.StatusRejected || req.Status == store.StatusSuspended) && req.Reason == "" {
			writeJSONError(w, http.StatusUnprocessableEntity, "a reason is required")
			return
		}
		cur, err := st.GetUserStatus(r.Context(), oid)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if req.Status == store.StatusSuspended && cur.Status != store.StatusActive {
			writeJSONError(w, http.StatusUnprocessableEntity, "only an active user can be suspended")
			return
		}
		if req.Status == store.StatusRejected && cur.Status != store.StatusPending {
			writeJSONError(w, http.StatusUnprocessableEntity, "only a pending user can be rejected; suspend an active one")
			return
		}
		by := ""
		if admin != nil {
			by = admin.OID
		}
		if err := st.SetUserStatus(r.Context(), oid, req.Status, req.Reason, by); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if cur.Status != store.StatusActive && req.Status == store.StatusActive {
			action = "user.approve"
			if cur.Status != store.StatusPending {
				action = "user.restore"
			}
		}
		_ = a.Store.LogAudit(r.Context(), by, action, "user:"+oid, map[string]any{"from": cur.Status, "to": req.Status, "reason": req.Reason})
		writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
	}
}

// ProviderInfo is one way to sign in, as the login page needs it.
type ProviderInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// AuthProviders: GET /api/auth/providers (no sign-in needed). The login page draws a
// button for each.
func AuthProviders(list []ProviderInfo) http.HandlerFunc {
	if list == nil {
		list = []ProviderInfo{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"providers": list})
	}
}
