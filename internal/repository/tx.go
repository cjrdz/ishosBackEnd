// tx.go — Transaction helpers for Bun-backed repositories.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

// runInTx executes fn inside a Bun transaction. If db is already a transaction,
// fn runs directly on it; otherwise a new transaction is started and committed.
func runInTx(ctx context.Context, db bun.IDB, fn func(ctx context.Context, tx bun.Tx) error) error {
	if tx, ok := db.(bun.Tx); ok {
		return fn(ctx, tx)
	}
	if db, ok := db.(*bun.DB); ok {
		return db.RunInTx(ctx, &sql.TxOptions{}, fn)
	}
	return fmt.Errorf("unsupported db type: %T", db)
}
