// cmd/api/main.go — Application entry point.
//
// Chi is the HTTP router and server.
// PocketBase is bootstrapped as a Go library for authentication
// (token validation, employee management) — it does NOT run its own HTTP server.
// Ent + PostgreSQL is the ORM/database for business data.
//
// Usage:
//
//	go run ./cmd/api                     # start with defaults from .env
//	PORT=9000 go run ./cmd/api           # custom port
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/server"
)

func main() {
	// ── Logging ──────────────────────────────────────────────────────────
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// ── Configuration ────────────────────────────────────────────────────
	cfg := config.Load()

	if cfg.Env == "development" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// ── PocketBase (auth engine — Go API only, no HTTP server) ───────────
	pb, err := auth.New(cfg.PBDataDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialise PocketBase auth engine")
	}
	log.Info().Str("data_dir", cfg.PBDataDir).Msg("PocketBase auth engine ready")

	// ── One-time bootstrap ────────────────────────────────────────────────
	// When BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD are set and no
	// employee records exist yet, create the first admin automatically. This
	// is the only way to seed the first admin in a distroless Docker image
	// (e.g. Render) where there is no shell or Go runtime available.
	// After the first successful run you can unset these env vars.
	if cfg.BootstrapAdminEmail != "" && cfg.BootstrapAdminPassword != "" {
		existing, listErr := pb.ListEmployees()
		if listErr != nil {
			log.Warn().Err(listErr).Msg("Bootstrap: could not list employees, skipping auto-bootstrap")
		} else if len(existing) == 0 {
			_, createErr := pb.CreateEmployee(
				cfg.BootstrapAdminEmail,
				cfg.BootstrapAdminPassword,
				cfg.BootstrapAdminName,
				"",
				"admin",
				false,
			)
			if createErr != nil {
				log.Error().Err(createErr).Str("email", cfg.BootstrapAdminEmail).Msg("Bootstrap: failed to create admin")
			} else {
				log.Info().Str("email", cfg.BootstrapAdminEmail).Msg("Bootstrap: first admin created successfully — remove BOOTSTRAP_ADMIN_* env vars")
			}
		} else {
			log.Info().Int("count", len(existing)).Msg("Bootstrap: employees already exist, skipping")
		}
	}

	// ── PocketBase Admin UI (development only) ───────────────────────────
	// When ENABLE_PB_ADMIN_UI=true, start PocketBase's built-in HTTP server
	// on a separate port so you can manage employees, inspect tokens, etc.
	// via http://localhost:<PB_ADMIN_UI_PORT>/_/
	if cfg.EnablePBAdminUI {
		if cfg.Env == "production" && !cfg.AllowPBAdminUIInProduction {
			log.Warn().
				Msg("PocketBase admin UI requested but blocked in production; set ALLOW_PB_ADMIN_UI_IN_PRODUCTION=true to override")
		} else {
			go func() {
				addr := fmt.Sprintf("%s:%s", cfg.PBAdminUIHost, cfg.PBAdminUIPort)
				allowedOrigins := splitCSV(cfg.PBAdminUIAllowedOrigins)
				log.Info().
					Str("bind", addr).
					Str("origins", strings.Join(allowedOrigins, ",")).
					Msg("Starting PocketBase admin UI")
				if err := pb.StartAdminUI(addr, allowedOrigins); err != nil {
					log.Error().Err(err).Msg("PocketBase admin UI stopped")
				}
			}()
		}
	}

	// ── Server (Chi router + Ent database) ───────────────────────────────
	srv, err := server.New(pb, cfg)
	if err != nil {
		_ = pb.Close()
		log.Fatal().Err(err).Msg("Failed to initialise server")
	}

	// ── HTTP server with graceful shutdown ────────────────────────────────
	addr := ":" + cfg.Port
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start listening in a goroutine so we can handle shutdown signals.
	go func() {
		log.Info().
			Str("addr", addr).
			Str("env", cfg.Env).
			Msg("Chi API server started")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("Shutting down server…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	srv.Close()
	_ = pb.Close()
	log.Info().Msg("Server exited cleanly")
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			origins = append(origins, v)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:8090"}
	}
	return origins
}
