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
	q := url.Values{}
	q.Set("client_id", a.ClientID)
	q.Set("state", state)
	if redirectURI != "" {
		q.Set("redirect_uri", redirectURI)
	}
	return a.web() + "/login/oauth/authorize?" + q.Encode()
}

// ExchangeCode trades the one-time code from the authorization redirect for a user
// token. The App's client secret is sent once and never returned or logged.
func (a *App) ExchangeCode(ctx context.Context, code, redirectURI string) (Token, error) {
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
