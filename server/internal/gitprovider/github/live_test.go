package github_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
)

// TestLiveGitHub talks to real GitHub. It runs only when the App settings and a target
// repository are in the environment, so CI and normal runs skip it:
//
//	TF_LIVE_GITHUB_REPO=owner/name  TF_LIVE_GITHUB_INSTALLATION_ID=123
//	TF_GITHUB_APP_ID=...  TF_GITHUB_APP_PRIVATE_KEY_FILE=path (or _B64)
//
// It reads the repository and creates ONE new branch named terraflow/live-smoke-<time>;
// it never touches an existing branch. Delete the branch afterwards.
func TestLiveGitHub(t *testing.T) {
	full := os.Getenv("TF_LIVE_GITHUB_REPO")
	instRaw := os.Getenv("TF_LIVE_GITHUB_INSTALLATION_ID")
	appID := os.Getenv("TF_GITHUB_APP_ID")
	if full == "" || instRaw == "" || appID == "" {
		t.Skip("live GitHub settings are not set")
	}
	inst, err := strconv.ParseInt(instRaw, 10, 64)
	if err != nil {
		t.Fatal("TF_LIVE_GITHUB_INSTALLATION_ID must be a number")
	}
	key, err := github.LoadPrivateKey(os.Getenv("TF_GITHUB_APP_PRIVATE_KEY_B64"), os.Getenv("TF_GITHUB_APP_PRIVATE_KEY_FILE"), os.ReadFile)
	if err != nil {
		t.Fatalf("private key: %v", err)
	}
	parts := strings.SplitN(full, "/", 2)
	if len(parts) != 2 {
		t.Fatal("TF_LIVE_GITHUB_REPO must be owner/name")
	}
	app := &github.App{ID: appID, Key: key}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := app.GetInstallation(ctx, inst); err != nil {
		t.Fatalf("installation check: %v", err)
	}
	src := &github.Source{App: app, InstallationID: inst, Owner: parts[0], Repo: parts[1], Path: os.Getenv("TF_LIVE_GITHUB_FOLDER")}
	snap, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.Commit == "" {
		t.Fatal("snapshot has no commit")
	}
	name := "terraflow/live-smoke-" + strconv.FormatInt(time.Now().Unix(), 10)
	res, err := src.CreateBranch(ctx, gitprovider.BranchRequest{
		Branch: name, Base: snap.Commit, Message: "TerraFlow live smoke test",
		Files: []gitprovider.FileChange{{Path: "terraflow-smoke/" + strconv.FormatInt(time.Now().Unix(), 10) + ".tf", Content: "# synthetic smoke test\n"}},
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	t.Logf("created branch %s at %s (delete it when done)", name, res.Commit)
}
