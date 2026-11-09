package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

const (
	sessionCookieName = "tf_session"
	// sessionDuration is also how long a changed Entra group membership can go
	// unnoticed: roles are read once at sign-in and not re-checked (D-010: no
	// Graph calls). Signing in again re-reads them.
	sessionDuration = 4 * time.Hour
)

type Handlers struct {
	oidc     *OIDCProvider
	queries  *store.Queries
	roles    RoleConfig
	logger   *slog.Logger
	insecure bool
}

func NewHandlers(oidc *OIDCProvider, q *store.Queries, roles RoleConfig, logger *slog.Logger, insecure bool) *Handlers {
	return &Handlers{oidc: oidc, queries: q, roles: roles, logger: logger, insecure: insecure}
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	nonce := randomHex(16)
	verifier := oauth2.GenerateVerifier()
	for name, value := range map[string]string{"tf_oidc_state": state, "tf_oidc_nonce": nonce, "tf_oidc_verifier": verifier} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: value,
			Path: "/", HttpOnly: true, Secure: !h.insecure, SameSite: http.SameSiteLaxMode, MaxAge: 300,
		})
	}
	http.Redirect(w, r, h.oidc.AuthCodeURL(state, nonce, verifier), http.StatusFound)
}

// nonceMatches reports whether the ID token carries the nonce this browser sent
// at login, so a token minted for another sign-in cannot be replayed here.
func nonceMatches(tokenNonce string, c *http.Cookie, err error) bool {
	return err == nil && c.Value != "" && subtle.ConstantTimeCompare([]byte(tokenNonce), []byte(c.Value)) == 1
}

func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("tf_oidc_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	verifier, err := r.Cookie("tf_oidc_verifier")
	if err != nil || verifier.Value == "" {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	idToken, err := h.oidc.Exchange(r.Context(), r.URL.Query().Get("code"), verifier.Value)
	if err != nil {
		h.logger.Error("oidc exchange", "err", err)
		http.Error(w, "auth failed", http.StatusInternalServerError)
		return
	}
	nonceCookie, nerr := r.Cookie("tf_oidc_nonce")
	if !nonceMatches(idToken.Nonce, nonceCookie, nerr) {
		h.logger.Warn("oidc nonce mismatch")
		http.Error(w, "invalid state", http.StatusBadRequest)
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
	for _, name := range []string{"tf_oidc_state", "tf_oidc_nonce", "tf_oidc_verifier"} {
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
