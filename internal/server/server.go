package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/handlers"
	appmiddleware "github.com/cjrdz/ishosBackEnd/internal/middleware"
)

// Server wraps the Chi router, Ent database client, and all handlers.
type Server struct {
	db     *ent.Client
	router chi.Router
	cfg    *config.Config
}

// New creates a new Server instance with all dependencies initialized.
func New(pb *auth.PBAuth, cfg *config.Config) (*Server, error) {
	// ── Database connection ──────────────────────────────────────────────
	pgCfg, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database url: %w", err)
	}

	drv := sql.OpenDB(dialect.Postgres, stdlib.OpenDB(*pgCfg))

	// Create Ent client
	client := ent.NewClient(ent.Driver(drv))
	log.Info().Msg("Connected to PostgreSQL database")

	// ── Initialize handlers ──────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(pb, cfg)
	employeeHandler := handlers.NewEmployeeHandler(pb)
	categoryHandler := handlers.NewCategoryHandler(client)
	productHandler := handlers.NewProductHandler(client, cfg)
	uploadHandler := handlers.NewUploadHandler(cfg)

	// Order handler requires rate limiter
	orderRateLimiter := appmiddleware.NewOrderRateLimiter(cfg.OrdersPerHour, cfg.OrdersPerDay)
	orderHandler := handlers.NewOrderHandler(client, cfg, orderRateLimiter)

	// ── Middleware ───────────────────────────────────────────────────────
	loginRateLimiter := appmiddleware.NewLoginRateLimiter(cfg.LoginAttemptsPerMinute)
	pbAuthMiddleware := appmiddleware.NewPBAuth(pb)

	// ── Router setup ─────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(appmiddleware.Logger)
	r.Use(appmiddleware.Recovery)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	allowedOrigins := strings.Split(cfg.AllowedOrigins, ",")
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// API routes
	r.Route("/api/v1", func(api chi.Router) {
		// ── Public routes ────────────────────────────────────────────

		// Auth
		api.Post("/auth/login", loginRateLimiter.Middleware(http.HandlerFunc(authHandler.Login)).ServeHTTP)
		api.Post("/auth/logout", authHandler.Logout)
		api.With(pbAuthMiddleware.OptionalSession).Get("/auth/session", authHandler.GetSession)

		// Products (public list, admin manage)
		api.Get("/products", productHandler.List)
		api.Get("/products/{id}", productHandler.Get)

		// Categories (public)
		api.Get("/categories", categoryHandler.List)
		api.Get("/categories/{slug}", categoryHandler.GetBySlug)

		// Orders (public create)
		api.Post("/orders", orderRateLimiter.Middleware(http.HandlerFunc(orderHandler.Create)).ServeHTTP)

		// ── Authenticated routes (admin/employee only) ───────────────
		api.Group(func(protected chi.Router) {
			protected.Use(pbAuthMiddleware.RequireSession)

			// Employees management
			protected.Get("/employees", employeeHandler.List)
			protected.Post("/employees", employeeHandler.Create)
			protected.Patch("/employees/{id}", employeeHandler.Update)
			protected.Patch("/employees/{id}/deactivate", employeeHandler.Deactivate)
			protected.Delete("/employees/{id}", employeeHandler.Delete)

			// Products management
			protected.Post("/products", productHandler.Create)
			protected.Patch("/products/{id}", productHandler.Update)
			protected.Patch("/products/{id}/availability", productHandler.ToggleAvailability)
			protected.Delete("/products/{id}", productHandler.Delete)

			// Categories management
			protected.Post("/categories", categoryHandler.Create)
			protected.Patch("/categories/{slug}", categoryHandler.Update)
			protected.Delete("/categories/{slug}", categoryHandler.Delete)

			// Orders management
			protected.Get("/orders", orderHandler.List)
			protected.Get("/orders/{id}", orderHandler.Get)
			protected.Patch("/orders/{id}/status", orderHandler.UpdateStatus)
			protected.Delete("/orders/{id}", orderHandler.Delete)

			// Upload
			protected.Post("/upload/image", uploadHandler.UploadImage)
			protected.Delete("/upload/image", uploadHandler.DeleteImage)
			protected.Get("/upload/images", uploadHandler.ListImages)
		})
	})

	log.Info().Msg("Chi router configured with all routes")

	return &Server{
		db:     client,
		router: r,
		cfg:    cfg,
	}, nil
}

// Router returns the configured Chi router.
func (s *Server) Router() http.Handler {
	return s.router
}

// Close gracefully shuts down the database connection.
func (s *Server) Close() {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database connection")
		} else {
			log.Info().Msg("Database connection closed")
		}
	}
}
