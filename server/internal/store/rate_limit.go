package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// rateRowTTL is how long an idle client row is kept; a full bucket forgets nothing
// important after that.
const rateRowTTL = time.Hour

// TakeToken takes one token from the bucket of key: the bucket holds burst tokens and
// gets one more every `every`. It reports whether a token was available. The row is
// locked for the length of the transaction, so concurrent servers never both take the
// last token.
func (q *Queries) TakeToken(ctx context.Context, key string, burst int, every time.Duration, now time.Time) (bool, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx,
		`INSERT INTO rate_limits (key, tokens, updated_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO NOTHING`,
		key, float64(burst), now)
	if err != nil {
		return false, err
	}
	if ct.RowsAffected() == 1 { // a new client: also drop rows nobody has used for a while
		if _, err := tx.Exec(ctx, `DELETE FROM rate_limits WHERE updated_at < $1`, now.Add(-rateRowTTL)); err != nil {
			return false, err
		}
	}
	var tokens float64
	var last time.Time
	if err := tx.QueryRow(ctx, `SELECT tokens, updated_at FROM rate_limits WHERE key = $1 FOR UPDATE`, key).Scan(&tokens, &last); err != nil && err != pgx.ErrNoRows {
		return false, err
	}
	if elapsed := now.Sub(last); elapsed > 0 {
		tokens += float64(elapsed) / float64(every)
	}
	if tokens > float64(burst) {
		tokens = float64(burst)
	}
	ok := tokens >= 1
	if ok {
		tokens--
	}
	if _, err := tx.Exec(ctx, `UPDATE rate_limits SET tokens = $2, updated_at = $3 WHERE key = $1`, key, tokens, now); err != nil {
		return false, err
	}
	return ok, tx.Commit(ctx)
}
