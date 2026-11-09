package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

// Sign-in with GitHub and the sign-up approval (D-050), against a fake GitHub and a
// real Postgres.

type loginEnv struct {
	t    *testing.T
	q    *store.Queries
	fake *githubtest.Fake
	gl   *GitHubLogin
	mw   http.Handler
}

func newLoginEnv(t *testing.T, policy SignupPolicy) *loginEnv {
	t.Helper()
	q, _ := storetest.New(t)
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	fake := githubtest.New("1", &k.PublicKey, now)
	t.Cleanup(fake.Close)
	h := (&Handlers{queries: q, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), insecure: true}).WithSignup(policy)
	gl := &GitHubLogin{
		App:         &github.App{ID: "1", Key: k, APIBase: fake.URL, WebBase: fake.URL, ClientID: githubtest.ClientID, ClientSecret: githubtest.ClientSecret, Now: now},
		RedirectURI: "http://tf.test/auth/github/callback", Handlers: h,
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	return &loginEnv{t: t, q: q, fake: fake, gl: gl, mw: RequireAuth(q)(ok)}
}

// signIn runs the browser round trip for a GitHub account and returns the callback
// answer and the session cookie it set (nil when none).
func (e *loginEnv) signIn(id int64, login string) (*httptest.ResponseRecorder, *http.Cookie) {
	e.t.Helper()
	e.fake.AddUser(id, login, nil)
	w := httptest.NewRecorder()
	e.gl.Login(w, httptest.NewRequest("GET", "/auth/github/login", nil))
	state := cookieNamed(w, ghStateCookie)
	loc, _ := url.Parse(w.Header().Get("Location"))
	if state == nil || loc.Query().Get("state") != state.Value {
		e.t.Fatal("the redirect must carry the state of the cookie")
	}
	req := httptest.NewRequest("GET", "/auth/github/callback?code="+e.fake.NewCode(id)+"&state="+state.Value, nil)
	req.AddCookie(state)
	cb := httptest.NewRecorder()
	e.gl.Callback(cb, req)
	return cb, cookieNamed(cb, sessionCookieName)
}

func (e *loginEnv) call(c *http.Cookie, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	if c != nil {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.mw.ServeHTTP(w, req)
	return w
}

func TestNewGitHubUserWaitsForApproval(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{})
	cb, c := e.signIn(42, "alice")
	if cb.Code != http.StatusFound || c == nil {
		t.Fatalf("callback = %d, session %v", cb.Code, c)
	}
	if got := e.call(c, "/api/me").Code; got != 200 {
		t.Errorf("a pending user may ask who they are: %d", got)
	}
	for _, p := range []string{"/api/projects", "/api/github/repos", "/api/projects/from-repo"} {
		if got := e.call(c, p); got.Code != http.StatusForbidden {
			t.Errorf("a pending user reached %s: %d", p, got.Code)
		}
	}
	st, err := e.q.GetUserStatus(t.Context(), "github:42")
	if err != nil || st.Status != store.StatusPending {
		t.Fatalf("status = %+v, %v", st, err)
	}
	// Approved: the same session works at once, no new sign-in needed.
	if err := e.q.SetUserStatus(t.Context(), "github:42", store.StatusActive, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := e.call(c, "/api/projects").Code; got != 200 {
		t.Errorf("after approval = %d", got)
	}
}

func TestPendingUserNeverGetsAnAdminRole(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{})
	_, c := e.signIn(42, "alice")
	sess, err := e.q.GetSession(t.Context(), c.Value)
	if err != nil {
		t.Fatal(err)
	}
	if hasRole(sess.Roles, "devops") {
		t.Fatal("a pending user must not have the admin role")
	}
}

func TestListedAdminIsActiveAtOnceWithTheAdminRole(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{Admins: []string{"github:1", "root@corp.test"}})
	_, c := e.signIn(1, "boss")
	if got := e.call(c, "/api/projects").Code; got != 200 {
		t.Fatalf("a listed admin = %d", got)
	}
	sess, _ := e.q.GetSession(t.Context(), c.Value)
	if !hasRole(sess.Roles, "devops") {
		t.Errorf("roles = %v", sess.Roles)
	}
	// A login name is not an identity: a person called "github:1" or any login cannot
	// become admin by choosing a name, and a different id is not admin.
	_, c2 := e.signIn(2, "github:1")
	if sess, _ := e.q.GetSession(t.Context(), c2.Value); hasRole(sess.Roles, "devops") {
		t.Error("an admin must be matched by id, not by login name")
	}
}

func TestOpenAndInviteOnlyModes(t *testing.T) {
	open := newLoginEnv(t, SignupPolicy{Mode: SignupOpen})
	_, c := open.signIn(5, "eve")
	if got := open.call(c, "/api/projects").Code; got != 200 {
		t.Errorf("open mode = %d", got)
	}

	inv := newLoginEnv(t, SignupPolicy{Mode: SignupInviteOnly})
	cb, c := inv.signIn(6, "mallory")
	if c != nil || cb.Header().Get("Location") != "/?signin=invite_only" {
		t.Errorf("an unknown user in invite-only mode: %v %v", c, cb.Header().Get("Location"))
	}
	if ok, _ := inv.q.UserExists(t.Context(), "github:6"); ok {
		t.Error("a refused sign-up must not leave a user row")
	}
	// Someone already known may sign in.
	if _, err := inv.q.EnsureUser(t.Context(), "github:7", "", "dan", store.StatusActive, 0); err != nil {
		t.Fatal(err)
	}
	if _, c := inv.signIn(7, "dan"); c == nil {
		t.Error("a known user must be able to sign in")
	}
}

