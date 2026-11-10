package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoginProviders(t *testing.T) {
	for _, c := range []struct {
		name, list    string
		entra, github bool
		want          []string
		wantErr       string
	}{
		{"every configured one", "", true, true, []string{"entra", "github"}, ""},
		{"github only", "", false, true, []string{"github"}, ""},
		{"a chosen one", "github", true, true, []string{"github"}, ""},
		{"none configured", "", false, false, nil, "no sign-in provider"},
		{"unknown", "gitlab", true, true, nil, "unknown provider"},
		{"listed but not configured", "entra", false, true, nil, "not configured"},
	} {
		got, err := loginProviders(c.list, c.entra, c.github)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: err %v, want it to contain %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v %v, want %v", c.name, got, err, c.want)
		}
	}
}

func TestGitHubLoginRedirectUsesTheSameHostAsTheConnectCallback(t *testing.T) {
	for in, want := range map[string]string{
		"http://localhost:5173/api/github/callback":  "http://localhost:5173/auth/github/callback",
		"https://tf.example.com/api/github/callback": "https://tf.example.com/auth/github/callback",
		"":          "http://localhost:8080/auth/github/callback",
		"not a url": "http://localhost:8080/auth/github/callback",
	} {
		if got := githubLoginRedirect(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
