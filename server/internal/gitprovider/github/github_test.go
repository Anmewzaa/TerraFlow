package github_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github/githubtest"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/gitprovidertest"
)

var (
	keyOnce sync.Once
	testKey *rsa.PrivateKey
	keyErr  error
)

func key(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	keyOnce.Do(func() { testKey, keyErr = rsa.GenerateKey(rand.Reader, 2048) })
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	return testKey
}

var clock = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

const (
	appID  = "123456"
	instID = int64(777)
	owner  = "alice"
	repo   = "infra"
	full   = owner + "/" + repo
	folder = "iac/"
)

type env struct {
	fake *githubtest.Fake
	app  *github.App
	src  *github.Source
}

func newEnv(t *testing.T) *env {
	t.Helper()
	k := key(t)
	fake := githubtest.New(appID, &k.PublicKey, func() time.Time { return clock })
	t.Cleanup(fake.Close)
	fake.AddRepo(full, folder, gitprovidertest.Start)
	fake.AddInstallation(instID, owner, full)
	app := &github.App{ID: appID, Key: k, APIBase: fake.URL, WebBase: "https://github.example", Now: func() time.Time { return clock }}
	return &env{fake: fake, app: app, src: &github.Source{App: app, InstallationID: instID, Owner: owner, Repo: repo, Branch: "main", Path: "iac"}}
}

func fixture(t *testing.T) gitprovidertest.Fixture {
	e := newEnv(t)
	return gitprovidertest.Fixture{
		Source:  e.src,
		Branch:  func(name string) map[string]string { return e.fake.Branch(full, folder, name) },
		Advance: func(c map[string]string) { e.fake.Push(full, folder, c) },
	}
}

func TestGitHubConforms(t *testing.T)         { gitprovidertest.Run(t, fixture) }
func TestGitHubBranchesFromBase(t *testing.T) { gitprovidertest.RunBase(t, fixture) }

