package api_test

import (
	"net/http"
	"testing"
)

func TestProjectGenerationSettingsAreReadByMembersAndChangedByOwnersAndCoOwners(t *testing.T) {
	e, id := teamEnv(t)
	url := "/api/projects/" + id + "/generation"
	good := map[string]any{
		"layout":    map[string]any{"mode": "single", "folder": "terraform", "file": "main.tf"},
		"variables": []map[string]any{{"name": "org", "description": "Organisation", "values": map[string]string{"dev": "aic"}}},
		"starters":  []map[string]string{{"filename": "providers.tf", "content": "provider \"azurerm\" {\n  features {}\n}\n"}},
	}

	// Nothing saved yet: members read an empty document.
	if code, body, raw := e.do("bob", "user", "GET", url, nil); code != http.StatusOK || body["variables"] != nil {
		t.Fatalf("default read: %d %s", code, raw)
	}
	// Someone outside the project sees nothing.
	if code, _, _ := e.do("mallory", "user", "GET", url, nil); code == http.StatusOK {
		t.Fatal("an outsider must not read a project's settings")
	}
	// A contributor cannot change them; a co-owner and the owner can.
	if code, _, _ := e.do("bob", "user", "PUT", url, good); code != http.StatusForbidden {
		t.Fatalf("contributor PUT = %d, want 403", code)
	}
	if code, _, raw := e.do("carol", "user", "PUT", url, good); code != http.StatusOK {
		t.Fatalf("co-owner PUT = %d %s", code, raw)
	}
	code, body, raw := e.do("bob", "user", "GET", url, nil)
	vars, _ := body["variables"].([]any)
	if code != http.StatusOK || len(vars) != 1 {
		t.Fatalf("read back: %d %s", code, raw)
	}

	// Invalid documents are refused whole, with every problem listed, and change nothing.
	bad := map[string]any{
		"layout":    map[string]any{"mode": "mystery"},
		"variables": []map[string]any{{"name": "Bad Name"}},
	}
	code, body, raw = e.do("alice", "user", "PUT", url, bad)
	problems, _ := body["problems"].([]any)
	if code != http.StatusUnprocessableEntity || len(problems) < 2 {
		t.Fatalf("invalid PUT: %d %s", code, raw)
	}
	if _, body, _ := e.do("bob", "user", "GET", url, nil); len(body["variables"].([]any)) != 1 {
		t.Fatal("a refused document must not change the stored one")
	}
	// An unknown key is refused too (a typo must not be silently dropped).
	if code, _, _ := e.do("alice", "user", "PUT", url, map[string]any{"layuot": map[string]any{}}); code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown key = %d, want 422", code)
	}
}
