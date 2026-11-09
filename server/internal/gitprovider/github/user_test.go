package github_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
)

// userEnv is newEnv with the App's client credentials and two repositories in the
// installation, only one of which the user can reach.
func userEnv(t *testing.T) (*env, string) {
	t.Helper()
	e := newEnv(t)
	e.app.APIBase, e.app.WebBase = e.fake.URL, e.fake.URL
	e.app.ClientID, e.app.ClientSecret = githubtest.ClientID, githubtest.ClientSecret
	e.fake.AddRepo(owner+"/secret", "", map[string]string{"main.tf": ""})
	e.fake.AddInstallation(instID, owner, full, owner+"/secret")
	code := e.fake.AddUser(42, "bob", []int64{instID}, full)
	return e, code
}

func TestExchangeCodeGivesAUserTokenOnce(t *testing.T) {
	e, code := userEnv(t)
	ctx := context.Background()
	tok, err := e.app.ExchangeCode(ctx, code, "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := e.app.GetUser(ctx, tok)
	if err != nil || u.ID != 42 || u.Login != "bob" {
		t.Fatalf("user = %+v, %v", u, err)
	}
	if _, err := e.app.ExchangeCode(ctx, code, ""); err == nil {
		t.Fatal("a code must work only once")
	}
}

func TestExchangeCodeRefusesBadCodeAndMissingSecret(t *testing.T) {
	e, _ := userEnv(t)
	if _, err := e.app.ExchangeCode(context.Background(), "nope", ""); err == nil {
		t.Fatal("a wrong code must be refused")
	}
	e.app.ClientSecret = ""
	if _, err := e.app.ExchangeCode(context.Background(), "x", ""); err == nil {
		t.Fatal("no client secret must be an error")
	}
}

func TestPickerListsOnlyWhatBothTheInstallationAndTheUserCanReach(t *testing.T) {
	e, code := userEnv(t)
	ctx := context.Background()
	tok, err := e.app.ExchangeCode(ctx, code, "")
	if err != nil {
		t.Fatal(err)
	}
	ins, err := e.app.UserInstallations(ctx, tok)
	if err != nil || len(ins) != 1 || ins[0].ID != instID {
		t.Fatalf("installations = %+v, %v", ins, err)
	}
	repos, err := e.app.UserInstallationRepos(ctx, tok, instID)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].FullName != full || repos[0].DefaultBranch != "main" {
		t.Fatalf("repos = %+v: the repository the user cannot reach must not be listed", repos)
	}
}

func TestAnInstallationTheUserCannotAccessIsNotFound(t *testing.T) {
	e, _ := userEnv(t)
	e.fake.AddInstallation(900, "carol", "carol/x")
	tok, err := e.app.ExchangeCode(context.Background(), e.fake.NewCode(42), "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.app.UserInstallationRepos(context.Background(), tok, 900)
	if !errors.Is(err, gitprovider.ErrNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestARevokedUserTokenIsRefused(t *testing.T) {
	e, code := userEnv(t)
	ctx := context.Background()
	tok, _ := e.app.ExchangeCode(ctx, code, "")
	e.fake.RevokeUserTokens()
	if _, err := e.app.GetUser(ctx, tok); !errors.Is(err, gitprovider.ErrUnauthorized) {
		t.Fatalf("err = %v, want unauthorized", err)
	}
}

func TestAuthorizeURLCarriesStateAndNoSecret(t *testing.T) {
	e, _ := userEnv(t)
	u := e.app.AuthorizeURL("st8", "http://localhost:8080/auth/github/callback")
	if !strings.Contains(u, "state=st8") || !strings.Contains(u, "client_id=") || strings.Contains(u, githubtest.ClientSecret) {
		t.Fatalf("url = %s", u)
	}
}

func TestUserTokenNeverAppearsInErrorsOrPrints(t *testing.T) {
	e, code := userEnv(t)
	ctx := context.Background()
	tok, _ := e.app.ExchangeCode(ctx, code, "")
	e.fake.RevokeUserTokens()
	_, err := e.app.GetUser(ctx, tok)
	for _, s := range []string{fmt.Sprint(tok), fmt.Sprintf("%+v", tok), fmt.Sprintf("%#v", tok), fmt.Sprint(err), fmt.Sprint(e.app)} {
		if strings.Contains(s, "ghu_") || strings.Contains(s, githubtest.ClientSecret) {
			t.Fatalf("a secret leaked: %q", s)
		}
	}
}
