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

func TestNewUsersStartWithTheGivenStatusAndKeepWhatAnAdminSets(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	st, err := q.EnsureUser(ctx, "github:42", "", "alice", store.StatusPending, 0)
	if err != nil || st.Status != store.StatusPending {
		t.Fatalf("new user = %+v, %v", st, err)
	}
	// A new sign-in must not turn a pending user active (or the other way round).
	if st, _ := q.EnsureUser(ctx, "github:42", "a@x", "Alice", store.StatusActive, 0); st.Status != store.StatusPending {
		t.Fatalf("sign-in changed the status to %s", st.Status)
	}
	if err := q.SetUserStatus(ctx, "github:42", store.StatusRejected, "not on the team", "admin"); err != nil {
		t.Fatal(err)
	}
	st, _ = q.EnsureUser(ctx, "github:42", "a@x", "Alice", store.StatusPending, 0)
	if st.Status != store.StatusRejected || st.Reason != "not on the team" {
		t.Fatalf("a rejected user must stay rejected: %+v", st)
	}
	if err := q.SetUserStatus(ctx, "nobody", store.StatusActive, "", "admin"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown user: %v", err)
	}
	if err := q.SetUserStatus(ctx, "github:42", "pending", "", "admin"); err == nil {
		t.Fatal("pending cannot be set by hand")
	}
}

func TestSuspendingEndsSessionsAtOnce(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	if _, err := q.EnsureUser(ctx, "github:7", "", "bob", store.StatusActive, 0); err != nil {
		t.Fatal(err)
	}
	if err := q.CreateSession(ctx, store.Session{ID: "sid", UserOID: "github:7", Roles: []string{"developer"}, GroupIDs: []string{}, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "sid"); err != nil {
		t.Fatal(err)
	}
	if err := q.SetUserStatus(ctx, "github:7", store.StatusSuspended, "left the company", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSession(ctx, "sid"); err == nil {
		t.Fatal("a suspended user's session must end at once")
	}
}

func TestPendingListIsCapped(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	for _, id := range []string{"github:1", "github:2"} {
		if _, err := q.EnsureUser(ctx, id, "", id, store.StatusPending, 2); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.EnsureUser(ctx, "github:3", "", "c", store.StatusPending, 2); !errors.Is(err, store.ErrSignupClosed) {
		t.Fatalf("a third pending sign-up must be refused: %v", err)
	}
	// Someone already known can still sign in, and an active sign-up is not capped.
	if _, err := q.EnsureUser(ctx, "github:1", "", "again", store.StatusPending, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := q.EnsureUser(ctx, "github:4", "", "admin", store.StatusActive, 2); err != nil {
		t.Fatal(err)
	}
	if store.ProviderOf("github:1") != "github" || store.ProviderOf("abc-guid") != "entra" {
		t.Fatal("ProviderOf")
	}
}
