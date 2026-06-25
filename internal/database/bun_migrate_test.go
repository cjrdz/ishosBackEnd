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

	// Spot-check a few tables exist using the raw *sql.DB.
	for _, table := range []string{"orders", "order_items", "products", "customers"} {
		var name string
		query := "SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1"
		err := rawDB.QueryRowContext(ctx, query, table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		require.Equal(t, table, name)
	}
}
