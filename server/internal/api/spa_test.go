package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func spaDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "<html>app</html>", "assets/app.js": "console.log(1)", "logo.svg": "<svg/>"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func get(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestSPAServesTheAppAndItsFiles(t *testing.T) {
	h := SPA(spaDir(t))

	if rec := get(h, "GET", "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("/ = %d %q", rec.Code, rec.Body.String())
	}
	// A screen of the app, reached by a refresh or a link, is the app.
	if rec := get(h, "GET", "/projects/abc"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("/projects/abc = %d", rec.Code)
	}
	rec := get(h, "GET", "/assets/app.js")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("asset = %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("a hashed asset should be cached for good, got %q", rec.Header().Get("Cache-Control"))
	}
	if csp := get(h, "GET", "/").Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("the app needs its own policy, got %q", csp)
	}
}

func TestSPAKeepsTheAPIAndOtherFilesHonest(t *testing.T) {
	h := SPA(spaDir(t))
	for _, p := range []string{"/api/nothing", "/api", "/auth/nothing", "/assets/missing.js", "/missing.png"} {
		if rec := get(h, "GET", p); rec.Code != 404 {
			t.Errorf("GET %s = %d, want 404 (not the app)", p, rec.Code)
		}
	}
	if rec := get(h, "POST", "/"); rec.Code != 404 {
		t.Errorf("POST / = %d, want 404", rec.Code)
	}
}

func TestSPAStaysInsideItsFolder(t *testing.T) {
	dir := spaDir(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := SPA(dir)
	for _, p := range []string{"/../secret.txt", "/%2e%2e/secret.txt", "/assets/../../secret.txt"} {
		rec := get(h, "GET", p)
		if strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("GET %s leaked a file outside the folder", p)
		}
	}
}
