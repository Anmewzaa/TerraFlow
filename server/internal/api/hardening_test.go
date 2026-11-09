package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var ok200 = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

func run(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSecurityHeaders(t *testing.T) {
	for _, hsts := range []bool{false, true} {
		rec := run(SecurityHeaders(hsts)(ok200), httptest.NewRequest("GET", "/api/me", nil))
		want := map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "same-origin",
			"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
			"Cache-Control":           "no-store",
		}
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("hsts=%v %s = %q, want %q", hsts, k, got, v)
			}
		}
		if got := rec.Header().Get("Strict-Transport-Security"); (got != "") != hsts {
			t.Errorf("hsts=%v but header is %q", hsts, got)
		}
	}
}

func TestCORS(t *testing.T) {
	h := CORS([]string{"https://app.example/", " https://other.example "})(ok200)
	tests := []struct {
		name, origin string
		allowed      bool
	}{
		{"listed origin", "https://app.example", true},
		{"listed after trimming", "https://other.example", true},
		{"unlisted origin", "https://evil.example", false},
		{"look-alike prefix", "https://app.example.evil.example", false},
		{"look-alike suffix", "https://evilapp.example", false},
		{"scheme differs", "http://app.example", false},
		{"null origin", "null", false},
		{"no origin", "", false},
	}
	for _, tc := range tests {
		req := httptest.NewRequest("GET", "/api/me", nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := run(h, req)
		got := rec.Header().Get("Access-Control-Allow-Origin")
		if tc.allowed && (got != tc.origin || rec.Header().Get("Access-Control-Allow-Credentials") != "true") {
			t.Errorf("%s: allow-origin %q", tc.name, got)
		}
		if !tc.allowed && (got != "" || rec.Header().Get("Access-Control-Allow-Credentials") != "") {
			t.Errorf("%s: must not be allowed, got %q", tc.name, got)
		}
		if got := rec.Header().Values("Vary"); len(got) == 0 {
			t.Errorf("%s: responses that depend on Origin must say Vary: Origin", tc.name)
		}
	}
	// Never a wildcard, even with no list at all.
	rec := run(CORS(nil)(ok200), func() *http.Request {
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.Header.Set("Origin", "https://evil.example")
		return r
	}())
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("with no allowed origins nothing may be allowed")
	}
	// A preflight is answered without running the handler.
	pre := httptest.NewRequest("OPTIONS", "/api/projects", nil)
	pre.Header.Set("Origin", "https://app.example")
	if rec := run(h, pre); rec.Code != http.StatusNoContent {
		t.Errorf("preflight: %d", rec.Code)
	}
}

func TestSameOriginWrites(t *testing.T) {
	h := SameOriginWrites([]string{"https://app.example"})(ok200)
	tests := []struct {
		name, method, host string
		headers            map[string]string
		want               int
	}{
		{"read from another site is not blocked here", "GET", "tf.example", map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, 200},
		{"write from the same origin", "POST", "tf.example", map[string]string{"Origin": "http://tf.example", "Sec-Fetch-Site": "same-origin"}, 200},
		{"write from another origin", "POST", "tf.example", map[string]string{"Origin": "https://evil.example"}, 403},
		{"write marked cross-site", "DELETE", "tf.example", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"write marked same-site (a sibling subdomain)", "PUT", "tf.example", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"write from a listed origin", "POST", "tf.example", map[string]string{"Origin": "https://app.example", "Sec-Fetch-Site": "cross-site"}, 200},
		{"host that merely contains ours", "POST", "tf.example", map[string]string{"Origin": "http://tf.example.evil.example"}, 403},
		{"origin with a different port", "POST", "tf.example", map[string]string{"Origin": "http://tf.example:8080"}, 403},
		{"unparsable origin", "POST", "tf.example", map[string]string{"Origin": "http://%zz"}, 403},
		{"null origin", "POST", "tf.example", map[string]string{"Origin": "null"}, 403},
		{"script without browser headers", "POST", "tf.example", nil, 200},
		{"behind a proxy that rewrites Host", "POST", "10.0.0.5:8080", map[string]string{"Origin": "https://tf.example", "X-Forwarded-Host": "tf.example"}, 200},
		{"forwarded host that does not match", "POST", "10.0.0.5:8080", map[string]string{"Origin": "https://evil.example", "X-Forwarded-Host": "tf.example"}, 403},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, "/api/projects", nil)
		req.Host = tc.host
		for k, v := range tc.headers {
			req.Header.Set(k, v)
		}
		if rec := run(h, req); rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, rec.Code, tc.want)
		}
	}
}

func TestRateLimit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := RateLimit(3, 10*time.Second, func() time.Time { return now })(ok200)
	call := func(addr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/auth/login", nil)
		req.RemoteAddr = addr
		return run(h, req)
	}
	for i := 0; i < 3; i++ {
		if rec := call("1.2.3.4:5000"); rec.Code != 200 {
			t.Fatalf("request %d inside the burst: %d", i+1, rec.Code)
		}
	}
	rec := call("1.2.3.4:5001") // another source port, same client
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("over the burst: %d %v", rec.Code, rec.Header())
	}
	if !strings.Contains(rec.Body.String(), "error") {
		t.Errorf("body: %s", rec.Body)
	}
	if rec := call("5.6.7.8:1"); rec.Code != 200 {
		t.Errorf("another client is not affected: %d", rec.Code)
	}
	now = now.Add(9 * time.Second)
	if rec := call("1.2.3.4:5000"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a token is not back after 9s: %d", rec.Code)
	}
	now = now.Add(2 * time.Second)
	if rec := call("1.2.3.4:5000"); rec.Code != 200 {
		t.Errorf("one token is back after 11s: %d", rec.Code)
	}
	if rec := call("1.2.3.4:5000"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("and only one: %d", rec.Code)
	}
	now = now.Add(time.Hour) // a long pause refills to the burst, not beyond
	for i := 0; i < 3; i++ {
		if rec := call("1.2.3.4:5000"); rec.Code != 200 {
			t.Fatalf("after a pause, request %d: %d", i+1, rec.Code)
		}
	}
	if rec := call("1.2.3.4:5000"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("tokens must cap at the burst: %d", rec.Code)
	}
}

func TestSharedRateLimitUsesTheSharedCountAndFallsBackWhenItFails(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	allow := true
	var failing bool
	take := func(_ context.Context, key string, burst int, every time.Duration, _ time.Time) (bool, error) {
		if failing {
			return false, errors.New("database down")
		}
		return allow, nil
	}
	h := SharedRateLimit(1, time.Minute, func() time.Time { return now }, take)(ok200)
	call := func() int {
		req := httptest.NewRequest("GET", "/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1"
		return run(h, req).Code
	}
	if call() != 200 {
		t.Fatal("the shared count allows")
	}
	allow = false
	if call() != http.StatusTooManyRequests {
		t.Fatal("the shared count refuses")
	}
	failing = true // the database is down: the in-memory limiter judges (burst 1)
	if call() != 200 || call() != http.StatusTooManyRequests {
		t.Fatal("a failing database must fall back to the in-memory limiter")
	}
}
