// server.go — Builds the Chi router, wires all handlers and middleware, and manages the DB connection lifecycle.
package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	"github.com/cjrdz/ishosBackEnd/internal/repository"
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
	settingsRepo := repository.NewAdminSettingRepository(client)
	settingsSvc := services.NewAdminSettingService(settingsRepo)
	defaultTokenTTLs := services.TokenTTLConfig{
		AuthCookieTTLHours:    cfg.AuthCookieTTLHours,
		AuthTokenTTLHours:     cfg.AuthTokenTTLHours,
		TrackingTokenTTLHours: cfg.TrackingTokenTTLHours,
	}

	authHandler := handlers.NewAuthHandler(pb, cfg, settingsSvc)
	employeeHandler := handlers.NewEmployeeHandler(pb)
	categoryHandler := handlers.NewCategoryHandler(client)
	productHandler := handlers.NewProductHandler(client, cfg)
	flavorHandler := handlers.NewFlavorHandler(client)
	addonHandler := handlers.NewAddonHandler(client)
	uploadHandler := handlers.NewUploadHandler(cfg)
	analyticsHandler := handlers.NewAnalyticsHandler(client, cfg)
	exportHandler := handlers.NewExportHandler(client)
	userHandler := handlers.NewUserHandler(client)
	adminSettingsHandler := handlers.NewAdminSettingsHandler(client, cfg)

	// Order handler requires rate limiter
	orderRateLimiter := appmiddleware.NewOrderRateLimiter(
		cfg.OrdersPerHour,
		cfg.OrdersPerDay,
		cfg.TrackingRequestsPerHour,
		cfg.RedisURL,
		cfg.RedisPrefix,
	)
	trackingTokenStore := services.NewSQLTrackingTokenStore(rawDB)
	trackingTokenService := services.NewTrackingTokenService(
		cfg.TrackingTokenSecret,
		time.Duration(cfg.TrackingTokenTTLHours)*time.Hour,
		trackingTokenStore,
	)
	trackingTokenService.SetTTLResolver(func(ctx context.Context) time.Duration {
		resolved, err := settingsSvc.GetTokenTTLConfig(ctx, defaultTokenTTLs)
		if err != nil {
			return time.Duration(defaultTokenTTLs.TrackingTokenTTLHours) * time.Hour
		}
		return time.Duration(resolved.TrackingTokenTTLHours) * time.Hour
	})
	orderHandler := handlers.NewOrderHandler(client, trackingTokenService, pb, cfg, orderRateLimiter, settingsSvc)

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
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return isOriginAllowed(origin, allowedOrigins)
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
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

		// Orders (public create; optionally capture authenticated staff identity)
		api.With(pbAuthMiddleware.OptionalSession).Post("/orders", orderRateLimiter.Middleware(http.HandlerFunc(orderHandler.Create)).ServeHTTP)
		api.Get("/orders/track", orderHandler.TrackPublic)

		// ── Settings routes (mixed public + protected) ──────────────
		api.Route("/settings", func(settings chi.Router) {
			// Public store status (orders kill-switch + active offers)
			settings.Get("/store/public", adminSettingsHandler.GetPublicStoreStatus)

			// Protected settings (admin/employee)
			settings.Group(func(protectedSettings chi.Router) {
				protectedSettings.Use(pbAuthMiddleware.RequireSession)
				protectedSettings.Get("/tabs", adminSettingsHandler.GetTabs)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/tabs", adminSettingsHandler.UpdateTabs)
				protectedSettings.Get("/panel-config", adminSettingsHandler.GetPanelConfig)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/panel-config", adminSettingsHandler.UpdatePanelConfig)
				protectedSettings.Get("/store", adminSettingsHandler.GetStoreSettings)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/store", adminSettingsHandler.UpdateStoreSettings)
			})
		})

		// ── Authenticated routes (admin/employee only) ───────────────
		api.Group(func(protected chi.Router) {
			protected.Use(pbAuthMiddleware.RequireSession)
			protected.With(appmiddleware.RequireAdmin).Post("/auth/lockout/reset", authHandler.ResetLoginLockout)

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
			protected.Patch("/categories/{id}", categoryHandler.Update)
			protected.Delete("/categories/{id}", categoryHandler.Delete)

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
	seen := make(map[string]struct{}, len(parts)*3)

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimSuffix(trimmed, "/")

		// Bare hostnames (no scheme) are expanded to both https and http so
		// callers can configure origins without repeating the same host twice.
		if strings.Contains(trimmed, "://") {
			if _, ok := seen[trimmed]; !ok {
				seen[trimmed] = struct{}{}
				origins = append(origins, trimmed)
			}
			continue
		}

		httpsOrigin := "https://" + trimmed
		httpOrigin := "http://" + trimmed

		if _, ok := seen[httpsOrigin]; !ok {
			seen[httpsOrigin] = struct{}{}
			origins = append(origins, httpsOrigin)
		}
		if _, ok := seen[httpOrigin]; !ok {
			seen[httpOrigin] = struct{}{}
			origins = append(origins, httpOrigin)
		}
		if _, ok := seen[trimmed]; !ok {
			seen[trimmed] = struct{}{}
			origins = append(origins, trimmed)
		}
	}

	// Fall back to the default Astro dev server when no origins are configured.
	if len(origins) == 0 {
		return []string{"http://localhost:4321"}
	}

	return origins
}

func isOriginAllowed(origin string, allowedOrigins []string) bool {
	origin = strings.TrimSpace(strings.TrimSuffix(origin, "/"))
	if origin == "" {
		return false
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil || parsedOrigin.Scheme == "" || parsedOrigin.Host == "" {
		return false
	}

	originScheme := strings.ToLower(parsedOrigin.Scheme)
	originHost := strings.ToLower(parsedOrigin.Hostname())
	originHostPort := strings.ToLower(parsedOrigin.Host)
	originNormalized := strings.ToLower(origin)

	for _, allowed := range allowedOrigins {
		candidate := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(allowed, "/")))
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			return true
		}

		if strings.Contains(candidate, "://") {
			parts := strings.SplitN(candidate, "://", 2)
			if len(parts) != 2 {
				continue
			}

			candidateScheme := parts[0]
			candidateHost := parts[1]

			if candidateScheme != originScheme {
				continue
			}

			// Wildcard subdomain: *.example.com matches foo.example.com but not example.com itself.
			if strings.HasPrefix(candidateHost, "*.") {
				suffix := strings.TrimPrefix(candidateHost, "*")
				if strings.HasSuffix(originHost, suffix) {
					return true
				}
				continue
			}

			if candidateHost == originHost || candidateHost == originHostPort || candidate == originNormalized {
				return true
			}
			continue
		}

		// Schemeless wildcard: *.example.com without a scheme matches any scheme.
		if strings.HasPrefix(candidate, "*.") {
			suffix := strings.TrimPrefix(candidate, "*")
			if strings.HasSuffix(originHost, suffix) {
				return true
			}
			continue
		}

		if candidate == originHost || candidate == originHostPort || candidate == originNormalized {
			return true
		}
	}

	return false
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
