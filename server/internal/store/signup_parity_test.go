package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestKnownPersonKeepsTheStatusAnAdminSet(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	u := store.LoginUser{OID: "github:42", Provider: "github", Name: "alice"}
	if st, created, err := q.RecordLogin(ctx, u, store.UserPending, 0, false); err != nil || !created || st != store.UserPending {
		t.Fatalf("new = %s %v %v", st, created, err)
	}
	if st, _, _ := q.RecordLogin(ctx, u, store.UserActive, 0, false); st != store.UserPending {
		t.Fatalf("signing in again changed the status to %s", st)
	}
	if err := q.SetUserStatus(ctx, "github:42", store.UserRejected, "not on the team", "admin"); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := q.RecordLogin(ctx, u, store.UserPending, 0, false); st != store.UserRejected {
		t.Fatalf("a rejected person must stay rejected, got %s", st)
	}
}

func TestSuspendingEndsSessionsAtOnceParity(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:7", Provider: "github", Name: "bob"}, store.UserActive, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := q.CreateSession(ctx, store.Session{ID: "sid", UserOID: "github:7", Roles: []string{"developer"}, GroupIDs: []string{}, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "sid"); err != nil {
		t.Fatal(err)
	}
	if err := q.SetUserStatus(ctx, "github:7", store.UserSuspended, "left", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "sid"); err == nil {
		t.Fatal("a suspended person's session must end at once")
	}
}

func TestPendingCapDoesNotBlockKnownOrActivePeople(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	for _, id := range []string{"github:1", "github:2"} {
		if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: id, Provider: "github", Name: id}, store.UserPending, 2, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:3", Provider: "github"}, store.UserPending, 2, false); !errors.Is(err, store.ErrPendingFull) {
		t.Fatalf("a third pending sign-up must be refused: %v", err)
	}
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:1", Provider: "github", Name: "again"}, store.UserPending, 2, false); err != nil {
		t.Fatalf("someone already known can still sign in: %v", err)
	}
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:4", Provider: "github"}, store.UserPending, 2, true); err != nil {
		t.Fatalf("a named admin is not capped: %v", err)
	}
}
