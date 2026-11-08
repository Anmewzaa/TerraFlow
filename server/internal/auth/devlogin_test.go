package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// who runs a request through the middleware and returns "oid:role", or "denied".
func who(r *http.Request) string {
	var d DevLogin
	got := "denied"
	h := d.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		got = u.OID + ":" + u.Roles[0]
	}))
	serve(h, r)
	return got
}

func TestDevLogin_PickAUserThenActAsThem(t *testing.T) {
	var d DevLogin
	if got := who(httptest.NewRequest("GET", "/api/me", nil)); got != "denied" {
		t.Fatalf("no cookie must be refused, got %q", got)
	}
	bad := httptest.NewRequest("GET", "/api/me", nil)
	bad.AddCookie(&http.Cookie{Name: DevCookie, Value: "root"})
	if got := who(bad); got != "denied" {
		t.Fatalf("an unknown user must be refused, got %q", got)
	}

	rec := serve(http.HandlerFunc(d.Login), httptest.NewRequest("POST", "/api/dev/login", strings.NewReader(`{"oid":"dev-alice-0002"}`)))
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("login: %d %v", rec.Code, rec.Result().Cookies())
	}
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	if got := who(req); got != "dev-alice-0002:developer" {
		t.Fatalf("got %q", got)
	}
	admin := httptest.NewRequest("GET", "/api/me", nil)
	admin.AddCookie(&http.Cookie{Name: DevCookie, Value: "dev-admin-0001"})
	if got := who(admin); got != "dev-admin-0001:devops" {
		t.Fatalf("got %q", got)
	}
}

func TestDevLogin_UsersListAndLogout(t *testing.T) {
	var d DevLogin
	rec := serve(http.HandlerFunc(d.Users), httptest.NewRequest("GET", "/api/dev/users", nil))
	if !strings.Contains(rec.Body.String(), `"current":""`) || !strings.Contains(rec.Body.String(), "dev-admin-0001") {
		t.Fatalf("users: %s", rec.Body)
	}
	r := httptest.NewRequest("GET", "/api/dev/users", nil)
	r.AddCookie(&http.Cookie{Name: DevCookie, Value: "dev-bob-0003"})
	if rec := serve(http.HandlerFunc(d.Users), r); !strings.Contains(rec.Body.String(), `"current":"dev-bob-0003"`) {
		t.Fatalf("current user: %s", rec.Body)
	}
	if rec := serve(http.HandlerFunc(d.Login), httptest.NewRequest("POST", "/api/dev/login", strings.NewReader(`{"oid":"nobody"}`))); rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown oid must be 400, got %d", rec.Code)
	}
	out := serve(http.HandlerFunc(d.Logout), httptest.NewRequest("POST", "/api/dev/logout", nil))
	if c := out.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Fatalf("logout must clear the cookie: %v", c)
	}
}
