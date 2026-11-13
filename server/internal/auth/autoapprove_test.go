package auth

import (
	"context"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func TestParseAutoApprove(t *testing.T) {
	a, err := ParseAutoApprove(" entra-tenant , GitHub-Org:My-Org ,, email-domain: Corp.Example ")
	if err != nil {
		t.Fatal(err)
	}
	want := AutoApprove{EntraTenant: true, GitHubOrgs: []string{"my-org"}, EmailDomains: []string{"corp.example"}}
	if !reflect.DeepEqual(a, want) || !a.NeedsGitHub() {
		t.Fatalf("got %+v", a)
	}
	if a, err := ParseAutoApprove(""); err != nil || a.EntraTenant || a.NeedsGitHub() {
		t.Fatalf("empty: %+v %v", a, err)
	}
	for _, bad := range []string{"everyone", "github-org:", "github-org:a/b", "github-org:-x", "email-domain:corp", "email-domain:@corp.example",
		"email-domain:*.corp.example", "entra-tenant:abc", "github-user:alice"} {
		if _, err := ParseAutoApprove(bad); err == nil {
			t.Errorf("%q must stop the start", bad)
		}
	}
	// Only the exact domain counts: no sub-domain, no look-alike ending.
	for email, ok := range map[string]bool{
		"a@corp.example": true, "A@CORP.EXAMPLE": true, "a@eu.corp.example": false, "a@notcorp.example": false,
		"a@corp.example.evil.test": false, "corp.example": false, "@corp.example": false,
	} {
		if got := want.domainMatches([]string{email}); got != ok {
			t.Errorf("domainMatches(%q) = %v, want %v", email, got, ok)
		}
	}
}

// signIn runs a whole sign-in with GitHub for a registered fake user and returns the
// status the person ends up with ("" when no session was started).
func signIn(t *testing.T, g *GitHubLogin, fake *githubtest.Fake, q *store.Queries, id int64) string {
	t.Helper()
	start := httptest.NewRecorder()
	g.Start(start, httptest.NewRequest("GET", "/auth/github/login", nil))
	loc, _ := url.Parse(start.Header().Get("Location"))
	req := httptest.NewRequest("GET", "/auth/github/callback?code="+fake.NewCode(id)+"&state="+url.QueryEscape(loc.Query().Get("state")), nil)
	req.AddCookie(cookieNamed(start, ghLoginStateCookie))
	req.AddCookie(cookieNamed(start, ghLoginVerifierCookie))
	rec := httptest.NewRecorder()
	g.Callback(rec, req)
	if cookieNamed(rec, sessionCookieName) == nil {
		return ""
	}
	status, err := q.UserStatus(context.Background(), "github:"+strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func TestAutoApproveRulesForGitHub(t *testing.T) {
	l, q := newLogin(t, "approval", "", 0)
	rules, err := ParseAutoApprove("github-org:acme,email-domain:corp.example")
	if err != nil {
		t.Fatal(err)
	}
	l.Policy.AutoApprove = rules
	g, fake := newGitHubLogin(t, l)

	fake.AddUser(1, "private-member", nil)
	fake.SetUserOrg(1, "Acme", false)
	fake.AddUser(2, "public-member", nil)
	fake.SetUserOrg(2, "acme", true)
	fake.AddUser(3, "elsewhere", nil)
	fake.SetUserOrg(3, "other", true)
	fake.AddUser(4, "verified", nil)
	fake.SetUserEmail(4, "dev@corp.example", true)
	fake.AddUser(5, "unverified", nil)
	fake.SetUserEmail(5, "boss@corp.example", false)
	fake.SetUserEmail(5, "me@home.test", true)
	fake.AddUser(6, "nobody", nil)

	for id, want := range map[int64]string{
		1: store.UserActive,  // GitHub confirms a private membership to the member
		2: store.UserActive,  // a public membership
		3: store.UserPending, // a member of another organisation
		4: store.UserActive,  // a verified address in the domain
		5: store.UserPending, // the address in the domain is not verified
		6: store.UserPending,
	} {
		if got := signIn(t, g, fake, q, id); got != want {
			t.Errorf("user %d: %q, want %q", id, got, want)
		}
	}
}

func TestSomeoneWaitingIsApprovedOnceARuleVouchesButNeverARejectedPerson(t *testing.T) {
	l, q := newLogin(t, "approval", "", 0)
	g, fake := newGitHubLogin(t, l)
	fake.AddUser(7, "late", nil)
	fake.AddUser(8, "refused", nil)
	for _, id := range []int64{7, 8} {
		if got := signIn(t, g, fake, q, id); got != store.UserPending {
			t.Fatalf("user %d before the rule: %q", id, got)
		}
	}
	if err := q.SetUserStatus(context.Background(), "github:8", store.UserRejected, "no", "admin"); err != nil {
		t.Fatal(err)
	}

	l.Policy.AutoApprove = AutoApprove{GitHubOrgs: []string{"acme"}}
	fake.SetUserOrg(7, "acme", true)
	fake.SetUserOrg(8, "acme", true)
	if got := signIn(t, g, fake, q, 7); got != store.UserActive {
		t.Fatalf("a waiting member: %q, want active", got)
	}
	if got := signIn(t, g, fake, q, 8); got != "" {
		t.Fatalf("a rejected person got a session with status %q", got)
	}
	if s, _ := q.UserStatus(context.Background(), "github:8"); s != store.UserRejected {
		t.Fatalf("a rule must not undo a rejection: %q", s)
	}
}

func TestSignInWithGitHubHandsTheProofToTheOwnerCheck(t *testing.T) {
	l, q := newLogin(t, "open", "", 0)
	g, fake := newGitHubLogin(t, l)
	fake.AddUser(9, "owner", nil)
	var got string
	g.Verified = func(_ context.Context, oid string, tok github.Token) {
		got = oid
		if strings.Contains(tok.String(), "ghu_") {
			t.Error("a token must never print its value")
		}
	}
	if s := signIn(t, g, fake, q, 9); s != store.UserActive || got != "github:9" {
		t.Fatalf("status %q, checked %q", s, got)
	}
}
