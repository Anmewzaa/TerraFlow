package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestPolicy(t *testing.T) {
	if _, err := NewPolicy("wide-open", "", 0); err == nil {
		t.Fatal("an unknown mode must stop the start")
	}
	p, err := NewPolicy("", " GitHub:42 , Boss@Corp.test ,, ", 5)
	if err != nil || p.Mode != SignupApproval {
		t.Fatalf("default mode: %+v %v", p, err)
	}
	for _, c := range []struct {
		id, email string
		want      bool
	}{
		{"github:42", "", true},
		{"GITHUB:42", "x@y.test", true},
		{"github:43", "boss@corp.test", true},
		{"github:43", "", false},
		{"alice", "", false}, // a login name never identifies anyone
	} {
		if got := p.IsAdmin(c.id, c.email); got != c.want {
			t.Errorf("IsAdmin(%q,%q) = %v, want %v", c.id, c.email, got, c.want)
		}
	}
	open, _ := NewPolicy("open", "", 0)
	inv, _ := NewPolicy("invite_only", "", 0)
	for name, c := range map[string]struct {
		p    Policy
		auto bool
		want string
	}{
		"approval":             {p, false, "pending"},
		"approval, vouched":    {p, true, "active"},
		"open":                 {open, false, "active"},
		"invite only":          {inv, false, ""},
		"invite only, vouched": {inv, true, "active"},
	} {
		if got := c.p.Initial(c.auto); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
}

func newLogin(t *testing.T, mode, admins string, maxPending int) (*Login, *store.Queries) {
	t.Helper()
	q, _ := storetest.New(t)
	p, err := NewPolicy(mode, admins, maxPending)
	if err != nil {
		t.Fatal(err)
	}
	return &Login{Queries: q, Policy: p, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Insecure: true}, q
}

// finish runs Login.Finish and returns where the browser is sent and the session cookie.
func finish(l *Login, id store.LoginUser, auto bool) (loc string, session string) {
	rec := httptest.NewRecorder()
	l.Finish(rec, httptest.NewRequest("GET", "/auth/callback", nil), id, nil, nil, auto)
	if c := cookieNamed(rec, sessionCookieName); c != nil {
		session = c.Value
	}
	return rec.Header().Get("Location"), session
}

func call(q *store.Queries, session, path string) (int, *User) {
	var got *User
	h := RequireAuth(q)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = UserFromContext(r.Context()) }))
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, got
}

func TestANewPersonWaitsForApprovalAndCanDoNothingElse(t *testing.T) {
	l, q := newLogin(t, "approval", "", 0)
	id := store.LoginUser{OID: "github:7", Provider: "github", Login: "newbie", Name: "Newbie"}
	loc, sid := finish(l, id, false)
	if loc != "/" || sid == "" {
		t.Fatalf("pending person: redirect %q, session %q", loc, sid)
	}
	// They may ask who they are (the screen shows "waiting"), and nothing else.
	if code, u := call(q, sid, "/api/me"); code != 200 || u == nil || u.Status != store.UserPending {
		t.Fatalf("/api/me: %d %+v", code, u)
	}
	for _, path := range []string{"/api/projects", "/api/github/repos", "/api/catalog"} {
		if code, u := call(q, sid, path); code != http.StatusForbidden || u != nil {
			t.Errorf("%s while pending: %d %v, want 403", path, code, u)
		}
	}
	// Approving takes effect at once, without signing in again.
	if err := q.SetUserStatus(context.Background(), id.OID, store.UserActive, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if code, u := call(q, sid, "/api/projects"); code != 200 || u == nil || u.Status != "" {
		t.Fatalf("after approval: %d %+v", code, u)
	}
	// Suspending takes effect at once too, and ends the session.
	if err := q.SetUserStatus(context.Background(), id.OID, store.UserSuspended, "left", "admin"); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(q, sid, "/api/projects"); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("after suspension: %d", code)
	}
	if loc, sid2 := finish(l, id, false); sid2 != "" || !strings.Contains(loc, "signin=suspended") {
		t.Fatalf("a suspended person signing in: %q %q", loc, sid2)
	}
}

