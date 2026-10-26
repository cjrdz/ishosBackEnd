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

	// Spot-check a few tables exist using the raw *sql.DB.
	for _, table := range []string{"orders", "order_items", "products", "customers"} {
		var name string
		query := "SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1"
		err := rawDB.QueryRowContext(ctx, query, table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		require.Equal(t, table, name)
	}
}
