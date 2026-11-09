package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestGrantsAreReplacedPerUserAndConnectionAndExpire(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	c, err := q.UpsertConnection(ctx, store.Connection{Provider: "github", ExternalID: "1", AccountLogin: "alice", CreatedBy: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	g := func(id, name string, exp time.Time) store.RepoGrant {
		return store.RepoGrant{RepoExternalID: id, RepoFullName: name, DefaultBranch: "main", GitHubUserID: "42", GitHubLogin: "bob", ExpiresAt: exp}
	}
	live := time.Now().Add(10 * time.Minute)
	if err := q.ReplaceGrants(ctx, "u1", c.ID, []store.RepoGrant{g("1", "alice/b", live), g("2", "alice/a", live)}); err != nil {
		t.Fatal(err)
	}
	if err := q.ReplaceGrants(ctx, "u2", c.ID, []store.RepoGrant{g("1", "alice/b", live)}); err != nil {
		t.Fatal(err)
	}
	list, _ := q.ListGrants(ctx, "u1")
	if len(list) != 2 || list[0].RepoFullName != "alice/a" {
		t.Fatalf("list = %+v, want two, sorted by name", list)
	}

	// A new authorization replaces the old list: repo 2 is gone, and u2 is untouched.
	if err := q.ReplaceGrants(ctx, "u1", c.ID, []store.RepoGrant{g("1", "alice/b", live)}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetGrant(ctx, "u1", c.ID, "2"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("a repository that lost access must lose its grant, err = %v", err)
	}
	if _, err := q.GetGrant(ctx, "u2", c.ID, "1"); err != nil {
		t.Fatalf("another user's grant must stay: %v", err)
	}

	// An expired grant is invisible and can be purged.
	if err := q.ReplaceGrants(ctx, "u3", c.ID, []store.RepoGrant{g("9", "alice/old", time.Now().Add(-time.Minute))}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetGrant(ctx, "u3", c.ID, "9"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("an expired grant must not be returned, err = %v", err)
	}
	if n, err := q.PurgeExpiredGrants(ctx); err != nil || n != 1 {
		t.Fatalf("purged %d, %v; want 1", n, err)
	}

	// A grant is used up once.
	if err := q.DeleteGrant(ctx, "u1", c.ID, "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetGrant(ctx, "u1", c.ID, "1"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("a deleted grant must be gone")
	}
}