func TestRejectedPersonCannotSignUpAgain(t *testing.T) {
	l, q := newLogin(t, "approval", "", 0)
	id := store.LoginUser{OID: "github:8", Provider: "github"}
	finish(l, id, false)
	if err := q.SetUserStatus(context.Background(), id.OID, store.UserRejected, "not on the team", "admin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if loc, sid := finish(l, id, false); sid != "" || !strings.Contains(loc, "signin=rejected") {
			t.Fatalf("attempt %d: %q %q", i, loc, sid)
		}
	}
}

func TestAdminsNamedInTheSettingsAreActiveAtOnceWithTheAdminRole(t *testing.T) {
	l, q := newLogin(t, "approval", "github:1", 1)
	// The approval list is already full, and the administrator still gets in.
	finish(l, store.LoginUser{OID: "github:50", Provider: "github"}, false)
	_, sid := finish(l, store.LoginUser{OID: "github:1", Provider: "github", Name: "Boss"}, false)
	code, u := call(q, sid, "/api/projects")
	if code != 200 || u == nil || !hasRole(u.Roles, "devops") {
		t.Fatalf("admin: %d %+v", code, u)
	}
	// An ordinary person gets no admin role, and a full list refuses new sign-ups.
	if loc, sid := finish(l, store.LoginUser{OID: "github:51", Provider: "github"}, false); sid != "" || !strings.Contains(loc, "signin=full") {
		t.Fatalf("full list: %q %q", loc, sid)
	}
}

func TestModes(t *testing.T) {
	open, q := newLogin(t, "open", "", 0)
	_, sid := finish(open, store.LoginUser{OID: "github:9", Provider: "github"}, false)
	if code, u := call(q, sid, "/api/projects"); code != 200 || u == nil || hasRole(u.Roles, "devops") {
		t.Fatalf("open mode: %d %+v", code, u)
	}
	inv, _ := newLogin(t, "invite_only", "", 0)
	if loc, sid := finish(inv, store.LoginUser{OID: "github:10", Provider: "github"}, false); sid != "" || !strings.Contains(loc, "signin=not_invited") {
		t.Fatalf("invite only: %q %q", loc, sid)
	}
}

func newGitHubLogin(t *testing.T, l *Login) (*GitHubLogin, *githubtest.Fake) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	fake := githubtest.New("1", &k.PublicKey, now)
	t.Cleanup(fake.Close)
	app := &github.App{ID: "1", Key: k, APIBase: fake.URL, WebBase: fake.URL, ClientID: githubtest.ClientID, ClientSecret: githubtest.ClientSecret, Now: now}
	return &GitHubLogin{App: app, RedirectURI: "http://tf.test/auth/github/callback", Login: l}, fake
}

func TestGitHubSignInUsesStateAndPKCEAndTheNumericId(t *testing.T) {
	l, q := newLogin(t, "approval", "", 0)
	g, fake := newGitHubLogin(t, l)

	start := httptest.NewRecorder()
	g.Start(start, httptest.NewRequest("GET", "/auth/github/login", nil))
	loc, _ := url.Parse(start.Header().Get("Location"))
	state := loc.Query().Get("state")
	if state == "" || loc.Query().Get("code_challenge") == "" || loc.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("authorize url lacks state or PKCE: %s", loc)
	}
	state2, ver := cookieNamed(start, ghLoginStateCookie), cookieNamed(start, ghLoginVerifierCookie)
	if state2 == nil || ver == nil || state2.Value != state || !state2.HttpOnly || ver.Value == loc.Query().Get("code_challenge") {
		t.Fatalf("cookies: %v %v", state2, ver)
	}

	code := fake.AddUser(4242, "Alice-Login", nil)
	cb := func(state string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/auth/github/callback?code="+code+"&state="+url.QueryEscape(state), nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		g.Callback(rec, req)
		return rec
	}
	// A wrong state, or no cookies, never signs anyone in.
	if rec := cb("forged", state2, ver); cookieNamed(rec, sessionCookieName) != nil || !strings.Contains(rec.Header().Get("Location"), "signin=state") {
		t.Fatalf("forged state: %s", rec.Header().Get("Location"))
	}
	if rec := cb(state); cookieNamed(rec, sessionCookieName) != nil {
		t.Fatal("no cookies must not sign in")
	}
	code = fake.AddUser(4242, "Alice-Login", nil)
	rec := cb(state, state2, ver)
	sc := cookieNamed(rec, sessionCookieName)
	if sc == nil {
		t.Fatalf("no session: %s", rec.Header().Get("Location"))
	}
	// The identity is the numeric id, never the login name; a new person is pending.
	if code, u := call(q, sc.Value, "/api/me"); code != 200 || u.OID != "github:4242" || u.Status != store.UserPending {
		t.Fatalf("me: %d %+v", code, u)
	}
}

