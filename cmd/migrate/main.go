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
	client, rawDB, err := database.OpenClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer client.Close()

	if err := database.EnsureSchema(context.Background(), client, rawDB); err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate database schema")
	}

	log.Info().Msg("Database schema is up to date")
}