func TestJWTIsASignedShortLivedAppToken(t *testing.T) {
	e := newEnv(t)
	tok, err := e.app.JWT()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("parts: %d", len(parts))
	}
	hdr, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if string(hdr) != `{"alg":"RS256","typ":"JWT"}` {
		t.Errorf("header %s", hdr)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var c struct {
		Iat, Exp int64
		Iss      string
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	if c.Iss != appID {
		t.Errorf("iss %q", c.Iss)
	}
	if c.Iat != clock.Add(-time.Minute).Unix() {
		t.Errorf("iat must start a minute in the past: %d", c.Iat)
	}
	if lifetime := c.Exp - c.Iat; lifetime > 10*60 {
		t.Errorf("GitHub allows ten minutes at most, got %d seconds", lifetime)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key(t).PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		t.Errorf("signature: %v", err)
	}
}

func TestInstallationTokenIsLimitedToOneRepository(t *testing.T) {
	e := newEnv(t)
	if _, err := e.src.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	repos, perms := e.fake.TokenFacts()
	if len(repos) != 1 || repos[0] != full {
		t.Errorf("a token must be limited to the one repository: %v", repos)
	}
	if perms[0]["contents"] != "write" || perms[0]["metadata"] != "read" || len(perms[0]) != 2 {
		t.Errorf("permissions must be exactly contents:write and metadata:read: %v", perms[0])
	}
}

func TestATokenIsMadePerOperationAndNeverReused(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.src.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := e.src.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := e.src.CreateBranch(ctx, gitprovider.BranchRequest{Branch: "b", Message: "m", Files: []gitprovider.FileChange{{Path: "n/a.tf", Content: "x"}}}); err != nil {
		t.Fatal(err)
	}
	if e.fake.TokensMinted != 3 {
		t.Errorf("three operations must make three tokens, got %d", e.fake.TokensMinted)
	}
}

func TestAnExpiredTokenIsReplacedOnceAndTheCallRetried(t *testing.T) {
	e := newEnv(t)
	e.fake.Reject401Once = true // the first repository call after the token was made
	if _, err := e.src.Snapshot(context.Background()); err != nil {
		t.Fatalf("one 401 must be survived: %v", err)
	}
	if e.fake.TokensMinted != 2 {
		t.Errorf("expected a second token, got %d", e.fake.TokensMinted)
	}
}

func TestAWrongKeyIsReportedAsARefusedCredential(t *testing.T) {
	e := newEnv(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e.app.Key = other
	_, err = e.src.Snapshot(context.Background())
	if !errors.Is(err, gitprovider.ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

func TestAnExpiredAppTokenIsRefused(t *testing.T) {
	e := newEnv(t)
	// Moving the App's clock a day back makes the JWT it signs too old for the fake.
	e.app.Now = func() time.Time { return clock.Add(-24 * time.Hour) }
	if _, err := e.src.Snapshot(context.Background()); !errors.Is(err, gitprovider.ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

func TestRateLimitAndMissingThings(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	e.fake.RateLimit = true
	if _, err := e.src.Snapshot(ctx); !errors.Is(err, gitprovider.ErrRateLimited) {
		t.Errorf("rate limit: %v", err)
	}

	missing := *e.src
	missing.Repo = "nope"
	if _, err := missing.Snapshot(ctx); err == nil {
		t.Error("a repository the installation cannot use must fail")
	} else if !errors.Is(err, gitprovider.ErrUnauthorized) && !errors.Is(err, gitprovider.ErrNotFound) {
		t.Errorf("repository missing: %v", err)
	}

	noBranch := *e.src
	noBranch.Branch = "nope"
	if _, err := noBranch.Snapshot(ctx); !errors.Is(err, gitprovider.ErrNotFound) {
		t.Errorf("branch missing: %v", err)
	}

	noInst := *e.src
	noInst.InstallationID = 1
	if _, err := noInst.Snapshot(ctx); !errors.Is(err, gitprovider.ErrNotFound) {
		t.Errorf("installation missing: %v", err)
	}
}

func TestSnapshotReadsOnlyTerraformInsideTheFolder(t *testing.T) {
	e := newEnv(t)
	e.fake.Push(full, "", map[string]string{
		"iac/common/README.md":      "# not terraform\n",
		"iac/common/big.tf":         strings.Repeat("# x\n", 100000), // over the size limit
		"iac/common/env/dev.tfvars": "project = \"p\"\n",
		"other/outside.tf":          "# a different folder\n",
		"README.md":                 "# repo\n",
	})
	snap, err := e.src.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	fs.WalkDir(snap.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})
	want := "common/env/dev.tfvars common/main.tf common/rg.tf vnet/vnet.tf"
	if strings.Join(got, " ") != want {
		t.Errorf("files:\n got %v\nwant %s", got, want)
	}
}

func TestSnapshotWithoutAFolderReadsTheRepositoryRoot(t *testing.T) {
	e := newEnv(t)
	root := *e.src
	root.Path = ""
	snap, err := root.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b, err := fs.ReadFile(snap.FS, "iac/common/rg.tf"); err != nil || len(b) == 0 {
		t.Errorf("root snapshot: %v", err)
	}
}

func TestATooLargeRepositoryIsAnErrorNotAPartialRead(t *testing.T) {
	e := newEnv(t)
	e.fake.TruncateTrees = true
	if _, err := e.src.Snapshot(context.Background()); !errors.Is(err, github.ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateBranchBuildsOneCommitOnTheBaseAndLeavesTheDefaultBranchAlone(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	snap, _ := e.src.Snapshot(ctx)
	res, err := e.src.CreateBranch(ctx, gitprovider.BranchRequest{
		Base: snap.Commit, Branch: "terraflow/p/dev-1", Message: "add storage",
		Files: []gitprovider.FileChange{{Path: "storage/main.tf", Content: "module \"sa\" {}\n"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Repo != full || res.URL != "https://github.example/alice/infra/tree/terraflow/p/dev-1" {
		t.Errorf("result: %+v", res)
	}
	if parents := e.fake.Parents(full, res.Commit); len(parents) != 1 || parents[0] != snap.Commit {
		t.Errorf("the commit must have the planned base as its only parent: %v", parents)
	}
	if e.fake.Head(full, "main") != snap.Commit {
		t.Error("the default branch must not move")
	}
	// The file is written below the project folder of the repository, not at its root.
	if got := e.fake.Branch(full, "", "terraflow/p/dev-1"); got["iac/storage/main.tf"] == "" || got["storage/main.tf"] != "" {
		t.Errorf("path prefix: %v", got)
	}
}

func TestBadInputIsRefusedBeforeAnyRequest(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	good := []gitprovider.FileChange{{Path: "a/b.tf", Content: "x"}}
	for name, req := range map[string]gitprovider.BranchRequest{
		"empty branch":      {Branch: "", Message: "m", Files: good},
		"branch with space": {Branch: "a b", Message: "m", Files: good},
		"branch dotdot":     {Branch: "a..b", Message: "m", Files: good},
		"branch with colon": {Branch: "a:b", Message: "m", Files: good},
		"branch .lock":      {Branch: "x.lock", Message: "m", Files: good},
		"no files":          {Branch: "ok", Message: "m"},
		"unsafe file":       {Branch: "ok", Message: "m", Files: []gitprovider.FileChange{{Path: "../x.tf", Content: "x"}}},
	} {
		if _, err := e.src.CreateBranch(ctx, req); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	for name, bad := range map[string]github.Source{
		"owner with slash": {App: e.app, InstallationID: instID, Owner: "a/b", Repo: repo},
		"repo dotdot":      {App: e.app, InstallationID: instID, Owner: owner, Repo: ".."},
		"empty owner":      {App: e.app, InstallationID: instID, Owner: "", Repo: repo},
		"folder escape":    {App: e.app, InstallationID: instID, Owner: owner, Repo: repo, Path: "../x"},
	} {
		src := bad
		if _, err := src.Snapshot(ctx); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if len(e.fake.Requests) != 0 {
		t.Errorf("no request may be made for bad input, saw %v", e.fake.Requests)
	}
}

// Nothing secret may leave through an error, a log line or a printed value.
func TestNoSecretLeaksInErrorsOrPrintedValues(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	der := x509.MarshalPKCS1PrivateKey(key(t))
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
	secrets := []string{base64.StdEncoding.EncodeToString(der)[:40], base64.StdEncoding.EncodeToString(key(t).D.Bytes())[:30], "ghs_fake"}

	var texts []string
	// A successful run, then every kind of failure.
	tok, _ := e.app.InstallationToken(ctx, instID, repo)
	texts = append(texts, fmt.Sprint(tok), fmt.Sprintf("%v %+v %#v %s", tok, tok, tok, tok), fmt.Sprint(e.app), fmt.Sprintf("%+v %#v", e.app, e.app))
	e.fake.RateLimit = true
	_, err := e.src.Snapshot(ctx)
	texts = append(texts, fmt.Sprint(err))
	bad := *e.src
	bad.Repo = "nope"
	_, err = bad.Snapshot(ctx)
	texts = append(texts, fmt.Sprint(err))
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	wrong := &github.App{ID: appID, Key: other, APIBase: e.fake.URL, Now: e.app.Now}
	_, err = (&github.Source{App: wrong, InstallationID: instID, Owner: owner, Repo: repo}).Snapshot(ctx)
	texts = append(texts, fmt.Sprint(err))
	_, err = github.ParsePrivateKey([]byte("-----BEGIN RSA PRIVATE KEY-----\n" + secrets[0] + "\n-----END RSA PRIVATE KEY-----"))
	texts = append(texts, fmt.Sprint(err))

	for _, text := range texts {
		for _, s := range secrets {
			if strings.Contains(text, s) {
				t.Errorf("a secret appears in %q", text)
			}
		}
		if strings.Contains(text, "PRIVATE KEY-----\n") || strings.Contains(text, "Bearer") {
			t.Errorf("key or header text in %q", text)
		}
	}
	_ = pemText
}

func TestParseAndLoadPrivateKey(t *testing.T) {
	k := key(t)
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	der8, _ := x509.MarshalPKCS8PrivateKey(k)
	pkcs8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der8})
	for name, b := range map[string][]byte{"PKCS#1": pkcs1, "PKCS#8": pkcs8} {
		got, err := github.ParsePrivateKey(b)
		if err != nil || got.N.Cmp(k.N) != 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, b := range map[string][]byte{"empty": nil, "text": []byte("hello"), "wrong block": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")})} {
		if _, err := github.ParsePrivateKey(b); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}

	read := func(string) ([]byte, error) { return pkcs1, nil }
	b64 := base64.StdEncoding.EncodeToString(pkcs1)
	if _, err := github.LoadPrivateKey(b64, "", read); err != nil {
		t.Errorf("base64: %v", err)
	}
	if _, err := github.LoadPrivateKey("", "/some/file.pem", read); err != nil {
		t.Errorf("file: %v", err)
	}
	if _, err := github.LoadPrivateKey(b64, "/some/file.pem", read); err == nil {
		t.Error("both set must be refused")
	}
	if _, err := github.LoadPrivateKey("", "", read); err == nil {
		t.Error("none set must be refused")
	}
	if _, err := github.LoadPrivateKey("%%%not base64%%%", "", read); err == nil {
		t.Error("bad base64 must be refused")
	}
	_, err := github.LoadPrivateKey("", "/missing.pem", func(p string) ([]byte, error) { return os.ReadFile(p) })
	if err == nil {
		t.Error("a missing file must be an error")
	}
}

func TestGetInstallationVerifiesWithTheAppIdentity(t *testing.T) {
	e := newEnv(t)
	in, err := e.app.GetInstallation(context.Background(), instID)
	if err != nil || in.ID != instID || in.Account.Login != owner {
		t.Fatalf("installation: %+v %v", in, err)
	}
	if _, err := e.app.GetInstallation(context.Background(), 999); !errors.Is(err, gitprovider.ErrNotFound) {
		t.Errorf("unknown installation: %v", err)
	}
}

func TestOutlineListsTerraformFilesWithoutDownloadingAny(t *testing.T) {
	e := newEnv(t)
	root := *e.src
	root.Path = ""
	tree, err := root.Outline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})
	if len(got) == 0 {
		t.Fatal("no Terraform files listed")
	}
	for _, p := range got {
		if !strings.HasSuffix(p, ".tf") && !strings.HasSuffix(p, ".tfvars") {
			t.Errorf("%s is not Terraform", p)
		}
		if b, _ := fs.ReadFile(tree, p); len(b) != 0 {
			t.Errorf("%s: an outline must not carry file contents", p)
		}
	}
	for _, r := range e.fake.Requests {
		if strings.HasPrefix(r, "GET ") && strings.Contains(r, "/git/blobs/") {
			t.Errorf("outline downloaded a file: %s", r)
		}
	}
}

func TestAnEmptyRepositoryHasItsOwnClearError(t *testing.T) {
	e := newEnv(t)
	e.fake.AddEmptyRepo("alice/blank")
	e.fake.AddInstallation(instID, owner, full, "alice/blank")
	src := *e.src
	src.Repo, src.Path = "blank", ""
	for name, call := range map[string]func() error{
		"snapshot": func() error { _, err := src.Snapshot(context.Background()); return err },
		"outline":  func() error { _, err := src.Outline(context.Background()); return err },
	} {
		if err := call(); !errors.Is(err, gitprovider.ErrEmptyRepo) {
			t.Errorf("%s: got %v, want ErrEmptyRepo", name, err)
		}
	}
}

func TestInitializeCreatesTheFirstCommitOfAnEmptyRepositoryAndNothingElse(t *testing.T) {
	e := newEnv(t)
	e.fake.AddEmptyRepo("alice/blank")
	e.fake.AddInstallation(instID, owner, full, "alice/blank")
	blank := *e.src
	blank.Repo, blank.Path, blank.Branch = "blank", "", ""
	sha, err := blank.Initialize(context.Background())
	if err != nil || sha == "" {
		t.Fatalf("initialize: %q %v", sha, err)
	}
	// The repository now has a default branch with exactly one file.
	if e.fake.Head("alice/blank", "main") != sha {
		t.Fatalf("the default branch does not point at the new commit")
	}
	if _, err := blank.Outline(context.Background()); err != nil {
		t.Fatalf("an initialized repository must be readable: %v", err)
	}
	// A second call, and a repository that already had commits, are refused unchanged.
	if _, err := blank.Initialize(context.Background()); !errors.Is(err, gitprovider.ErrNotEmpty) {
		t.Fatalf("second initialize: %v, want ErrNotEmpty", err)
	}
	if _, err := e.src.Initialize(context.Background()); !errors.Is(err, gitprovider.ErrNotEmpty) {
		t.Fatalf("non-empty repository: %v, want ErrNotEmpty", err)
	}
}
