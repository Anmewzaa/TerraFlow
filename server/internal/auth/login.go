package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// Login finishes a sign-in for any provider: it applies the sign-up policy, records the
// person, and starts a session only for someone who is allowed in. Providers prove who
// the person is; this decides what they may do.
type Login struct {
	Queries  *store.Queries
	Policy   Policy
	Logger   *slog.Logger
	Insecure bool
	// WebURL is where the screens are served when not by this server (the Vite dev
	// server); empty means the same origin.
	WebURL string
}

// back sends the browser to the screens, with ?signin=<code> when it was refused.
func (l *Login) back(w http.ResponseWriter, r *http.Request, code string) {
	target := strings.TrimRight(l.WebURL, "/") + "/"
	if code != "" {
		target += "?" + url.Values{"signin": {code}}.Encode()
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// Finish is called with an identity a provider has already proven. roles are what the
// provider granted (Entra groups); empty means an ordinary developer. autoApproved
// skips the waiting list for people the provider vouches for.
func (l *Login) Finish(w http.ResponseWriter, r *http.Request, id store.LoginUser, roles, groups []string, autoApproved bool) {
	if id.OID == "" {
		l.Logger.Error("sign-in without an identity", "provider", id.Provider)
		l.back(w, r, "error")
		return
	}
	admin := l.Policy.IsAdmin(id.OID, id.Email)
	if admin && !hasRole(roles, "devops") {
		roles = append(roles, "devops")
	}
	if len(roles) == 0 {
		roles = []string{"developer"}
	}
	if groups == nil {
		groups = []string{}
	}

	status, created, err := l.Queries.RecordLogin(r.Context(), id, l.Policy.Initial(autoApproved), l.Policy.MaxPending, admin)
	switch {
	case errors.Is(err, store.ErrPendingFull):
		l.Logger.Warn("sign-up refused: the approval list is full", "provider", id.Provider)
		l.back(w, r, "full")
		return
	case errors.Is(err, store.ErrNotInvited):
		l.Logger.Info("sign-up refused: invitation only", "user", id.OID)
		l.back(w, r, "not_invited")
		return
	case err != nil:
		l.Logger.Error("record sign-in", "err", err)
		l.back(w, r, "error")
		return
	}
	// Someone who was waiting and is now vouched for (a rule, an Entra group) need not
	// wait for an administrator any longer. Rejected and suspended people stay as they are.
	if !created && status == store.UserPending && autoApproved {
		if err := l.Queries.SetUserStatus(r.Context(), id.OID, store.UserActive, "approved by a sign-in rule", "settings"); err != nil {
			l.Logger.Error("auto-approve a waiting sign-up", "err", err)
		} else {
			status = store.UserActive
			_ = l.Queries.LogAudit(r.Context(), id.OID, "user.auto_approved", "user:"+id.OID, map[string]any{"provider": id.Provider})
		}
	}
	if created {
		_ = l.Queries.LogAudit(r.Context(), id.OID, "user.signup", "user:"+id.OID,
			map[string]any{"provider": id.Provider, "status": status, "login": id.Login})
		l.Logger.Info("new sign-up", "user", id.OID, "status", status)
	}
	if status == store.UserRejected || status == store.UserSuspended {
		l.back(w, r, status)
		return
	}

	sid := randomHex(32)
	if err := l.Queries.CreateSession(r.Context(), store.Session{
		ID: sid, UserOID: id.OID, Email: id.Email, Name: id.Name, Roles: roles, GroupIDs: groups,
		ExpiresAt: time.Now().Add(sessionDuration),
	}); err != nil {
		l.Logger.Error("create session", "err", err)
		l.back(w, r, "error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: sid,
		Path: "/", HttpOnly: true, Secure: !l.Insecure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionDuration.Seconds()),
	})
	l.back(w, r, "")
}

func hasRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}
