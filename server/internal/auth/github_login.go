package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

const (
	ghLoginStateCookie    = "tf_ghlogin_state"
	ghLoginVerifierCookie = "tf_ghlogin_verifier"
	ghLoginCookiePath     = "/auth/github"
)

// GitHubLogin is "Sign in with GitHub" through the same GitHub App that opens branches
// (D-050). The browser proves who it is with state and PKCE; TerraFlow reads the
// account (GET /user), uses its numeric id as the identity, and discards the token.
type GitHubLogin struct {
	App *github.App
	// RedirectURI is the callback URL registered on the App for sign-in
	// (.../auth/github/callback); it differs from the connect callback.
	RedirectURI string
	Login       *Login
	// Verified, when set, is called with the proven identity and the token before the
	// token is dropped: the server uses it to re-check the repositories of the projects
	// this person owns (D-061). It must not keep the token.
	Verified func(ctx context.Context, oid string, tok github.Token)
}

func (g *GitHubLogin) cookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: ghLoginCookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: !g.Login.Insecure, SameSite: http.SameSiteLaxMode}
}

// Start: GET /auth/github/login sends the browser to GitHub.
func (g *GitHubLogin) Start(w http.ResponseWriter, r *http.Request) {
	state, verifier := randomHex(16), oauth2.GenerateVerifier()
	http.SetCookie(w, g.cookie(ghLoginStateCookie, state, 600))
	http.SetCookie(w, g.cookie(ghLoginVerifierCookie, verifier, 600))
	challenge := oauth2.S256ChallengeFromVerifier(verifier)
	http.Redirect(w, r, g.App.AuthorizeURLPKCE(state, g.RedirectURI, challenge), http.StatusFound)
}

// Callback: GET /auth/github/callback checks the state, trades the code for a token
// (with the PKCE verifier), learns the account and hands the identity to Login.
func (g *GitHubLogin) Callback(w http.ResponseWriter, r *http.Request) {
	sc, serr := r.Cookie(ghLoginStateCookie)
	vc, verr := r.Cookie(ghLoginVerifierCookie)
	// The one-time cookies are spent whatever happens next.
	http.SetCookie(w, g.cookie(ghLoginStateCookie, "", -1))
	http.SetCookie(w, g.cookie(ghLoginVerifierCookie, "", -1))
	state := r.URL.Query().Get("state")
	if serr != nil || verr != nil || state == "" || vc.Value == "" ||
		subtle.ConstantTimeCompare([]byte(sc.Value), []byte(state)) != 1 {
		g.Login.back(w, r, "state")
		return
	}
	if r.URL.Query().Get("error") != "" {
		g.Login.back(w, r, "denied")
		return
	}
	tok, err := g.App.ExchangeCodeVerifier(r.Context(), r.URL.Query().Get("code"), g.RedirectURI, vc.Value)
	if err != nil {
		g.Login.Logger.Warn("github sign-in: code refused", "err", err.Error())
		g.Login.back(w, r, "code")
		return
	}
	u, err := g.App.GetUser(r.Context(), tok)
	if err != nil {
		g.Login.Logger.Warn("github sign-in: cannot read the account", "err", err.Error())
		g.Login.back(w, r, "github")
		return
	}
	name := strings.TrimSpace(u.Name)
	if name == "" {
		name = u.Login
	}
	oid := "github:" + strconv.FormatInt(u.ID, 10)
	if g.Verified != nil {
		g.Verified(r.Context(), oid, tok)
	}
	approved := false
	// The GitHub rules of TF_AUTO_APPROVE (D-062) are asked with the token before it is dropped.
	if rules := g.Login.Policy.AutoApprove; rules.NeedsGitHub() {
		var rule string
		if approved, rule = rules.GitHubVouches(r.Context(), g.App, u, tok, g.Login.Logger.Warn); approved {
			g.Login.Logger.Info("github sign-in: approved by a rule", "user", oid, "rule", rule)
		}
	}
	g.Login.Finish(w, r, store.LoginUser{
		OID: oid, Provider: "github",
		Login: u.Login, Email: strings.ToLower(strings.TrimSpace(u.Email)), Name: name,
	}, nil, nil, approved)
}

// ProviderInfo is one sign-in button.
type ProviderInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Href  string `json:"href"`
}

// Providers: GET /auth/providers tells the login page which buttons to show.
func Providers(list []ProviderInfo) http.HandlerFunc {
	if list == nil {
		list = []ProviderInfo{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"providers": list})
	}
}
