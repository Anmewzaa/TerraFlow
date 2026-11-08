// Package storetest gives tests a real, migrated Postgres database.
//
// Set TF_TEST_PG_DSN to a server DSN that may create databases, for example
//
//	postgres://terraflow:localdev@localhost:5432/postgres?sslmode=disable
//
// Each call creates a throwaway database, migrates it and drops it when the test
// ends. Without the variable the test is skipped, so plain `go test ./...` still
// works on a machine with no database.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// DSN returns a migrated throwaway database's DSN and a Queries for it.
func New(t testing.TB) (*store.Queries, string) {
	t.Helper()
	adminDSN := os.Getenv("TF_TEST_PG_DSN")
	if adminDSN == "" {
		t.Skip("TF_TEST_PG_DSN not set; skipping the Postgres test")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Fatalf("connect to %s: %v", adminDSN, err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatalf("ping postgres: %v", err)
	}
	var b [6]byte
	rand.Read(b[:])
	name := "tf_test_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		admin.Close()
		t.Fatalf("create database: %v", err)
	}

	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dsn := u.String()

	if err := store.Migrate(dsn); err != nil {
		admin.Exec(ctx, fmt.Sprintf(`DROP DATABASE %q WITH (FORCE)`, name))
		admin.Close()
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(ctx, fmt.Sprintf(`DROP DATABASE %q WITH (FORCE)`, name))
		admin.Close()
	})
	return store.New(pool), dsn
}
