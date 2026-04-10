package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
	"github.com/cjrdz/ishosBackEnd/internal/handlers"
	appmiddleware "github.com/cjrdz/ishosBackEnd/internal/middleware"
	"github.com/cjrdz/ishosBackEnd/internal/services"
)

// Server wraps the Chi router, Ent database client, and all handlers.
type Server struct {
	db     *ent.Client
	router chi.Router
	cfg    *config.Config
}

// New creates a new Server instance with all dependencies initialized.
func New(pb *auth.PBAuth, cfg *config.Config) (*Server, error) {
	client, rawDB, err := database.OpenClient(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database client: %w", err)
	}

	if err := database.EnsureSchema(context.Background(), client, rawDB); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to migrate database schema: %w", err)
	}
	if err := services.EnsureTrackingTokenSchema(context.Background(), rawDB); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to initialize tracking token schema: %w", err)
	}
	if err := auth.InitTokenRevocationStore(rawDB); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to initialize token revocation store: %w", err)
	}
	log.Info().Msg("Connected to PostgreSQL database and verified schema")

	// ── Initialize handlers ──────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(pb, cfg)
	employeeHandler := handlers.NewEmployeeHandler(pb)
	categoryHandler := handlers.NewCategoryHandler(client)
	productHandler := handlers.NewProductHandler(client, cfg)
	flavorHandler := handlers.NewFlavorHandler(client)
	addonHandler := handlers.NewAddonHandler(client)
	uploadHandler := handlers.NewUploadHandler(cfg)
	analyticsHandler := handlers.NewAnalyticsHandler(client)
	exportHandler := handlers.NewExportHandler(client)
	userHandler := handlers.NewUserHandler(client)
	adminSettingsHandler := handlers.NewAdminSettingsHandler(client)

	// Order handler requires rate limiter
	orderRateLimiter := appmiddleware.NewOrderRateLimiter(
		cfg.OrdersPerHour,
		cfg.OrdersPerDay,
		cfg.TrackingRequestsPerHour,
		cfg.RedisURL,
		cfg.RedisPrefix,
	)
	trackingTokenStore := services.NewSQLTrackingTokenStore(rawDB)
	trackingTokenService := services.NewTrackingTokenService(cfg.TrackingTokenSecret, trackingTokenStore)
	orderHandler := handlers.NewOrderHandler(client, trackingTokenService, pb, cfg, orderRateLimiter)

	// ── Middleware ───────────────────────────────────────────────────────
	loginRateLimiter := appmiddleware.NewLoginRateLimiter(cfg.LoginAttemptsPerMinute)
	pbAuthMiddleware := appmiddleware.NewPBAuth(pb)

	// ── Router setup ─────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(appmiddleware.SecurityHeaders)
	r.Use(appmiddleware.Logger)
	r.Use(appmiddleware.Recovery)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	allowedOrigins := parseAllowedOrigins(cfg.AllowedOrigins)
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

		// Flavors (public list of active)
		api.Get("/flavors", flavorHandler.List)
		api.Get("/flavors/{id}", flavorHandler.Get)

		// Addons (public list of active)
		api.Get("/addons", addonHandler.List)
		api.Get("/addons/{id}", addonHandler.Get)

		// Orders (public create)
		api.Post("/orders", orderRateLimiter.Middleware(http.HandlerFunc(orderHandler.Create)).ServeHTTP)
		api.Get("/orders/track", orderHandler.TrackPublic)

		// ── Authenticated routes (admin/employee only) ───────────────
		api.Group(func(protected chi.Router) {
			protected.Use(pbAuthMiddleware.RequireSession)

			// Employees management
			protected.With(appmiddleware.RequireAdmin).Get("/employees", employeeHandler.List)
			protected.With(appmiddleware.RequireAdmin).Post("/employees", employeeHandler.Create)
			protected.With(appmiddleware.RequireAdmin).Patch("/employees/{id}", employeeHandler.Update)
			protected.With(appmiddleware.RequireAdmin).Patch("/employees/{id}/deactivate", employeeHandler.Deactivate)
			protected.With(appmiddleware.RequireAdmin).Delete("/employees/{id}", employeeHandler.Delete)

			// Users (customer directory)
			protected.Get("/users", userHandler.List)
			protected.Get("/users/{id}", userHandler.Get)
			protected.With(appmiddleware.RequireAdmin).Get("/users/{id}/orders", userHandler.Orders)
			protected.With(appmiddleware.RequireAdmin).Post("/users", userHandler.Create)
			protected.With(appmiddleware.RequireAdmin).Patch("/users/{id}", userHandler.Update)
			protected.With(appmiddleware.RequireAdmin).Delete("/users/{id}", userHandler.Delete)

			// Admin dashboard settings
			protected.Get("/settings/tabs", adminSettingsHandler.GetTabs)
			protected.With(appmiddleware.RequireAdmin).Patch("/settings/tabs", adminSettingsHandler.UpdateTabs)

			// Products management
			protected.Post("/products", productHandler.Create)
			protected.Patch("/products/{id}", productHandler.Update)
			protected.Patch("/products/{id}/availability", productHandler.ToggleAvailability)
			protected.Delete("/products/{id}", productHandler.Delete)

			// Product-Flavor linking
			protected.Post("/products/{id}/flavors/{flavorId}", productHandler.LinkFlavor)
			protected.Delete("/products/{id}/flavors/{flavorId}", productHandler.UnlinkFlavor)

			// Product-Addon linking
			protected.Post("/products/{id}/addons/{addonId}", productHandler.LinkAddon)
			protected.Delete("/products/{id}/addons/{addonId}", productHandler.UnlinkAddon)

			// Flavors management
			protected.Post("/flavors", flavorHandler.Create)
			protected.Patch("/flavors/{id}", flavorHandler.Update)
			protected.Delete("/flavors/{id}", flavorHandler.Delete)

			// Addons management
			protected.Post("/addons", addonHandler.Create)
			protected.Patch("/addons/{id}", addonHandler.Update)
			protected.Delete("/addons/{id}", addonHandler.Delete)

			// Categories management
			protected.Post("/categories", categoryHandler.Create)
			protected.Patch("/categories/{slug}", categoryHandler.Update)
			protected.Delete("/categories/{slug}", categoryHandler.Delete)

			// Orders management
			protected.Get("/orders", orderHandler.List)
			protected.Get("/orders/{id}", orderHandler.Get)
			protected.Patch("/orders/{id}", orderHandler.Update)
			protected.Post("/orders/{id}/approve", orderHandler.Approve)
			protected.Post("/orders/{id}/reject", orderHandler.Reject)
			protected.Patch("/orders/{id}/status", orderHandler.UpdateStatus)
			protected.Patch("/orders/{id}/notes", orderHandler.UpdateNotes)
			protected.Delete("/orders/{id}", orderHandler.Delete)

			// Analytics (admin only)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/overview", analyticsHandler.Overview)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/orders-over-time", analyticsHandler.OrdersOverTime)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/top-products", analyticsHandler.TopProducts)

			// Export and purge (admin only)
			protected.With(appmiddleware.RequireAdmin).Get("/export/orders", exportHandler.ExportOrders)
			protected.With(appmiddleware.RequireAdmin).Delete("/export/orders", exportHandler.PurgeOrders)

			// Upload
			protected.Post("/upload/image", uploadHandler.UploadImage)
			protected.With(appmiddleware.RequireAdmin).Delete("/upload/image", uploadHandler.DeleteImage)
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

func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimSuffix(trimmed, "/")
		origins = append(origins, trimmed)
	}

	if len(origins) == 0 {
		return []string{"http://localhost:4321"}
	}

	return origins
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
