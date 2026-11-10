package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// User statuses (migration 015).
const (
	UserActive    = "active"
	UserPending   = "pending"
	UserRejected  = "rejected"
	UserSuspended = "suspended"
)

// ErrPendingFull: too many sign-ups are waiting; a new one is refused until an admin
// clears some (so the approval list cannot be flooded).
var ErrPendingFull = errors.New("too many sign-ups are waiting for approval")

// ErrNotInvited: sign-up is by invitation only and this person has no account yet.
var ErrNotInvited = errors.New("sign-up is by invitation only")

// LoginUser is who a provider says just signed in.
type LoginUser struct {
	OID      string // github:<numeric id>, or the Entra object id
	Provider string // github | entra
	Login    string // display only; never an identifier
	Email    string
	Name     string
}

// RecordLogin creates the user with the status the sign-up policy chose, or refreshes
// the name, email and last login of a known one, and returns the status they have
// now. A known user keeps their status (a rejected or suspended person is not
// promoted by signing in again). force makes the person active at once and is for
// administrators named in the settings, who must never be locked out.
func (q *Queries) RecordLogin(ctx context.Context, u LoginUser, initial string, maxPending int, force bool) (status string, created bool, err error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `SELECT status FROM users WHERE oid = $1 FOR UPDATE`, u.OID).Scan(&status)
	switch {
	case err == nil:
		if force {
			status = UserActive
		}
		_, err = tx.Exec(ctx,
			`UPDATE users SET email = $2, name = $3, login = $4, provider = $5, status = $6,
			        last_login_at = now(),
			        status_at = CASE WHEN status <> $6 THEN now() ELSE status_at END,
			        status_by = CASE WHEN status <> $6 THEN 'settings' ELSE status_by END
			 WHERE oid = $1`, u.OID, u.Email, u.Name, u.Login, u.Provider, status)
	case errors.Is(err, pgx.ErrNoRows):
		if initial == "" && !force {
			return "", false, ErrNotInvited
		}
		created = true
		status = initial
		if force {
			status = UserActive
		}
		if status == UserPending && maxPending > 0 {
			var n int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE status = 'pending'`).Scan(&n); err != nil {
				return "", false, err
			}
			if n >= maxPending {
				return "", false, ErrPendingFull
			}
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO users (oid, email, name, login, provider, status, status_at)
			 VALUES ($1, $2, $3, $4, $5, $6, now())`, u.OID, u.Email, u.Name, u.Login, u.Provider, status)
	}
	if err != nil {
		return "", false, err
	}
	return status, created, tx.Commit(ctx)
}

// UserStatus returns the status of a user, or "" when there is no such row (a user from
// before migration 015 or a made-up development user: treated as active by callers).
func (q *Queries) UserStatus(ctx context.Context, oid string) (string, error) {
	var s string
	err := q.pool.QueryRow(ctx, `SELECT status FROM users WHERE oid = $1`, oid).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return s, err
}

// SetUserStatus changes a user's status, records who did it and why, and ends every
// session of a person who is no longer allowed in. pgx.ErrNoRows: no such user.
func (q *Queries) SetUserStatus(ctx context.Context, oid, status, reason, by string) error {
	switch status {
	case UserActive, UserPending, UserRejected, UserSuspended:
	default:
		return errors.New("unknown status")
	}
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`UPDATE users SET status = $2, status_reason = $3, status_by = $4, status_at = now() WHERE oid = $1`,
		oid, status, reason, by)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if status == UserRejected || status == UserSuspended {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_oid = $1`, oid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
