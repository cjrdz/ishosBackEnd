// cmd/cleanup/main.go — One-off cleanup of legacy flavor/addon product-link columns.
//
// Usage:
//
//	go run ./cmd/cleanup
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("Cleanup failed")
	}
}

func run() error {
	cfg := config.Load()
	rawDB, err := database.OpenRawDB(cfg)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer func() { _ = rawDB.Close() }()

	ctx := context.Background()

	stmts := []string{
		`ALTER TABLE flavors DROP COLUMN IF EXISTS product_flavors;`,
		`ALTER TABLE addons DROP COLUMN IF EXISTS product_addons;`,
		`ALTER TABLE products DROP COLUMN IF EXISTS flavor_products;`,
		`ALTER TABLE products DROP COLUMN IF EXISTS addon_products;`,
	}

	for _, stmt := range stmts {
		if _, execErr := rawDB.ExecContext(ctx, stmt); execErr != nil {
			return fmt.Errorf("execute cleanup statement %q: %w", stmt, execErr)
		}
		log.Info().Str("stmt", stmt).Msg("Executed successfully")
	}

	log.Info().Msg("Cleanup complete: legacy product-link columns removed")
	return nil
}
