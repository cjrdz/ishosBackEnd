// server.go — Builds the Chi router, wires all handlers and middleware, and manages the DB connection lifecycle.
package server

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
	"github.com/cjrdz/ishosBackEnd/internal/events"
	"github.com/cjrdz/ishosBackEnd/internal/handlers"
	appmiddleware "github.com/cjrdz/ishosBackEnd/internal/middleware"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// Server wraps the Chi router, database connection, and all handlers.
type Server struct {
	db                 *stdsql.DB
	router             chi.Router
	cfg                *config.Config
	trackingTokenStore *repository.TrackingTokenBunStore
	cleanupCancel      context.CancelFunc
}

// New creates a new Server instance with all dependencies initialized.
func New(pb *auth.PBAuth, cfg *config.Config) (*Server, error) {
	rawDB, err := database.OpenRawDB(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database client: %w", err)
	}
	bunDB := database.OpenBunDB(rawDB)

	if err := database.EnsureBunSchema(context.Background(), bunDB); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("failed to migrate database schema: %w", err)
	}
	auth.SetTokenRevocationBackend(repository.NewRevokedTokenBunStore(bunDB))
	if err := repository.InitTrackingTokenStore(bunDB); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("failed to initialize tracking token store: %w", err)
	}
	log.Info().Msg("Connected to PostgreSQL database and verified schema")

	// ── Repositories ─────────────────────────────────────────────────────
	categoryRepo := repository.NewCategoryBunRepository(bunDB)
	flavorRepo := repository.NewFlavorBunRepository(bunDB)
	addonRepo := repository.NewAddonBunRepository(bunDB)
	productRepo := repository.NewProductBunRepository(bunDB)
	orderRepo := repository.NewOrderBunRepository(bunDB)
	settingsRepo := repository.NewAdminSettingBunRepository(bunDB)
	customerRepo := repository.NewCustomerBunRepository(bunDB)
	containerTypeRepo := repository.NewContainerTypeBunRepository(bunDB)
	inventoryRepo := repository.NewInventoryBunRepository(bunDB)

	// ── Services ─────────────────────────────────────────────────────────
	categorySvc := services.NewCategoryService(categoryRepo)
	customerSvc := services.NewCustomerService(customerRepo)
	flavorSvc := services.NewFlavorService(flavorRepo)
	addonSvc := services.NewAddonService(addonRepo)
	productSvc := services.NewProductService(productRepo)
	settingsSvc := services.NewAdminSettingService(settingsRepo)
	containerTypeSvc := services.NewContainerTypeService(containerTypeRepo)
	inventoryService := services.NewInventoryService(inventoryRepo, containerTypeRepo)
	auditSvc := audit.NewService(repository.NewAuditLogBunRepository(bunDB))
	authService := auth.NewService(
		pb,
		cfg.LoginAttemptsPerMinute,
		cfg.LoginLockoutMaxFailures,
		cfg.LoginLockoutWindowMinutes,
		cfg.LoginLockoutDurationMinutes,
	)
	defaultTokenTTLs := services.TokenTTLConfig{
		AuthCookieTTLHours:      cfg.AuthCookieTTLHours,
		AuthTokenTTLHours:       cfg.AuthTokenTTLHours,
		TrackingTokenTTLHours:   cfg.TrackingTokenTTLHours,
		InactivityLogoutSeconds: cfg.InactivityLogoutSeconds,
	}

	// ── Tracking token service (needed by order service) ─────────────────
	trackingTokenStore := repository.NewTrackingTokenBunStore(bunDB)
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

	// ── Order composition (moved from handler constructor) ─────────────────
	whatsapp := services.NewWhatsAppService(cfg)
	orderDispatcher := events.NewDispatcher()
	orderDispatcher.SubscribeOrderStatusChanged(func(evt events.OrderStatusChanged) {
		switch evt.Status {
		case events.OrderStatusCreated:
			whatsapp.SendOrderCreated(evt.Phone, evt.OrderNumber, evt.Total, evt.TrackingURL)
		case events.OrderStatusApproved:
			whatsapp.SendOrderApproved(evt.Phone, evt.OrderNumber, evt.TrackingURL)
		case events.OrderStatusRejected:
			whatsapp.SendOrderRejected(evt.Phone, evt.OrderNumber, evt.Reason, evt.TrackingURL)
		case events.OrderStatusReady:
			whatsapp.SendOrderReady(evt.Phone, evt.OrderNumber, evt.TrackingURL)
		}
	})
	orderStore := repository.NewOrderStore(bunDB, productRepo, services.NewOrderEnrichmentService())
	orderSvc := services.NewOrderService(
		orderRepo,
		orderStore,
		customerRepo,
		productRepo,
		inventoryService,
		trackingTokenService,
		orderDispatcher,
		cfg.PublicStoreURL,
		auditSvc,
	)
	orderResponseMapper := services.NewOrderResponseMapper(pb)
	publicStoreURL := strings.TrimRight(strings.TrimSpace(cfg.PublicStoreURL), "/")

	// ── Handlers ─────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(pb, authService, cfg, settingsSvc)
	employeeHandler := handlers.NewEmployeeHandler(pb, auditSvc)
	categoryHandler := handlers.NewCategoryHandler(categorySvc)
	productHandler := handlers.NewProductHandler(productSvc, cfg, inventoryService)
	// Pre-warm the product list cache so the first admin login request is fast.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := productSvc.List(ctx, "", true); err != nil {
			log.Warn().Err(err).Msg("Failed to pre-warm product cache")
		} else {
			log.Debug().Msg("Product cache pre-warmed")
		}
	}()
	flavorHandler := handlers.NewFlavorHandler(flavorSvc)
	addonHandler := handlers.NewAddonHandler(addonSvc)
	uploadHandler := handlers.NewUploadHandler(cfg)
	analyticsHandler := handlers.NewAnalyticsHandler(orderRepo, cfg)
	exportHandler := handlers.NewExportHandler(orderRepo, auditSvc)
	userHandler := handlers.NewUserHandler(customerSvc)
	adminSettingsHandler := handlers.NewAdminSettingsHandler(settingsSvc, auditSvc)

	// Inventory handler
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	containerTypeHandler := handlers.NewContainerTypeHandler(containerTypeSvc)

	// Order handler requires rate limiter
	orderRateLimiter := appmiddleware.NewOrderRateLimiter(
		cfg.OrdersPerHour,
		cfg.OrdersPerDay,
		cfg.TrackingRequestsPerHour,
		cfg.RedisURL,
		cfg.RedisPrefix,
	)
	orderHandler := handlers.NewOrderHandler(orderSvc, trackingTokenService, orderResponseMapper, settingsSvc, publicStoreURL, orderRateLimiter)

	// ── Middleware ───────────────────────────────────────────────────────
	loginRateLimiter := appmiddleware.NewLoginRateLimit(authService)
	authMiddleware := appmiddleware.NewAuth(authService)

	// ── Router setup ─────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(appmiddleware.RequestIDMiddleware())
	r.Use(middleware.ClientIPFromXFFTrustedProxies(1))
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
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API routes
	r.Route("/api/v1", func(api chi.Router) {
		// ── Public routes ────────────────────────────────────────────

		// Auth
		api.Post("/auth/login", loginRateLimiter.Middleware(http.HandlerFunc(authHandler.Login)).ServeHTTP)
		api.Post("/auth/logout", authHandler.Logout)
		api.With(authMiddleware.OptionalSession).Get("/auth/session", authHandler.GetSession)

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
		api.With(authMiddleware.OptionalSession).Post("/orders", orderRateLimiter.Middleware(http.HandlerFunc(orderHandler.Create)).ServeHTTP)
		api.Get("/orders/track", orderHandler.TrackPublic)

		// ── Settings routes (mixed public + protected) ──────────────
		api.Route("/settings", func(settings chi.Router) {
			// Public store status (orders kill-switch + active offers)
			settings.Get("/store/public", adminSettingsHandler.GetPublicStoreStatus)

			// Protected settings (admin/employee)
			settings.Group(func(protectedSettings chi.Router) {
				protectedSettings.Use(authMiddleware.RequireSession)
				protectedSettings.Get("/store", adminSettingsHandler.GetStoreSettings)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/store", adminSettingsHandler.UpdateStoreSettings)
			})
		})

		// ── Authenticated routes (admin/employee only) ───────────────
		api.Group(func(protected chi.Router) {
			protected.Use(authMiddleware.RequireSession)
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
			protected.With(appmiddleware.RequireAdmin).Post("/users/upsert", userHandler.Upsert)
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

			// Inventory (admin only)
			protected.With(appmiddleware.RequireAdmin).Route("/inventory", func(inv chi.Router) {
				inv.Get("/", inventoryHandler.List)
				inv.Get("/flavors", inventoryHandler.ListFlavorInventory)
				inv.Get("/unit-products", inventoryHandler.ListUnitInventory)
				inv.Post("/unit-products", inventoryHandler.CreateUnitInventoryItem)
				inv.Get("/stats", inventoryHandler.GetStats)
				inv.Get("/low-stock", inventoryHandler.GetLowStock)
				inv.Get("/{id}", inventoryHandler.Get)
				inv.Get("/{id}/movements", inventoryHandler.GetMovementHistory)
				inv.Post("/{id}/entries", inventoryHandler.RecordEntry)
				inv.Post("/{id}/unit-entries", inventoryHandler.RecordUnitEntry)
				inv.Post("/{id}/adjustments", inventoryHandler.RecordAdjustment)
				inv.Post("/{id}/products/{productId}", inventoryHandler.LinkInventoryItemToProduct)
				inv.Delete("/{id}/products/{productId}", inventoryHandler.UnlinkInventoryItemFromProduct)
				inv.Get("/flavors/{id}/stock", inventoryHandler.GetStockForFlavor)
			})

			// Container types (admin only)
			protected.With(appmiddleware.RequireAdmin).Route("/container-types", func(ct chi.Router) {
				ct.Get("/", containerTypeHandler.List)
				ct.Post("/", containerTypeHandler.Create)
				ct.Get("/{id}", containerTypeHandler.Get)
				ct.Patch("/{id}", containerTypeHandler.Update)
				ct.Delete("/{id}", containerTypeHandler.Delete)
			})

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
		db:                 rawDB,
		router:             r,
		cfg:                cfg,
		trackingTokenStore: trackingTokenStore,
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

// StartBackgroundJobs starts non-blocking maintenance goroutines (e.g. periodic
// cleanup of expired tracking tokens). Call this after the server is running.
func (s *Server) StartBackgroundJobs(ctx context.Context) {
	if s.trackingTokenStore == nil {
		return
	}

	jobCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run once shortly after startup to clean any backlog.
		if err := s.trackingTokenStore.CleanupExpired(jobCtx); err != nil {
			log.Warn().Err(err).Msg("Initial tracking-token cleanup failed")
		}

		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				if err := s.trackingTokenStore.CleanupExpired(jobCtx); err != nil {
					log.Warn().Err(err).Msg("Periodic tracking-token cleanup failed")
				}
			}
		}
	}()
}

// Close gracefully shuts down the database connection.
func (s *Server) Close() {
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database connection")
		} else {
			log.Info().Msg("Database connection closed")
		}
	}
}
