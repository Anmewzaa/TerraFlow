package auth

import (
	"crypto/subtle"
	"net/http"
	"strconv"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
)

const ghStateCookie = "tf_gh_login_state"

// GitHubLogin is sign-in with GitHub (D-050): OAuth2 through the same GitHub App as the
// repository connection. The identity is the numeric GitHub account id, never the
// login name, and the user token is used once to read it and then dropped, so
// signing in gives TerraFlow no access to any repository.
type GitHubLogin struct {
	App         *github.App
	RedirectURI string // the callback URL registered on the GitHub App
	Handlers    *Handlers
}

// Login: GET /auth/github/login.
func (g *GitHubLogin) Login(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name: ghStateCookie, Value: state, Path: "/auth/github", MaxAge: 300,
		HttpOnly: true, Secure: !g.Handlers.insecure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, g.App.AuthorizeURL(state, g.RedirectURI), http.StatusFound)
}

// Callback: GET /auth/github/callback.
func (g *GitHubLogin) Callback(w http.ResponseWriter, r *http.Request) {
	h := g.Handlers
	c, err := r.Cookie(ghStateCookie)
	http.SetCookie(w, &http.Cookie{Name: ghStateCookie, Value: "", Path: "/auth/github", MaxAge: -1, HttpOnly: true, Secure: !h.insecure, SameSite: http.SameSiteLaxMode})
	state := r.URL.Query().Get("state")
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		http.Redirect(w, r, "/?signin=state", http.StatusFound)
		return
	}
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/?signin=denied", http.StatusFound)
		return
	}
	tok, err := g.App.ExchangeCode(r.Context(), r.URL.Query().Get("code"), g.RedirectURI)
	if err != nil {
		h.logger.Warn("github sign-in: code exchange failed")
		http.Redirect(w, r, "/?signin=code", http.StatusFound)
		return
	}
	u, err := g.App.GetUser(r.Context(), tok)
	if err != nil || u.ID == 0 {
		h.logger.Warn("github sign-in: could not read the account")
		http.Redirect(w, r, "/?signin=github", http.StatusFound)
		return
	}
	// tok goes out of scope here; it is never stored, logged or returned.
	oid := "github:" + strconv.FormatInt(u.ID, 10)
	roles := []string{"developer"}
	if h.signup.IsAdmin(oid, "") {
		roles = []string{"developer", "devops"}
	}
	name := u.Login
	if !h.finishSignIn(w, r, oid, "", name, roles, nil) {
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
