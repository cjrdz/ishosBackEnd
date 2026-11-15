// cmd/bootstrap/main.go — Creates the first admin employee in PocketBase.
//
// This is used in production (where the PocketBase admin UI is disabled) to
// bootstrap the very first admin account so they can log in via the API and
// then create other employees through the Astro panel.
//
// Usage:
//
//	go run ./cmd/bootstrap -email admin@domain.com -password SecurePass123! [-name "Admin User"] [-phone "+50688881234"]
//
// The command connects to the PocketBase SQLite database at PB_DATA_DIR
// (default ./pb_data), creates the admin record, and exits. It does NOT
// start any HTTP server.
package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	email := flag.String("email", "", "Admin email (required)")
	password := flag.String("password", "", "Admin password (required)")
	name := flag.String("name", "Admin", "Admin display name")
	phone := flag.String("phone", "", "Admin phone number")
	flag.Parse()

	if *email == "" || *password == "" {
		log.Fatal().Msg("Usage: go run ./cmd/bootstrap -email admin@domain.com -password SecurePass123!")
	}

	// Load config to get PB_DATA_DIR (honours .env file and env vars).
	cfg := config.Load()

	log.Info().Str("data_dir", cfg.PBDataDir).Msg("Connecting to PocketBase")

	pb, err := auth.New(cfg.PBDataDir, cfg.AuthTokenTTLHours)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialise PocketBase")
	}

	record, err := pb.CreateEmployee(*email, *password, *name, *phone, "admin", "", false)
	if err != nil {
		_ = pb.Close()
		log.Fatal().Err(err).Str("email", *email).Msg("Failed to create admin employee")
	}

	log.Info().
		Str("id", record.Id).
		Str("email", record.Email()).
		Str("name", record.GetString("name")).
		Str("role", record.GetString("role")).
		Msg("✅ Admin employee created successfully")

	_ = pb.Close()
	log.Info().Msg("You can now log in via POST /api/v1/auth/login")
}
