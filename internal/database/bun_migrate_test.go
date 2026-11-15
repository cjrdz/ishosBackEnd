// bun_migrate_test.go — Smoke test for Bun schema creation.
// Skips unless a Postgres DATABASE_URL is available and reachable.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

func TestEnsureBunSchema(t *testing.T) {
	ctx := context.Background()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/ishosFactory?sslmode=disable"
	}

	rawDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Skipf("cannot open postgres (%v); skipping bun migration smoke test", err)
	}
	defer func() { _ = rawDB.Close() }()
	// SET search_path below is per-connection; pin the pool to one connection
	// so the migration transaction runs in the test schema.
	rawDB.SetMaxOpenConns(1)

	if err := rawDB.PingContext(ctx); err != nil {
		t.Skipf("postgres not reachable (%v); skipping bun migration smoke test", err)
	}

	// Use a fresh schema so the test is hermetic.
	schemaName := "bun_migrate_test"
	if _, err := rawDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)); err != nil {
		t.Fatalf("drop test schema: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName)); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", schemaName)); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	db := OpenBunDB(rawDB)
	require.NoError(t, EnsureBunSchema(ctx, db))
	// A second run must be a no-op on an up-to-date schema.
	require.NoError(t, EnsureBunSchema(ctx, db))

	var version int
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&version))
	require.Equal(t, schemaVersion, version)

	// Upgrade path: a database migrated before schema_version existed (full
	// schema + data, no version row) must migrate idempotently and keep its data.
	_, err = rawDB.ExecContext(ctx, `INSERT INTO categories (id, name, slug, is_active, display_order, created_at, updated_at)
		VALUES (gen_random_uuid(), 'Conos', 'conos', true, 0, NOW(), NOW())`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, "DROP TABLE schema_version")
	require.NoError(t, err)

	require.NoError(t, EnsureBunSchema(ctx, db))

	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&version))
	require.Equal(t, schemaVersion, version)
	var categories int
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM categories").Scan(&categories))
	require.Equal(t, 1, categories, "existing rows must survive the upgrade")

	// v1 → v2: products zeroed by the update bug get their free topping back,
	// once; a 0 set after the upgrade is left alone.
	_, err = rawDB.ExecContext(ctx, `INSERT INTO products (id, name, description, price, category_products, free_toppings, created_at, updated_at)
		SELECT gen_random_uuid(), 'Cono', '', 0.9, id, 0, NOW(), NOW() FROM categories LIMIT 1`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, "DELETE FROM schema_version WHERE version > 1")
	require.NoError(t, err)
	require.NoError(t, EnsureBunSchema(ctx, db))
	var free int
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT free_toppings FROM products WHERE name = 'Cono'").Scan(&free))
	require.Equal(t, 1, free, "v2 repairs products reset to 0 by the update bug")
	_, err = rawDB.ExecContext(ctx, "UPDATE products SET free_toppings = 0 WHERE name = 'Cono'")
	require.NoError(t, err)
	require.NoError(t, EnsureBunSchema(ctx, db))
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT free_toppings FROM products WHERE name = 'Cono'").Scan(&free))
	require.Equal(t, 0, free, "the repair runs once; later deliberate zeros stay")

	// v2 → v3: rebuild a single-shop (v2) layout, then upgrade it.
	for _, stmt := range []string{
		"DROP TABLE location_availability",
		"ALTER TABLE orders DROP COLUMN location_id",
		"ALTER TABLE stock_movements DROP COLUMN location_id",
		"ALTER TABLE inventory_items DROP COLUMN location_id",
		"ALTER TABLE audit_logs DROP COLUMN location_id",
		"DROP TABLE locations",
		"ALTER TABLE inventory_items ADD CONSTRAINT inventory_items_name_key UNIQUE (name)",
		"DELETE FROM schema_version WHERE version > 2",
		`INSERT INTO flavors (id, name, created_at, updated_at) VALUES ('11111111-1111-1111-1111-111111111111', 'Fresa', NOW(), NOW())`,
		`INSERT INTO inventory_items (id, name, type, current_stock, low_stock_threshold, flavor_id, created_at, updated_at)
			VALUES ('22222222-2222-2222-2222-222222222222', 'Fresa', 'ball_based', 40, 10, '11111111-1111-1111-1111-111111111111', NOW(), NOW())`,
		`INSERT INTO stock_movements (id, type, quantity, inventory_item_id, created_at)
			VALUES (gen_random_uuid(), 'entry', 40, '22222222-2222-2222-2222-222222222222', NOW())`,
		`INSERT INTO orders (id, order_number, customer_name, customer_phone, total_amount, status_timestamps, created_at, updated_at)
			VALUES (gen_random_uuid(), 'ORD-20261101-0001', 'Ana', '70000000', 1.5, '{}', NOW(), NOW())`,
		`INSERT INTO admin_settings (id, key, value, created_at, updated_at)
			VALUES (gen_random_uuid(), 'store.hours', '{"days": {"mon": [{"open": "14:00", "close": "21:00"}]}}', NOW(), NOW()),
			       (gen_random_uuid(), 'store.orders.enabled', '{"enabled": false}', NOW(), NOW())`,
	} {
		_, err = rawDB.ExecContext(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	require.NoError(t, EnsureBunSchema(ctx, db))
	require.NoError(t, EnsureBunSchema(ctx, db), "v3 must be idempotent")

	var locations, ordersOnFirst, unassigned int
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM locations").Scan(&locations))
	require.Equal(t, 3, locations)
	var firstID string
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT id::text FROM locations WHERE slug = 'sucursal-1'").Scan(&firstID))
	require.NoError(t, rawDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE location_id = $1", firstID).Scan(&ordersOnFirst))
	require.Equal(t, 1, ordersOnFirst, "existing orders belong to Sucursal 1")
	require.NoError(t, rawDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_movements sm JOIN inventory_items ii ON ii.id = sm.inventory_item_id WHERE sm.location_id <> ii.location_id").Scan(&unassigned))
	require.Equal(t, 0, unassigned, "movements follow their item's location")

	var stockFirst, stockOthers, rowsOthers int
	require.NoError(t, rawDB.QueryRowContext(ctx,
		"SELECT current_stock FROM inventory_items WHERE flavor_id = '11111111-1111-1111-1111-111111111111' AND location_id = $1", firstID).Scan(&stockFirst))
	require.Equal(t, 40, stockFirst)
	require.NoError(t, rawDB.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(SUM(current_stock), 0) FROM inventory_items WHERE flavor_id = '11111111-1111-1111-1111-111111111111' AND location_id <> $1", firstID).Scan(&rowsOthers, &stockOthers))
	require.Equal(t, 2, rowsOthers, "the other locations track the flavour from zero")
	require.Equal(t, 0, stockOthers)

	var enabledCount, onlineCount, withHours int
	require.NoError(t, rawDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FILTER (WHERE orders_enabled), COUNT(*) FILTER (WHERE online_orders_enabled), COUNT(*) FILTER (WHERE hours ? 'mon') FROM locations").
		Scan(&enabledCount, &onlineCount, &withHours))
	require.Equal(t, 0, enabledCount, "the global orders switch is copied")
	require.Equal(t, 0, onlineCount, "online ordering starts off")
	require.Equal(t, 3, withHours, "the global hours are copied")

	// Spot-check a few tables exist using the raw *sql.DB.
	for _, table := range []string{"orders", "order_items", "products", "customers"} {
		var name string
		query := "SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1"
		err := rawDB.QueryRowContext(ctx, query, table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		require.Equal(t, table, name)
	}
}
