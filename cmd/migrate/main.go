package main

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
)

// ── This function is intended to be run as a one-off command to migrate the database schema.
func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg := config.Load()
	rawDB, err := database.OpenRawDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer func() { _ = rawDB.Close() }()

	bunDB := database.OpenBunDB(rawDB)
	if err := database.EnsureBunSchema(context.Background(), bunDB); err != nil {
		log.Error().Err(err).Msg("Failed to migrate database schema")
		return
	}

	log.Info().Msg("Database schema is up to date")
}