func TestRejectedAndSuspendedUsersGetNoSession(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{})
	_, c := e.signIn(8, "carol")
	if err := e.q.SetUserStatus(t.Context(), "github:8", store.StatusRejected, "no", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := e.call(c, "/api/me").Code; got != http.StatusUnauthorized {
		t.Errorf("the old session of a rejected user = %d", got)
	}
	cb, c := e.signIn(8, "carol")
	if c != nil || cb.Header().Get("Location") != "/?signin=rejected" {
		t.Errorf("rejected sign-in: %v %v", c, cb.Header().Get("Location"))
	}
	_, _ = e.q.EnsureUser(t.Context(), "github:9", "", "x", store.StatusActive, 0)
	_ = e.q.SetUserStatus(t.Context(), "github:9", store.StatusSuspended, "left", "admin")
	if cb, c := e.signIn(9, "x"); c != nil || cb.Header().Get("Location") != "/?signin=suspended" {
		t.Errorf("suspended sign-in: %v %v", c, cb.Header().Get("Location"))
	}
}

func TestPendingCapRefusesNewSignUps(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{MaxPending: 1})
	e.signIn(10, "a")
	cb, c := e.signIn(11, "b")
	if c != nil || cb.Header().Get("Location") != "/?signin=full" {
		t.Errorf("over the cap: %v %v", c, cb.Header().Get("Location"))
	}
}

func TestGitHubCallbackChecksStateAndDenial(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{Mode: SignupOpen})
	e.fake.AddUser(3, "x", nil)
	code := e.fake.NewCode(3)
	for name, req := range map[string]*http.Request{
		"no cookie":   httptest.NewRequest("GET", "/auth/github/callback?code="+code+"&state=abc", nil),
		"wrong state": withCookie(httptest.NewRequest("GET", "/auth/github/callback?code="+code+"&state=abc", nil), ghStateCookie, "other"),
		"empty state": withCookie(httptest.NewRequest("GET", "/auth/github/callback?code="+code, nil), ghStateCookie, ""),
		"user denied": withCookie(httptest.NewRequest("GET", "/auth/github/callback?error=access_denied&state=abc", nil), ghStateCookie, "abc"),
		"a bad code":  withCookie(httptest.NewRequest("GET", "/auth/github/callback?code=nope&state=abc", nil), ghStateCookie, "abc"),
	} {
		w := httptest.NewRecorder()
		e.gl.Callback(w, req)
		if cookieNamed(w, sessionCookieName) != nil || w.Code != http.StatusFound {
			t.Errorf("%s: %d, session %v", name, w.Code, cookieNamed(w, sessionCookieName))
		}
	}
	if ok, _ := e.q.UserExists(t.Context(), "github:3"); ok {
		t.Error("a failed sign-in must not create a user")
	}
}

func withCookie(r *http.Request, name, value string) *http.Request {
	r.AddCookie(&http.Cookie{Name: name, Value: value})
	return r
}

func TestGitHubAndEntraAccountsAreNeverLinked(t *testing.T) {
	e := newLoginEnv(t, SignupPolicy{Mode: SignupOpen})
	// An Entra user whose object id happens to equal a GitHub id, and one with the
	// same email: still two people.
	if _, err := e.q.EnsureUser(t.Context(), "42", "alice@corp.test", "Alice (Entra)", store.StatusActive, 0); err != nil {
		t.Fatal(err)
	}
	e.signIn(42, "alice")
	users, _ := e.q.ListUsers(t.Context())
	got := map[string]string{}
	for _, u := range users {
		got[u.OID] = u.Provider
	}
	if got["42"] != "entra" || got["github:42"] != "github" || len(got) != 2 {
		t.Errorf("users = %v", got)
	}
	if ProviderOfForTest("github:"+strconv.Itoa(42)) != "github" {
		t.Error("provider")
	}
}

func ProviderOfForTest(oid string) string { return store.ProviderOf(oid) }

func TestSignupPolicy(t *testing.T) {
	cases := []struct {
		p     SignupPolicy
		oid   string
		email string
		admin bool
		want  string
	}{
		{SignupPolicy{}, "github:1", "", false, store.StatusPending},
		{SignupPolicy{Mode: SignupApproval}, "github:1", "", false, store.StatusPending},
		{SignupPolicy{Mode: SignupOpen}, "github:1", "", false, store.StatusActive},
		{SignupPolicy{Mode: SignupInviteOnly}, "github:1", "", false, ""},
		{SignupPolicy{Admins: []string{"github:1"}}, "github:1", "", false, store.StatusActive},
		{SignupPolicy{Admins: []string{"Boss@Corp.test"}}, "9", "boss@corp.test", false, store.StatusActive},
		{SignupPolicy{Mode: SignupInviteOnly}, "9", "", true, store.StatusActive},
		{SignupPolicy{Admins: []string{""}}, "", "", false, store.StatusPending},
	}
	for i, c := range cases {
		if got := c.p.Initial(c.oid, c.email, c.admin); got != c.want {
			t.Errorf("case %d: %q, want %q", i, got, c.want)
		}
	}
}
