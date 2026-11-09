package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// User statuses (D-050). Only an active user may use TerraFlow.
const (
	StatusPending   = "pending"
	StatusActive    = "active"
	StatusRejected  = "rejected"
	StatusSuspended = "suspended"
)

// ErrSignupClosed: the pending list is full, so no new sign-up is taken.
var ErrSignupClosed = errors.New("sign-up is full for now")

// ProviderOf says which identity provider a user id belongs to: "github:<id>" is
// GitHub; an id without a prefix is Entra (it predates the prefix). Accounts of
// different providers are never linked.
func ProviderOf(oid string) string {
	if i := strings.Index(oid, ":"); i > 0 {
		return oid[:i]
	}
	return "entra"
}

// UserStatus is the account state an admin sees and a sign-in checks.
type UserStatus struct {
	Status string
	Reason string
}

// EnsureUser records a sign-in. A new user gets initial as their status (when
// maxPending is above zero and that many users already wait, a pending sign-up is
// refused with ErrSignupClosed); an existing user keeps the status an admin gave
// them, and only the name, email and last sign-in change. It returns the status.
func (q *Queries) EnsureUser(ctx context.Context, oid, email, name, initial string, maxPending int) (UserStatus, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return UserStatus{}, err
	}
	defer tx.Rollback(ctx)

	var st UserStatus
	err = tx.QueryRow(ctx,
		`UPDATE users SET email = $2, name = $3, last_login_at = now() WHERE oid = $1 RETURNING status, status_reason`,
		oid, email, name).Scan(&st.Status, &st.Reason)
	if err == nil {
		return st, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return UserStatus{}, err
	}
	if initial == StatusPending && maxPending > 0 {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE status = 'pending'`).Scan(&n); err != nil {
			return UserStatus{}, err
		}
		if n >= maxPending {
			return UserStatus{}, ErrSignupClosed
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO users (oid, email, name, provider, status) VALUES ($1, $2, $3, $4, $5)`,
		oid, email, name, ProviderOf(oid), initial); err != nil {
		return UserStatus{}, err
	}
	return UserStatus{Status: initial}, tx.Commit(ctx)
}

// GetUserStatus returns the status of a user; an unknown user is pgx.ErrNoRows.
func (q *Queries) GetUserStatus(ctx context.Context, oid string) (UserStatus, error) {
	var st UserStatus
	err := q.pool.QueryRow(ctx, `SELECT status, status_reason FROM users WHERE oid = $1`, oid).Scan(&st.Status, &st.Reason)
	return st, err
}

// SetUserStatus changes a user's status (approve, reject, suspend, restore) and
// ends the sessions of anyone who is no longer active, at once.
func (q *Queries) SetUserStatus(ctx context.Context, oid, status, reason, by string) error {
	switch status {
	case StatusActive, StatusRejected, StatusSuspended:
	default:
		return fmt.Errorf("invalid status %q", status)
	}
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ct, err := tx.Exec(ctx,
		`UPDATE users SET status = $2, status_reason = $3, status_by = $4, status_at = $5 WHERE oid = $1`,
		oid, status, reason, by, time.Now())
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if status != StatusActive {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_oid = $1`, oid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// UserExists reports whether the user has signed in before.
func (q *Queries) UserExists(ctx context.Context, oid string) (bool, error) {
	var ok bool
	err := q.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE oid = $1)`, oid).Scan(&ok)
	return ok, err
}
