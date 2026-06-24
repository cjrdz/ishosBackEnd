// bun.go — Opens a Bun database client over the shared pgx *sql.DB pool.
package database

import (
	stdsql "database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// OpenBunDB creates a Bun client from an existing *sql.DB pool.
// The caller owns the *sql.DB lifecycle; Bun only wraps it.
func OpenBunDB(rawDB *stdsql.DB) *bun.DB {
	return bun.NewDB(rawDB, pgdialect.New())
}
