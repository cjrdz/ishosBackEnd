// client.go — Opens the Ent/PostgreSQL client and verifies the schema on startup.
// OpenClient wires pgx → database/sql → Ent so a single *sql.DB is shared across both layers.
// EnsureSchema runs Ent auto-migration and guards against any columns that migrations may have missed.
package database

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/cjrdz/ishosBackEnd/ent"
)

func OpenClient(databaseURL string) (*ent.Client, *stdsql.DB, error) {
	pgCfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse database url: %w", err)
	}

	rawDB := stdlib.OpenDB(*pgCfg)
	// Pool sized for a single-instance API: enough headroom without exhausting Postgres connection slots.
	rawDB.SetMaxOpenConns(10)
	rawDB.SetMaxIdleConns(5)
	rawDB.SetConnMaxIdleTime(5 * time.Minute)
	rawDB.SetConnMaxLifetime(30 * time.Minute)
	driver := entsql.OpenDB(dialect.Postgres, rawDB)

	return ent.NewClient(ent.Driver(driver)), rawDB, nil
}

func EnsureSchema(ctx context.Context, client *ent.Client, rawDB *stdsql.DB) error {
	if err := client.Schema.Create(ctx); err != nil {
		return fmt.Errorf("run schema migration: %w", err)
	}

	if err := verifyAddonGroupColumn(ctx, rawDB); err != nil {
		return err
	}

	if err := verifyProductGlobalExclusionColumns(ctx, rawDB); err != nil {
		return err
	}

	return nil
}

// verifyAddonGroupColumn is a post-migration sanity check: Ent schema.Create does not always
// apply ALTER TABLE additions when the table already exists, so we assert the column is present
// rather than discovering the missing column at runtime on the first write.
func verifyAddonGroupColumn(ctx context.Context, rawDB *stdsql.DB) error {
	const query = `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = current_schema()
	  AND table_name = 'addons'
	  AND column_name = 'group_name'
)`

	var exists bool
	if err := rawDB.QueryRowContext(ctx, query).Scan(&exists); err != nil {
		return fmt.Errorf("verify addons.group_name column: %w", err)
	}
	if !exists {
		return fmt.Errorf("verify addons.group_name column: missing after migration")
	}

	return nil
}

func verifyProductGlobalExclusionColumns(ctx context.Context, rawDB *stdsql.DB) error {
	const query = `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'products'
  AND column_name IN ('exclude_global_flavors', 'exclude_global_addons')`

	rows, err := rawDB.QueryContext(ctx, query)
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
