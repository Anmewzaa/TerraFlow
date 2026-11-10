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

func TestSignupStatusFlow(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	u := store.LoginUser{OID: "github:42", Provider: "github", Login: "alice", Email: "a@x.test", Name: "Alice"}

	st, created, err := q.RecordLogin(ctx, u, store.UserPending, 10, false)
	if err != nil || st != store.UserPending || !created {
		t.Fatalf("first login: %q %v %v", st, created, err)
	}
	// Signing in again does not change the status or create a second row.
	if st, created, _ = q.RecordLogin(ctx, u, store.UserActive, 10, false); st != store.UserPending || created {
		t.Fatalf("second login: %q created=%v", st, created)
	}
	if err := q.SetUserStatus(ctx, u.OID, store.UserActive, "", "admin-1"); err != nil {
		t.Fatal(err)
	}
	if s, _ := q.UserStatus(ctx, u.OID); s != store.UserActive {
		t.Fatalf("status = %q", s)
	}

	// Suspending ends the sessions at once.
	if err := q.CreateSession(ctx, store.Session{ID: "sid-1", UserOID: u.OID, Roles: []string{"developer"}, GroupIDs: []string{}, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := q.SetUserStatus(ctx, u.OID, store.UserSuspended, "left the team", "admin-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "sid-1"); err == nil {
		t.Fatal("a suspended user's session must be gone")
	}
	// A rejected or suspended person is not promoted by signing in again...
	if st, _, _ = q.RecordLogin(ctx, u, store.UserActive, 10, false); st != store.UserSuspended {
		t.Fatalf("after suspend, login gave %q", st)
	}
	// ...except an administrator named in the settings.
	if st, _, _ = q.RecordLogin(ctx, u, store.UserActive, 10, true); st != store.UserActive {
		t.Fatalf("forced login gave %q", st)
	}

	if err := q.SetUserStatus(ctx, "github:nobody", store.UserActive, "", "a"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown user: %v", err)
	}
	if err := q.SetUserStatus(ctx, u.OID, "bogus", "", "a"); err == nil {
		t.Fatal("an unknown status must be refused")
	}
	if s, _ := q.UserStatus(ctx, "nobody"); s != "" {
		t.Fatalf("unknown user status = %q, want empty", s)
	}
}

func TestPendingSignupsAreCapped(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	for _, id := range []string{"github:1", "github:2"} {
		if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: id, Provider: "github"}, store.UserPending, 2, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:3", Provider: "github"}, store.UserPending, 2, false); !errors.Is(err, store.ErrPendingFull) {
		t.Fatalf("third pending: %v", err)
	}
	// An administrator is not blocked by the cap, and a known pending user can still sign in.
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:3", Provider: "github"}, store.UserPending, 2, true); err != nil {
		t.Fatalf("forced: %v", err)
	}
	if _, _, err := q.RecordLogin(ctx, store.LoginUser{OID: "github:1", Provider: "github"}, store.UserPending, 2, false); err != nil {
		t.Fatalf("known pending user: %v", err)
	}
	list, _ := q.ListUsers(ctx)
	pending := 0
	for _, u := range list {
		if u.Status == store.UserPending {
			pending++
		}
	}
	if pending != 2 {
		t.Fatalf("pending = %d, want 2", pending)
	}
}
