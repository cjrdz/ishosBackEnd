// client.go — Opens the PostgreSQL connection pool and provides shared backfill helpers.
package database

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/config"
)

// OpenRawDB opens a *sql.DB from the configured PostgreSQL URL and applies
// the operator-tuned connection pool settings.
func OpenRawDB(cfg *config.Config) (*stdsql.DB, error) {
	pgCfg, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	rawDB := stdlib.OpenDB(*pgCfg)
	rawDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	rawDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	rawDB.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeMinutes) * time.Minute)
	rawDB.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeMinutes) * time.Minute)

	// Warm up the connection pool so the first real request doesn't pay
	// connection-establishment cost.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rawDB.PingContext(ctx); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return rawDB, nil
}

// sqlExecutor is satisfied by both *sql.DB and *bun.DB so backfill helpers can
// be reused by the migration path.
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (stdsql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *stdsql.Row
}

// ensureBunDB is a compile-time assertion that *bun.DB satisfies sqlExecutor.
var _ sqlExecutor = (*bun.DB)(nil)

// verifyAddonGroupColumn is a post-migration sanity check: schema creation does not always
// apply ALTER TABLE additions when the table already exists, so we assert the column is present
// rather than discovering the missing column at runtime on the first write.
func verifyAddonGroupColumn(ctx context.Context, db sqlExecutor) error {
	const query = `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = current_schema()
	  AND table_name = 'addons'
	  AND column_name = 'group_name'
)`

	var exists bool
	if err := db.QueryRowContext(ctx, query).Scan(&exists); err != nil {
		return fmt.Errorf("verify addons.group_name column: %w", err)
	}
	if !exists {
		return fmt.Errorf("verify addons.group_name column: missing after migration")
	}

	return nil
}

// backfillOrderItemSnapshotProductID copies the existing product foreign-key
// (product_order_items) into the new snapshot_product_id column for rows that
// haven't been backfilled yet. This lets list queries avoid loading the product
// edge while keeping detail views working during the transition.
func backfillOrderItemSnapshotProductID(ctx context.Context, db sqlExecutor) error {
	const query = `
UPDATE order_items
SET snapshot_product_id = product_order_items
WHERE snapshot_product_id IS NULL
  AND product_order_items IS NOT NULL`

	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("backfill order_items.snapshot_product_id: %w", err)
	}
	if res != nil {
		rows, rowsErr := res.RowsAffected()
		if rowsErr == nil && rows > 0 {
			// Best-effort logging via stdlib log since zerolog isn't wired here.
			fmt.Printf("backfilled %d order_items.snapshot_product_id rows\n", rows)
		}
	}
	return nil
}

// ensureOrderItemIndexes creates indexes on the order_item foreign-key columns
// if they don't already exist. Without these indexes, item lookups trigger a
// full table scan on every order list query.
func ensureOrderItemIndexes(ctx context.Context, db sqlExecutor) error {
	const orderIndex = `CREATE INDEX IF NOT EXISTS orderitem_order_id ON order_items(order_items)`
	const productIndex = `CREATE INDEX IF NOT EXISTS orderitem_product_id ON order_items(product_order_items)`

	if _, err := db.ExecContext(ctx, orderIndex); err != nil {
		return fmt.Errorf("create orderitem_order_id index: %w", err)
	}
	if _, err := db.ExecContext(ctx, productIndex); err != nil {
		return fmt.Errorf("create orderitem_product_id index: %w", err)
	}
	return nil
}

// backfillOrderCustomerID links historical orders to canonical customer records.
// It creates a customer for each distinct phone that does not yet exist, then
// updates orders.customer_id to point at the matching customer.
func backfillOrderCustomerID(ctx context.Context, db sqlExecutor) error {
	const createMissingCustomers = `
INSERT INTO customers (id, name, user_type, phone, email, status, created_at, updated_at)
SELECT gen_random_uuid(),
       MAX(o.customer_name) AS name,
       'user' AS user_type,
       o.customer_phone AS phone,
       MAX(o.customer_email) AS email,
       'active' AS status,
       NOW() AS created_at,
       NOW() AS updated_at
FROM orders o
LEFT JOIN customers c ON c.phone = o.customer_phone
WHERE o.customer_id IS NULL
  AND c.id IS NULL
GROUP BY o.customer_phone
ON CONFLICT (phone) DO NOTHING`

	if _, err := db.ExecContext(ctx, createMissingCustomers); err != nil {
		return fmt.Errorf("backfill orders.customer_id (create customers): %w", err)
	}

	const linkOrders = `
UPDATE orders o
SET customer_id = c.id
FROM customers c
WHERE o.customer_id IS NULL
  AND c.phone = o.customer_phone`

	res, err := db.ExecContext(ctx, linkOrders)
	if err != nil {
		return fmt.Errorf("backfill orders.customer_id (link orders): %w", err)
	}
	if res != nil {
		rows, rowsErr := res.RowsAffected()
		if rowsErr == nil && rows > 0 {
			fmt.Printf("backfilled %d orders.customer_id rows\n", rows)
		}
	}
	return nil
}

func verifyProductGlobalExclusionColumns(ctx context.Context, db sqlExecutor) error {
	const query = `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'products'
  AND column_name IN ('exclude_global_flavors', 'exclude_global_addons')`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("verify products exclusion columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	found := map[string]bool{
		"exclude_global_flavors": false,
		"exclude_global_addons":  false,
	}

	for rows.Next() {
		var columnName string
		if scanErr := rows.Scan(&columnName); scanErr != nil {
			return fmt.Errorf("verify products exclusion columns scan: %w", scanErr)
		}
		found[columnName] = true
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return fmt.Errorf("verify products exclusion columns rows: %w", rowsErr)
	}

	if !found["exclude_global_flavors"] || !found["exclude_global_addons"] {
		return fmt.Errorf("verify products exclusion columns: missing after migration")
	}

	return nil
}
