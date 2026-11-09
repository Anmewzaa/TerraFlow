package auth

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

// Phase 7 security tests for sign-in: role mapping, the OIDC redirect, the
// callback's state check, the session middleware and the dev sign-in.

func testHandlers(insecure bool, q *store.Queries) *Handlers {
	return &Handlers{
		oidc: &OIDCProvider{oauth: oauth2.Config{
			ClientID:    "client-1",
			RedirectURL: "https://tf.example/auth/callback",
			Endpoint:    oauth2.Endpoint{AuthURL: "https://login.example/authorize", TokenURL: "https://login.example/token"},
			Scopes:      []string{"openid"},
		}},
		queries:  q,
		roles:    RoleConfig{DevOpsGroupID: "g-ops", DeveloperGroupID: "g-dev"},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		insecure: insecure,
	}
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestMapRoles(t *testing.T) {
	cfg := RoleConfig{DevOpsGroupID: "g-ops", DeveloperGroupID: "g-dev"}
	tests := []struct {
		name   string
		cfg    RoleConfig
		groups []string
		want   []string
	}{
		{"devops group", cfg, []string{"g-ops"}, []string{"devops"}},
		{"developer group", cfg, []string{"g-dev"}, []string{"developer"}},
		{"both groups give both roles", cfg, []string{"g-dev", "g-ops"}, []string{"developer", "devops"}},
		{"unrelated groups give nothing", cfg, []string{"g-other", "g-x"}, nil},
		{"no groups give nothing", cfg, nil, nil},
		{"empty group list gives nothing", cfg, []string{}, nil},
		{"group ids are exact, not prefixes", cfg, []string{"g-ops-2", "G-OPS"}, nil},
		{"unconfigured mapping falls back to developer", RoleConfig{}, []string{"g-ops"}, []string{"developer"}},
		{"only devops configured, developer group ignored", RoleConfig{DevOpsGroupID: "g-ops"}, []string{"g-dev"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MapRoles(tc.cfg, tc.groups)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLogin_RedirectAndCookies(t *testing.T) {
	for _, insecure := range []bool{false, true} {
		h := testHandlers(insecure, nil)
		rec := httptest.NewRecorder()
		h.Login(rec, httptest.NewRequest("GET", "/auth/login", nil))
		if rec.Code != http.StatusFound {
			t.Fatalf("insecure=%v: status %d", insecure, rec.Code)
		}
		loc, err := url.Parse(rec.Header().Get("Location"))
		if err != nil || loc.Host != "login.example" {
			t.Fatalf("redirect %q: %v", rec.Header().Get("Location"), err)
		}
		q := loc.Query()
		if q.Get("client_id") != "client-1" || q.Get("response_type") != "code" || q.Get("redirect_uri") != "https://tf.example/auth/callback" {
			t.Errorf("authorize params: %v", q)
		}
		state, nonce := cookieNamed(rec, "tf_oidc_state"), cookieNamed(rec, "tf_oidc_nonce")
		if state == nil || nonce == nil {
			t.Fatalf("state and nonce cookies must be set: %v", rec.Result().Cookies())
		}
		if q.Get("state") != state.Value || q.Get("nonce") != nonce.Value {
			t.Errorf("the redirect must carry the cookie's state and nonce")
		}
		if len(state.Value) < 32 || len(nonce.Value) < 32 || state.Value == nonce.Value {
			t.Errorf("state/nonce must be long and different: %q %q", state.Value, nonce.Value)
		}
		for _, c := range []*http.Cookie{state, nonce} {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 300 || c.Path != "/" {
				t.Errorf("%s flags: %+v", c.Name, c)
			}
			if c.Secure == insecure {
				t.Errorf("%s Secure=%v with insecure=%v", c.Name, c.Secure, insecure)
			}
		}
	}
}

func TestLogin_FreshStateEveryTime(t *testing.T) {
	h := testHandlers(false, nil)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		rec := httptest.NewRecorder()
		h.Login(rec, httptest.NewRequest("GET", "/auth/login", nil))
		v := cookieNamed(rec, "tf_oidc_state").Value
		if seen[v] {
			t.Fatalf("state value repeated: %q", v)
		}
		seen[v] = true
	}
}

func TestCallback_RejectsBadState(t *testing.T) {
	h := testHandlers(false, nil)
	tests := []struct {
		name   string
		target string
		cookie string
	}{
		{"no cookie", "/auth/callback?state=abc&code=c", ""},
		{"no state in query", "/auth/callback?code=c", "abc"},
		{"different state", "/auth/callback?state=other&code=c", "abc"},
		{"empty both", "/auth/callback?code=c", ""},
		{"case differs", "/auth/callback?state=ABC&code=c", "abc"},
		{"error from the provider, no state", "/auth/callback?error=access_denied", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.target, nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "tf_oidc_state", Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			h.Callback(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400", rec.Code)
			}
			if c := cookieNamed(rec, sessionCookieName); c != nil {
				t.Fatalf("no session may be created: %+v", c)
			}
		})
	}
}

func TestLogin_UsesPKCE(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandlers(false, nil).Login(rec, httptest.NewRequest("GET", "/auth/login", nil))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("code_challenge_method") != "S256" || loc.Query().Get("code_challenge") == "" {
		t.Fatalf("PKCE S256 challenge missing: %v", loc.Query())
	}
	v := cookieNamed(rec, "tf_oidc_verifier")
	if v == nil || !v.HttpOnly || v.MaxAge != 300 || len(v.Value) < 43 {
		t.Fatalf("the verifier must be a long httpOnly short-lived cookie: %+v", v)
	}
	// The challenge is the S256 hash of the verifier, and the verifier itself
	// never appears in the redirect.
	if loc.Query().Get("code_challenge") != oauth2.S256ChallengeFromVerifier(v.Value) {
		t.Error("code_challenge must be S256(verifier)")
	}
	if strings.Contains(rec.Header().Get("Location"), v.Value) {
		t.Error("the verifier must not be sent to the provider in the authorize request")
	}
}

func TestLogin_NewVerifierEveryTime(t *testing.T) {
	h := testHandlers(false, nil)
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		h.Login(rec, httptest.NewRequest("GET", "/auth/login", nil))
		v := cookieNamed(rec, "tf_oidc_verifier").Value
		if seen[v] {
			t.Fatal("verifier repeated")
		}
		seen[v] = true
	}
}

func TestCallback_NeedsTheVerifierCookie(t *testing.T) {
	h := testHandlers(false, nil)
	req := httptest.NewRequest("GET", "/auth/callback?state=abc&code=c", nil)
	req.AddCookie(&http.Cookie{Name: "tf_oidc_state", Value: "abc"})
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || cookieNamed(rec, sessionCookieName) != nil {
		t.Fatalf("without the verifier: %d", rec.Code)
	}
}

func TestNonceMatches(t *testing.T) {
	c := &http.Cookie{Name: "tf_oidc_nonce", Value: "n-1"}
	tests := []struct {
		name  string
		token string
		c     *http.Cookie
		err   error
		want  bool
	}{
		{"same nonce", "n-1", c, nil, true},
		{"another sign-in's nonce", "n-2", c, nil, false},
		{"token without a nonce", "", c, nil, false},
		{"no cookie", "n-1", nil, http.ErrNoCookie, false},
		{"empty cookie and empty claim", "", &http.Cookie{Name: "tf_oidc_nonce"}, nil, false},
		{"prefix is not enough", "n-", c, nil, false},
	}
	for _, tc := range tests {
		if got := nonceMatches(tc.token, tc.c, tc.err); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestSessionMiddleware(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	mustSession := func(id string, exp time.Time, roles []string) {
		t.Helper()
		err := q.CreateSession(ctx, store.Session{ID: id, UserOID: "u-" + id, Email: id + "@x.test", Name: id, Roles: roles, GroupIDs: []string{"g1"}, ExpiresAt: exp})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.EnsureUser(context.Background(), "u-live", "", "live", store.StatusActive, 0); err != nil {
		t.Fatal(err)
	}
	mustSession("live", time.Now().Add(time.Hour), []string{"devops"})
	mustSession("expired", time.Now().Add(-time.Minute), []string{"devops"})

	var got *User
	h := RequireAuth(q)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = UserFromContext(r.Context()) }))
	call := func(cookie string) int {
		got = nil
		req := httptest.NewRequest("GET", "/api/me", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	for name, cookie := range map[string]string{
		"no cookie":       "",
		"unknown session": "nope",
		"expired session": "expired",
		"sql-ish value":   "x' OR '1'='1",
	} {
		if code := call(cookie); code != http.StatusUnauthorized || got != nil {
			t.Errorf("%s: status %d user %v, want 401 and no user", name, code, got)
		}
	}
	if code := call("live"); code != http.StatusOK || got == nil {
		t.Fatalf("live session: %d %v", code, got)
	}
	if got.OID != "u-live" || !reflect.DeepEqual(got.Roles, []string{"devops"}) || !reflect.DeepEqual(got.GroupIDs, []string{"g1"}) {
		t.Fatalf("user from session: %+v", got)
	}

	// A cookie with another name never authenticates.
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "live"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong cookie name: %d", rec.Code)
	}
}

func TestLogout_EndsTheSession(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	if err := q.CreateSession(ctx, store.Session{ID: "s1", UserOID: "u", Roles: []string{}, GroupIDs: []string{}, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	h := testHandlers(false, q)

	req := httptest.NewRequest("GET", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s1"})
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if c := cookieNamed(rec, sessionCookieName); c == nil || c.MaxAge >= 0 {
		t.Fatalf("the session cookie must be cleared: %+v", c)
	}
	if _, err := q.GetSession(ctx, "s1"); err == nil {
		t.Fatal("the session row must be gone, so the old cookie is useless")
	}
	// Logging out without a session is harmless.
	rec = httptest.NewRecorder()
	h.Logout(rec, httptest.NewRequest("GET", "/auth/logout", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("logout without a session: %d", rec.Code)
	}
}

func TestMe_RequiresUser(t *testing.T) {
	h := testHandlers(false, nil)
	rec := httptest.NewRecorder()
	h.Me(rec, httptest.NewRequest("GET", "/api/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

// --- Dev sign-in (kept on purpose: it is how both roles are tried locally) ---

func TestDevUsers_OnlyOneAdmin(t *testing.T) {
	admins, ids := 0, map[string]bool{}
	for _, u := range DevUsers {
		if u.Role == "devops" {
			admins++
		}
		if ids[u.OID] {
			t.Errorf("duplicate dev user %s", u.OID)
		}
		ids[u.OID] = true
		if !strings.HasPrefix(u.OID, "dev-") || !strings.HasSuffix(u.Email, "@terraflow.local") {
			t.Errorf("dev users must be recognisably fake: %+v", u)
		}
	}
	if admins != 1 {
		t.Fatalf("exactly one dev admin expected, got %d", admins)
	}
}

func TestDevLogin_RejectsBadInput(t *testing.T) {
	var d DevLogin
	for name, body := range map[string]string{
		"unknown oid":    `{"oid":"dev-root"}`,
		"empty oid":      `{"oid":""}`,
		"not json":       `oid=dev-admin-0001`,
		"empty body":     ``,
		"wrong type":     `{"oid":["dev-admin-0001"]}`,
		"role injection": `{"oid":"dev-alice-0002","role":"devops"}`,
		"oversize body":  `{"oid":"` + strings.Repeat("a", 4096) + `"}`,
	} {
		rec := httptest.NewRecorder()
		d.Login(rec, httptest.NewRequest("POST", "/api/dev/login", strings.NewReader(body)))
		if name == "role injection" {
			// Extra fields are ignored; the role still comes from the fixed user list.
			req := httptest.NewRequest("GET", "/api/me", nil)
			req.AddCookie(rec.Result().Cookies()[0])
			if got := who(req); got != "dev-alice-0002:developer" {
				t.Errorf("%s: got %q", name, got)
			}
			continue
		}
		if rec.Code != http.StatusBadRequest || len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: status %d, cookies %v", name, rec.Code, rec.Result().Cookies())
		}
	}
}

func TestDevLogin_CookieFlagsAndLogout(t *testing.T) {
	var d DevLogin
	rec := httptest.NewRecorder()
	d.Login(rec, httptest.NewRequest("POST", "/api/dev/login", strings.NewReader(`{"oid":"dev-admin-0001"}`)))
	c := cookieNamed(rec, DevCookie)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("dev cookie: %+v", c)
	}
	rec = httptest.NewRecorder()
	d.Logout(rec, httptest.NewRequest("POST", "/api/dev/logout", nil))
	if c := cookieNamed(rec, DevCookie); c == nil || c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("logout must clear the cookie: %+v", c)
	}
	// After logout the middleware refuses the request again.
	if got := who(httptest.NewRequest("GET", "/api/me", nil)); got != "denied" {
		t.Fatalf("got %q", got)
	}
}

func TestDevBypass_UsesTheGivenRole(t *testing.T) {
	for _, role := range []string{"developer", "devops"} {
		var got *User
		h := DevBypass(role)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = UserFromContext(r.Context()) }))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if got == nil || got.OID != DevUserOID || !reflect.DeepEqual(got.Roles, []string{role}) {
			t.Fatalf("role %s: %+v", role, got)
		}
	}
}
