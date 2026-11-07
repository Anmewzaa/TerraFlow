package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

const (
	sessionCookieName = "tf_session"
	sessionDuration   = 8 * time.Hour
)

type Handlers struct {
	oidc       *OIDCProvider
	queries    *store.Queries
	roles      RoleConfig
	logger     *slog.Logger
	insecure   bool
}

func NewHandlers(oidc *OIDCProvider, q *store.Queries, roles RoleConfig, logger *slog.Logger, insecure bool) *Handlers {
	return &Handlers{oidc: oidc, queries: q, roles: roles, logger: logger, insecure: insecure}
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	nonce := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name: "tf_oidc_state", Value: state,
		Path: "/", HttpOnly: true, Secure: !h.insecure, SameSite: http.SameSiteLaxMode, MaxAge: 300,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "tf_oidc_nonce", Value: nonce,
		Path: "/", HttpOnly: true, Secure: !h.insecure, SameSite: http.SameSiteLaxMode, MaxAge: 300,
	})
	http.Redirect(w, r, h.oidc.AuthCodeURL(state, nonce), http.StatusFound)
}

func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("tf_oidc_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	idToken, err := h.oidc.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		h.logger.Error("oidc exchange", "err", err)
		http.Error(w, "auth failed", http.StatusInternalServerError)
		return
	}

	var claims struct {
		OID    string   `json:"oid"`
		Email  string   `json:"preferred_username"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		h.logger.Error("parse claims", "err", err)
		http.Error(w, "auth failed", http.StatusInternalServerError)
		return
	}

	groups := claims.Groups
	if groups == nil {
		groups = []string{}
	}

	roles := MapRoles(h.roles, groups)
	if len(roles) == 0 {
		http.Error(w, "no authorized role", http.StatusForbidden)
		return
	}

	sid := randomHex(32)
	sess := store.Session{
		ID:        sid,
		UserOID:   claims.OID,
		Email:     claims.Email,
		Name:      claims.Name,
		Roles:     roles,
		GroupIDs:  groups,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if err := h.queries.CreateSession(r.Context(), sess); err != nil {
		h.logger.Error("create session", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: sid,
		Path: "/", HttpOnly: true, Secure: !h.insecure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionDuration.Seconds()),
	})

	// Clear OIDC cookies
	for _, name := range []string{"tf_oidc_state", "tf_oidc_nonce"} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1})
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		_ = h.queries.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Path: "/", MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
