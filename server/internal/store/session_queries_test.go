package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func newSession(id string, exp time.Time) store.Session {
	return store.Session{ID: id, UserOID: "u-" + id, Email: id + "@x.test", Name: id,
		Roles: []string{"devops"}, GroupIDs: []string{}, ExpiresAt: exp}
}

func TestSessionIDIsStoredHashed(t *testing.T) {
	q, dsn := storetest.New(t)
	ctx := context.Background()
	secret := "this-is-the-cookie-value"
	if err := q.CreateSession(ctx, newSession(secret, time.Now().Add(time.Hour))); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var stored string
	if err := conn.QueryRow(ctx, `SELECT id FROM sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == secret || strings.Contains(stored, secret) || len(stored) != 64 {
		t.Fatalf("the table must hold a SHA-256 hex digest, not the cookie value: %q", stored)
	}

	// Looking it up with the cookie value works; looking it up with the stored
	// digest (what a leaked backup would give) does not.
	got, err := q.GetSession(ctx, secret)
	if err != nil || got.UserOID != "u-"+secret || got.ID != secret {
		t.Fatalf("lookup by cookie value: %+v %v", got, err)
	}
	if _, err := q.GetSession(ctx, stored); err == nil {
		t.Fatal("the stored digest must not work as a cookie")
	}
}

func TestSessionDeleteAndExpiry(t *testing.T) {
	q, dsn := storetest.New(t)
	ctx := context.Background()
	if err := q.CreateSession(ctx, newSession("old", time.Now().Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "old"); err == nil {
		t.Fatal("an expired session must not be returned")
	}
	if err := q.CreateSession(ctx, newSession("live", time.Now().Add(time.Hour))); err != nil {
		t.Fatal(err)
	}

	// Creating a session purged the expired row.
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows after the purge: %d %v", n, err)
	}

	if err := q.DeleteSession(ctx, "live"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "live"); err == nil {
		t.Fatal("a deleted session must not be returned")
	}
	// Deleting something that is not there is harmless.
	if err := q.DeleteSession(ctx, "nope"); err != nil {
		t.Fatal(err)
	}
}