func TestAdminIsMatchedByIdNotByLoginName(t *testing.T) {
	l, q := newLogin(t, "approval", "github:1", 0)
	// A person who merely calls themselves "github:1" is not the administrator.
	_, sid := finish(l, store.LoginUser{OID: "github:2", Provider: "github", Login: "github:1"}, false)
	if _, u := call(q, sid, "/api/me"); u == nil || hasRole(u.Roles, "devops") || u.Status != store.UserPending {
		t.Fatalf("a login name must never make an administrator: %+v", u)
	}
}

func TestPendingAndRejectedPeopleNeverHoldAnAdminRoleOrASession(t *testing.T) {
	l, q := newLogin(t, "approval", "", 0)
	id := store.LoginUser{OID: "github:9", Provider: "github"}
	_, sid := finish(l, id, false)
	if _, u := call(q, sid, "/api/me"); u == nil || hasRole(u.Roles, "devops") {
		t.Fatalf("a pending person must not be an administrator: %+v", u)
	}
	if err := q.SetUserStatus(context.Background(), id.OID, store.UserRejected, "no", "admin"); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(q, sid, "/api/me"); code != http.StatusUnauthorized {
		t.Fatalf("the old session of a rejected person = %d", code)
	}
}

func TestGitHubAndEntraAccountsAreNeverLinked(t *testing.T) {
	l, q := newLogin(t, "open", "", 0)
	// An Entra object id equal to a GitHub id, and the same e-mail: still two people.
	finish(l, store.LoginUser{OID: "42", Provider: "entra", Email: "alice@corp.test", Name: "Alice (Entra)"}, false)
	finish(l, store.LoginUser{OID: "github:42", Provider: "github", Email: "alice@corp.test", Name: "alice"}, false)
	users, err := q.ListUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, u := range users {
		got[u.OID] = u.Provider
	}
	if got["42"] != "entra" || got["github:42"] != "github" || len(got) != 2 {
		t.Fatalf("users = %v", got)
	}
}

func TestGitHubCallbackRefusesDenialAndBadCodes(t *testing.T) {
	l, q := newLogin(t, "open", "", 0)
	g, fake := newGitHubLogin(t, l)
	fake.AddUser(3, "x", nil)
	start := httptest.NewRecorder()
	g.Start(start, httptest.NewRequest("GET", "/auth/github/login", nil))
	state, ver := cookieNamed(start, ghLoginStateCookie), cookieNamed(start, ghLoginVerifierCookie)
	if state == nil || ver == nil {
		t.Fatal("no cookies")
	}
	for name, path := range map[string]string{
		"user denied": "/auth/github/callback?error=access_denied&state=" + url.QueryEscape(state.Value),
		"a bad code":  "/auth/github/callback?code=nope&state=" + url.QueryEscape(state.Value),
		"empty state": "/auth/github/callback?code=nope",
	} {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(state)
		req.AddCookie(ver)
		rec := httptest.NewRecorder()
		g.Callback(rec, req)
		if cookieNamed(rec, sessionCookieName) != nil || rec.Code != http.StatusFound {
			t.Errorf("%s: %d, session %v", name, rec.Code, cookieNamed(rec, sessionCookieName))
		}
	}
	if s, _ := q.UserStatus(context.Background(), "github:3"); s != "" {
		t.Error("a failed sign-in must not create a user")
	}
}
