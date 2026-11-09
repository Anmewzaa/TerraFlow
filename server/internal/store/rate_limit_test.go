package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestTakeTokenSharesOneCountAndRefills(t *testing.T) {
	q, dsn := storetest.New(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	take := func(key string) bool {
		ok, err := q.TakeToken(ctx, key, 3, 10*time.Second, now)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for i := 0; i < 3; i++ {
		if !take("1.2.3.4") {
			t.Fatalf("take %d inside the burst was refused", i+1)
		}
	}
	if take("1.2.3.4") {
		t.Fatal("the fourth take must be refused")
	}
	if !take("5.6.7.8") {
		t.Fatal("another client must not be affected")
	}
	now = now.Add(9 * time.Second)
	if take("1.2.3.4") {
		t.Fatal("no token is back after 9s")
	}
	now = now.Add(2 * time.Second)
	if !take("1.2.3.4") {
		t.Fatal("a token is back after 11s")
	}
	// Idle rows are removed when a new client appears.
	now = now.Add(2 * time.Hour)
	take("9.9.9.9")
	var n int
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM rate_limits`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("idle rows must be purged, %d left", n)
	}
}
