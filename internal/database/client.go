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

	return nil
}

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
