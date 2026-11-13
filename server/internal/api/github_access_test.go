package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func (m *memGH) GetProjectRepo(_ context.Context, projectID string) (store.ProjectRepo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.repos[projectID]
	if !ok {
		return store.ProjectRepo{}, errors.New("no link")
	}
	return r, nil
}

func (m *memGH) audited(action string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, a := range m.audit {
		if a == action {
			n++
		}
	}
	return n
}

// failing makes every request to a path containing part fail, as if GitHub were down.
type failing struct {
	part string
	next http.RoundTripper
}

func (f failing) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.Contains(r.URL.Path, f.part) {
		return nil, errors.New("connection refused")
	}
	return f.next.RoundTrip(r)
}

func TestAnOwnerWhoLosesRepositoryAccessStopsTheProjectUntilTheyVerifyAgain(t *testing.T) {
	e := newGHEnv(t)
	e.authorize(t, "u1", e.fake.AddUser(42, "alice", []int64{7}, "alice/infra", "alice/docs"))
	w := e.do("u1", "POST", "/api/projects/from-repo", createBody(e, "u1", "Platform", "alice/infra", "iac"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	var p store.Project
	json.Unmarshal(w.Body.Bytes(), &p)
	repos := ProjectRepos(nil, e.api.App, e.st)
	open := func() error { _, err := repos(context.Background(), p); return err }
	status := func() string { return e.st.repos[p.ID].AccessStatus }
	if err := open(); err != nil {
		t.Fatalf("a fresh project opens: %v", err)
	}

	// Someone else connecting GitHub says nothing about this project.
	e.authorize(t, "u2", e.fake.AddUser(43, "bob", []int64{7}, "alice/docs"))
	if status() == store.RepoAccessNeedsAttention {
		t.Fatal("another person's authorization must not change the owner's project")
	}

	// GitHub cannot be asked: not knowing is not losing access.
	e.fake.SetUserRepos(42, "alice/docs")
	e.api.App.HTTP = &http.Client{Transport: failing{part: "/repositories", next: http.DefaultTransport}}
	e.authorize(t, "u1", e.fake.NewCode(42))
	if status() == store.RepoAccessNeedsAttention || open() != nil {
		t.Fatalf("an unreachable GitHub must change nothing: %q %v", status(), open())
	}
	e.api.App.HTTP = nil

	// The owner proves who they are again and GitHub no longer lists the repository.
	e.authorize(t, "u1", e.fake.NewCode(42))
	if status() != store.RepoAccessNeedsAttention {
		t.Fatalf("status = %q, want needs_attention", status())
	}
	err := open()
	if !errors.Is(err, gitprovider.ErrUnauthorized) || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("the project must stop with a reason: %v", err)
	}
	if providerStatus(err) != http.StatusConflict {
		t.Fatalf("status code = %d", providerStatus(err))
	}
	if e.st.audited("github.access_lost") != 1 {
		t.Fatalf("audit: %v", e.st.audit)
	}
	// Checking again while nothing changed is not audited again.
	e.authorize(t, "u1", e.fake.NewCode(42))
	if e.st.audited("github.access_lost") != 1 {
		t.Fatalf("audited twice: %v", e.st.audit)
	}

	// Access is back on GitHub; the next proof clears it.
	e.fake.SetUserRepos(42, "alice/infra", "alice/docs")
	e.authorize(t, "u1", e.fake.NewCode(42))
	if status() != store.RepoAccessOK || open() != nil {
		t.Fatalf("after regaining access: %q %v", status(), open())
	}
	if e.st.audited("github.access_restored") != 1 {
		t.Fatalf("audit: %v", e.st.audit)
	}
}

func TestAnOwnerWhoCanNoLongerUseTheInstallationLosesAccess(t *testing.T) {
	e := newGHEnv(t)
	e.authorize(t, "u1", e.fake.AddUser(42, "alice", []int64{7}, "alice/infra"))
	w := e.do("u1", "POST", "/api/projects/from-repo", createBody(e, "u1", "Platform", "alice/infra", "iac"))
	var p store.Project
	json.Unmarshal(w.Body.Bytes(), &p)
	// The same person now reaches GitHub through no installation at all.
	e.authorize(t, "u1", e.fake.AddUser(42, "alice", nil))
	if got := e.st.repos[p.ID].AccessStatus; got != store.RepoAccessNeedsAttention {
		t.Fatalf("status = %q, want needs_attention", got)
	}
}
