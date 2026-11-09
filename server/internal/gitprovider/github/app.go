// Package github is the GitHub implementation of gitprovider.Source, built on a
// GitHub App (D-049, D-050). The App's private key signs a short JWT; the JWT is
// exchanged for an installation token that lasts one hour and is limited to one
// repository. Tokens are made per operation and never stored, logged or returned.
package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
)

// DefaultAPIBase is the GitHub REST API.
const DefaultAPIBase = "https://api.github.com"

// maxBody caps what is read from any response, so a misbehaving server cannot
// exhaust memory.
const maxBody = 16 << 20

// App holds the credentials of the GitHub App. There is one per TerraFlow instance.
type App struct {
	// ID is the App ID (or client ID), the `iss` of the JWT.
	ID string
	// Key signs the JWT. It is the most sensitive value of the whole feature: it can
	// make a token for every installation, so it is never printed or returned.
	Key *rsa.PrivateKey

	// APIBase defaults to DefaultAPIBase; tests point it at a local server.
	APIBase string
	// WebBase is where branch links go (default https://github.com).
	WebBase string
	HTTP    *http.Client
	// Now is the clock (tests set it).
	Now func() time.Time
}

func (a *App) api() string {
	if a.APIBase != "" {
		return strings.TrimRight(a.APIBase, "/")
	}
	return DefaultAPIBase
}

func (a *App) web() string {
	if a.WebBase != "" {
		return strings.TrimRight(a.WebBase, "/")
	}
	return "https://github.com"
}

func (a *App) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// String never shows the key.
func (a *App) String() string { return "github.App{id=" + a.ID + ", key=<hidden>}" }

// ParsePrivateKey reads the PEM file GitHub gives out (PKCS#1 "RSA PRIVATE KEY", or
// PKCS#8). Errors never include the key material.
func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("the private key is not PEM text")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("the private key is not a valid RSA key")
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("the private key is not an RSA key")
	}
	return rk, nil
}

// LoadPrivateKey reads the key from a base64 value (for a host's secret store) or,
// failing that, from a PEM file path. Exactly one must be given.
func LoadPrivateKey(b64, file string, readFile func(string) ([]byte, error)) (*rsa.PrivateKey, error) {
	switch {
	case b64 != "" && file != "":
		return nil, errors.New("set only one of the private key file and the base64 key")
	case b64 != "":
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			return nil, errors.New("the base64 private key is not valid base64")
		}
		return ParsePrivateKey(raw)
	case file != "":
		raw, err := readFile(file)
		if err != nil {
			return nil, fmt.Errorf("cannot read the private key file: %w", errors.Unwrap(err))
		}
		return ParsePrivateKey(raw)
	default:
		return nil, errors.New("no GitHub App private key is configured")
	}
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// JWT returns a token that proves "I am this App". It lives nine minutes (GitHub
// allows ten) and starts a minute in the past to tolerate clock drift.
func (a *App) JWT() (string, error) {
	if a.Key == nil || a.ID == "" {
		return "", errors.New("the GitHub App is not configured")
	}
	now := a.now()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(9 * time.Minute).Unix(), "iss": a.ID})
	signing := b64url(header) + "." + b64url(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.Key, crypto.SHA256, sum[:])
	if err != nil {
		return "", errors.New("could not sign the GitHub App token")
	}
	return signing + "." + b64url(sig), nil
}

// Token is an installation token.
type Token struct {
	value   string
	Expires time.Time
}

// String never shows the value.
func (t Token) String() string { return "github.Token{<hidden>}" }

// GoString keeps %#v from showing it too.
func (t Token) GoString() string { return t.String() }

// InstallationToken makes a one-hour token for an installation, limited to one
// repository with `contents: write` and `metadata: read`.
func (a *App) InstallationToken(ctx context.Context, installationID int64, repo string) (Token, error) {
	jwt, err := a.JWT()
	if err != nil {
		return Token{}, err
	}
	body := map[string]any{
		"repositories": []string{repo},
		"permissions":  map[string]string{"contents": "write", "metadata": "read"},
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := a.call(ctx, jwt, http.MethodPost, "/app/installations/"+strconv.FormatInt(installationID, 10)+"/access_tokens", body, &out); err != nil {
		var he *HTTPError
		if errors.As(err, &he) && he.Status == http.StatusUnprocessableEntity {
			// The installation no longer includes this repository (removed or renamed).
			return Token{}, fmt.Errorf("%w: the GitHub App installation cannot access this repository", gitprovider.ErrUnauthorized)
		}
		return Token{}, err
	}
	if out.Token == "" {
		return Token{}, errors.New("GitHub returned no token")
	}
	return Token{value: out.Token, Expires: out.ExpiresAt}, nil
}

// call sends one API request with the given bearer credential and decodes the JSON
// answer into out (when not nil). The credential is only ever sent, never kept.
func (a *App) call(ctx context.Context, bearer, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.api()+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "TerraFlow")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client().Do(req)
	if err != nil {
		// The URL and method are safe to show; never wrap the request (headers).
		return fmt.Errorf("GitHub is not reachable: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("reading the GitHub answer: %w", err)
	}
	if resp.StatusCode >= 300 {
		return statusError(resp, raw, path)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("unexpected GitHub answer for %s", path)
		}
	}
	return nil
}

func unwrapURLError(err error) error {
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) && ue.Unwrap() != nil {
		return ue.Unwrap()
	}
	return err
}

// statusError turns an HTTP failure into the package's errors. The message names the
// status and GitHub's own short message, never a header or a credential.
func statusError(resp *http.Response, raw []byte, path string) error {
	var msg struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &msg)
	detail := msg.Message
	if len(detail) > 200 {
		detail = detail[:200]
	}
	limited := resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(detail), "rate limit"))
	switch {
	case limited:
		return fmt.Errorf("%w: %s", gitprovider.ErrRateLimited, detail)
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w (%d %s)", gitprovider.ErrUnauthorized, resp.StatusCode, detail)
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", gitprovider.ErrNotFound, path)
	default:
		return &HTTPError{Status: resp.StatusCode, Message: detail, Path: path}
	}
}

// HTTPError is a GitHub answer that is none of the cases above.
type HTTPError struct {
	Status  int
	Message string
	Path    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("GitHub answered %d for %s: %s", e.Status, e.Path, e.Message)
}

// Installation describes an installation of the App (used to verify one).
type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
		ID    int64  `json:"id"`
	} `json:"account"`
	RepositorySelection string `json:"repository_selection"`
}

// GetInstallation reads one installation with the App's own identity.
func (a *App) GetInstallation(ctx context.Context, id int64) (Installation, error) {
	jwt, err := a.JWT()
	if err != nil {
		return Installation{}, err
	}
	var out Installation
	err = a.call(ctx, jwt, http.MethodGet, "/app/installations/"+strconv.FormatInt(id, 10), nil, &out)
	return out, err
}
