// bun.go — Opens a Bun database client over the shared pgx *sql.DB pool.
package database

import (
	"context"
	stdsql "database/sql"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// OpenBunDB creates a Bun client from an existing *sql.DB pool.
// The caller owns the *sql.DB lifecycle; Bun only wraps it.
func OpenBunDB(rawDB *stdsql.DB) *bun.DB {
	db := bun.NewDB(rawDB, pgdialect.New())
	db.AddQueryHook(queryHook{})
	return db
}

type queryHook struct{}

func (queryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (queryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	dur := time.Since(event.StartTime)
	// Never log the raw query text — it may contain user data or secrets.
	logger := log.Debug().
		Dur("duration_ms", dur).
		Str("operation", event.Operation())
	if event.Err != nil && !isBenignQueryError(event.Err) {
		logger = logger.Err(event.Err)
	}
	if dur > 100*time.Millisecond {
		logger.Msg("slow SQL query")
	} else {
		logger.Msg("SQL query")
	}
}

// isBenignQueryError reports errors that are normal outcomes of application
// logic and should not be logged as SQL failures.
//   - sql.ErrNoRows: expected when a lookup finds nothing (e.g. idempotency key,
//     customer by phone).
//   - sql.ErrTxDone: expected from deferred tx.Rollback() after a successful commit.
func isBenignQueryError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, stdsql.ErrNoRows) || errors.Is(err, stdsql.ErrTxDone)
}
