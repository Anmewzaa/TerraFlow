package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
)

// Layer 2 of the connection (D-049): the user authorizes with GitHub and TerraFlow
// uses the resulting *user token* during that one request only, to learn which
// installations and repositories this user can really reach. A user token is never
// stored, logged or returned; the installation token (app.go) is what writes.

// maxPages bounds a paged listing (100 per page), so a huge account cannot make one
// request run forever.
const maxPages = 10

// User is the GitHub account behind a user token.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	// Name and Email are what the profile shows; either may be empty and neither is an
	// identifier (the numeric ID is).
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Repo is a repository a user may connect.
type Repo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// AuthorizeURL is where the browser is sent to let the user authorize the App. state
// is the anti-forgery value the caller checks on return.
func (a *App) AuthorizeURL(state, redirectURI string) string {
	return a.AuthorizeURLPKCE(state, redirectURI, "")
}

// AuthorizeURLPKCE is AuthorizeURL with a PKCE (S256) challenge, for sign-in: the code
// that comes back is useless without the verifier only this browser session holds.
func (a *App) AuthorizeURLPKCE(state, redirectURI, challenge string) string {
	q := url.Values{}
	q.Set("client_id", a.ClientID)
	q.Set("state", state)
	if redirectURI != "" {
		q.Set("redirect_uri", redirectURI)
	}
	if challenge != "" {
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
	}
	return a.web() + "/login/oauth/authorize?" + q.Encode()
}

// ExchangeCode trades the one-time code from the authorization redirect for a user
// token. The App's client secret is sent once and never returned or logged.
func (a *App) ExchangeCode(ctx context.Context, code, redirectURI string) (Token, error) {
	return a.ExchangeCodeVerifier(ctx, code, redirectURI, "")
}

// ExchangeCodeVerifier is ExchangeCode for a flow that started with a PKCE challenge.
func (a *App) ExchangeCodeVerifier(ctx context.Context, code, redirectURI, verifier string) (Token, error) {
	if a.ClientID == "" || a.ClientSecret == "" {
		return Token{}, errors.New("the GitHub App has no client id and secret for user authorization")
	}
	if code == "" {
		return Token{}, errors.New("missing authorization code")
	}
	form := url.Values{}
	form.Set("client_id", a.ClientID)
	form.Set("client_secret", a.ClientSecret)
	form.Set("code", code)
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}
	if verifier != "" {
		form.Set("code_verifier", verifier)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.web()+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "TerraFlow")
	resp, err := a.client().Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("GitHub is not reachable: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return Token{}, fmt.Errorf("reading the GitHub answer: %w", err)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(raw, &out) != nil {
		return Token{}, errors.New("GitHub refused the authorization code")
	}
	// GitHub answers 200 with an error field for a bad or reused code.
	if out.Error != "" || out.AccessToken == "" {
		return Token{}, errors.New("GitHub refused the authorization code")
	}
	return Token{value: out.AccessToken}, nil
}

// GetUser reads the account a user token belongs to.
func (a *App) GetUser(ctx context.Context, t Token) (User, error) {
	var u User
	if err := a.call(ctx, t.value, http.MethodGet, "/user", nil, &u); err != nil {
		return User{}, err
	}
	if u.ID == 0 || u.Login == "" {
		return User{}, errors.New("GitHub returned no account for this token")
	}
	return u, nil
}

// UserInstallations lists the installations of this App that the user can access.
func (a *App) UserInstallations(ctx context.Context, t Token) ([]Installation, error) {
	var all []Installation
	for page := 1; page <= maxPages; page++ {
		var out struct {
			Installations []Installation `json:"installations"`
		}
		p := "/user/installations?per_page=100&page=" + strconv.Itoa(page)
		if err := a.call(ctx, t.value, http.MethodGet, p, nil, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Installations...)
		if len(out.Installations) < 100 {
			break
		}
	}
	return all, nil
}

// UserInstallationRepos lists the repositories of one installation that this user can
// access: exactly the intersection the picker needs, so a user cannot connect a
// repository they cannot reach themselves.
func (a *App) UserInstallationRepos(ctx context.Context, t Token, installationID int64) ([]Repo, error) {
	var all []Repo
	for page := 1; page <= maxPages; page++ {
		var out struct {
			Repositories []Repo `json:"repositories"`
		}
		p := "/user/installations/" + strconv.FormatInt(installationID, 10) + "/repositories?per_page=100&page=" + strconv.Itoa(page)
		if err := a.call(ctx, t.value, http.MethodGet, p, nil, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Repositories...)
		if len(out.Repositories) < 100 {
			break
		}
	}
	return all, nil
}

// IsOrgMember reports whether GitHub confirms that login is a member of org, asked
// with that person's own token. GitHub only tells a member about private membership
// (and only when the App may read the organisation's members); otherwise the public
// membership decides. So true is always a real membership, and false means "not
// confirmed", never "confirmed absent".
func (a *App) IsOrgMember(ctx context.Context, t Token, org, login string) (bool, error) {
	base := "/orgs/" + url.PathEscape(org)
	var last error
	for _, p := range []string{base + "/members/" + url.PathEscape(login), base + "/public_members/" + url.PathEscape(login)} {
		err := a.call(ctx, t.value, http.MethodGet, p, nil, nil)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, gitprovider.ErrNotFound) && !errors.Is(err, gitprovider.ErrUnauthorized) {
			last = err
		}
	}
	return false, last
}

// VerifiedEmails lists the e-mail addresses GitHub has verified for the account behind
// a user token (lower case). It needs the App's account permission "Email addresses"
// (read); without it GitHub refuses and the error says so.
func (a *App) VerifiedEmails(ctx context.Context, t Token) ([]string, error) {
	var list []struct {
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
	}
	if err := a.call(ctx, t.value, http.MethodGet, "/user/emails?per_page=100", nil, &list); err != nil {
		return nil, err
	}
	var out []string
	for _, e := range list {
		if e.Verified && e.Email != "" {
			out = append(out, strings.ToLower(e.Email))
		}
	}
	return out, nil
}
