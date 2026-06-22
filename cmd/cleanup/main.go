// cmd/cleanup/main.go — One-off cleanup of legacy flavor/addon product-link columns.
//
// Usage:
//
//	go run ./cmd/cleanup
package main

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg := config.Load()
	_, rawDB, err := database.OpenClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
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
			log.Fatal().Err(execErr).Str("stmt", stmt).Msg("Failed to execute cleanup statement")
		}
		log.Info().Str("stmt", stmt).Msg("Executed successfully")
	}

	log.Info().Msg("Cleanup complete: legacy product-link columns removed")
}
