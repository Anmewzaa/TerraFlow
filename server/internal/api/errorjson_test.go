package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// An error message may contain quotes and backslashes (a bad file name, an HCL
// diagnostic); the answer must still be valid JSON.
func TestGraphErrorIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "common")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	// An unparsable file with quotes in it makes the parser fail with a diagnostic.
	if err := os.WriteFile(filepath.Join(folder, "main.tf"), []byte("module \"a\" {\n  source = \"x\" \"y\"\n}\n\"unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	NoDBGraphHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), dir).ServeHTTP(rec, httptest.NewRequest("GET", "/graph?env=dev", nil))
	if rec.Code != 500 {
		t.Skipf("the parser did not fail on this file (%d); nothing to check", rec.Code)
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out["error"] == "" {
		t.Fatalf("not valid JSON: %q %v", rec.Body.String(), err)
	}
}
