// server.go — Builds the Chi router, wires all handlers and middleware, and manages the DB connection lifecycle.
package server

import (
	"bytes"
	"context"
	stdsql "database/sql"
	"fmt"
	"io"
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
	"github.com/cjrdz/ishosBackEnd/internal/openapi"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

const maxJSONBodyBytes = 1 << 20 // 1 MiB

// Server wraps the Chi router, database connection, and all handlers.
type Server struct {
	db                 *stdsql.DB
	router             chi.Router
	cfg                *config.Config
	trackingTokenStore *repository.TrackingTokenBunStore
	revokedTokenStore  *repository.RevokedTokenBunStore
	orderStore         *repository.OrderStore
	archiver           *services.OrderArchiver
	dispatcher         *events.Dispatcher
	whatsapp           *services.WhatsAppService
	cleanupCancel      context.CancelFunc
	archiverCancel     context.CancelFunc
}

// New creates a new Server instance with all dependencies initialized.
func New(pb *auth.PBAuth, cfg *config.Config) (*Server, error) {
	rawDB, err := database.OpenRawDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open database client: %w", err)
	}
	bunDB := database.OpenBunDB(rawDB)

	if err := database.EnsureBunSchema(context.Background(), bunDB); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("failed to migrate database schema: %w", err)
	}
	revokedTokenStore := repository.NewRevokedTokenBunStore(bunDB)
	auth.SetTokenRevocationBackend(revokedTokenStore)
	if err := repository.InitTrackingTokenStore(bunDB); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("failed to initialize tracking token store: %w", err)
	}
	log.Info().Msg("Connected to PostgreSQL database and verified schema")

	auditSvc := audit.NewService(repository.NewAuditLogBunRepository(bunDB))

	// ── Repositories ─────────────────────────────────────────────────────
	categoryRepo := repository.NewCategoryBunRepository(bunDB)
	flavorRepo := repository.NewFlavorBunRepository(bunDB)
	addonRepo := repository.NewAddonBunRepository(bunDB)
	productRepo := repository.NewProductBunRepository(bunDB)
	orderRepo := repository.NewOrderBunRepository(bunDB, auditSvc).WithStoreTimeZone(cfg.StoreTimeZone)
	settingsRepo := repository.NewAdminSettingBunRepository(bunDB)
	customerRepo := repository.NewCustomerBunRepository(bunDB)
	containerTypeRepo := repository.NewContainerTypeBunRepository(bunDB)
	inventoryRepo := repository.NewInventoryBunRepository(bunDB)
	locationRepo := repository.NewLocationBunRepository(bunDB)

	// ── Services ─────────────────────────────────────────────────────────
	categorySvc := services.NewCategoryService(categoryRepo)
	customerSvc := services.NewCustomerService(customerRepo)
	flavorSvc := services.NewFlavorService(flavorRepo)
	addonSvc := services.NewAddonService(addonRepo)
	productSvc := services.NewProductService(productRepo)
	settingsSvc := services.NewAdminSettingService(settingsRepo).WithLocation(cfg.StoreLocation())
	containerTypeSvc := services.NewContainerTypeService(containerTypeRepo)
	locationSvc := services.NewLocationService(locationRepo, cfg.StoreLocation())
	inventoryService := services.NewInventoryService(inventoryRepo, containerTypeRepo,
		services.WithFlavorRepo(flavorRepo), services.WithLocationIDs(locationSvc.ActiveIDs))
	authService := auth.NewService(
		pb,
		cfg.LoginAttemptsPerMinute,
		cfg.LoginLockoutMaxFailures,
		cfg.LoginLockoutWindowMinutes,
		cfg.LoginLockoutDurationMinutes,
		cfg.RedisURL,
		cfg.RedisPrefix,
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
	orderStore := repository.NewOrderStore(bunDB, productRepo, services.NewOrderEnrichmentService(), auditSvc)
	orderSvc := services.NewOrderService(
		orderRepo,
		orderStore,
		customerRepo,
		productRepo,
		inventoryService,
		trackingTokenService,
		orderDispatcher,
		cfg.PublicStoreURL,
	)
	orderSvc.SetOfferPrices(settingsSvc)
	archiveDefaults := services.ArchiveConfig{
		Enabled:         cfg.OrderArchiveEnabled,
		AgeDays:         cfg.OrderArchiveAgeDays,
		IntervalMinutes: cfg.OrderArchiveIntervalMinutes,
	}
	orderArchiver := services.NewOrderArchiver(orderRepo, settingsSvc, archiveDefaults)
	orderResponseMapper := services.NewOrderResponseMapper(pb)
	publicStoreURL := strings.TrimRight(strings.TrimSpace(cfg.PublicStoreURL), "/")

	// ── Handlers ─────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(pb, authService, cfg, settingsSvc, locationSvc)
	employeeHandler := handlers.NewEmployeeHandler(pb, auditSvc, locationSvc)
	locationHandler := handlers.NewLocationHandler(locationSvc, auditSvc)
	categoryHandler := handlers.NewCategoryHandler(categorySvc, auditSvc)
	productHandler := handlers.NewProductHandler(productSvc, cfg, inventoryService, auditSvc)
	// Pre-warm the product list cache so the first admin login request is fast.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Product cache pre-warm panic recovered")
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := productSvc.List(ctx, "", true); err != nil {
			log.Warn().Err(err).Msg("Failed to pre-warm product cache")
		} else {
			log.Debug().Msg("Product cache pre-warmed")
		}
	}()
	flavorHandler := handlers.NewFlavorHandler(flavorSvc, auditSvc)
	addonHandler := handlers.NewAddonHandler(addonSvc, auditSvc)
	uploadHandler := handlers.NewUploadHandler(cfg, auditSvc)
	analyticsHandler := handlers.NewAnalyticsHandler(orderRepo, cfg)
	exportHandler := handlers.NewExportHandler(orderRepo, auditSvc, cfg.StoreLocation())
	userHandler := handlers.NewUserHandler(customerSvc, auditSvc)
	adminSettingsHandler := handlers.NewAdminSettingsHandler(settingsSvc, locationSvc, auditSvc)

	// Inventory handler
	inventoryHandler := handlers.NewInventoryHandler(inventoryService, auditSvc)
	containerTypeHandler := handlers.NewContainerTypeHandler(containerTypeSvc, auditSvc)

	// Order handler requires rate limiter
	orderRateLimiter := appmiddleware.NewOrderRateLimiter(
		settingsSvc,
		services.PublicOrderRateLimitConfig{
			PhoneBurst:         cfg.PublicOrderPhoneBurst,
			PhoneWindowMinutes: cfg.PublicOrderPhoneWindowMinutes,
			PhoneBlockMinutes:  cfg.PublicOrderPhoneBlockMinutes,
			IPBurst:            cfg.PublicOrderIPBurst,
			IPWindowMinutes:    cfg.PublicOrderIPWindowMinutes,
			IPBlockMinutes:     cfg.PublicOrderIPBlockMinutes,
			StaffBurst:         cfg.PublicOrderStaffBurst,
			StaffWindowMinutes: cfg.PublicOrderStaffWindowMinutes,
		},
		cfg.TrackingRequestsPerHour,
		time.Hour,
		cfg.RedisURL,
		cfg.RedisPrefix,
	)
	orderHandler := handlers.NewOrderHandler(orderSvc, trackingTokenService, orderResponseMapper, settingsSvc, locationSvc, orderArchiver, publicStoreURL, orderRateLimiter, auditSvc)

	// ── Middleware ───────────────────────────────────────────────────────
	loginRateLimiter := appmiddleware.NewLoginRateLimit(authService)
	authMiddleware := appmiddleware.NewAuth(authService)
	inactivityTracker := appmiddleware.NewInactivityTracker()
	inactivityMiddleware := appmiddleware.NewInactivityMiddleware(inactivityTracker, settingsSvc, defaultTokenTTLs)
	resolveLocation := appmiddleware.ResolveLocation(locationSvc)
	// Public order creation also serves the admin POS: when a staff session is
	// present, resolve its location the same way protected routes do.
	resolveLocationIfSession := func(next http.Handler) http.Handler {
		withLocation := resolveLocation(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := auth.GetRecord(r.Context()); ok {
				withLocation.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// ── Router setup ─────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(appmiddleware.RequestIDMiddleware())
	r.Use(appmiddleware.ClientIP(cfg.BFFSharedSecret))
	r.Use(appmiddleware.SecurityHeaders)
	r.Use(appmiddleware.Logger)
	r.Use(appmiddleware.Recovery)
	r.Use(jsonBodySizeLimit)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	allowedOrigins := parseAllowedOrigins(cfg.AllowedOrigins)
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return isOriginAllowed(origin, allowedOrigins)
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With", appmiddleware.LocationHeader},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// OpenAPI spec endpoints (Huma docs-only mode; real traffic still uses Chi routes)
	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := openapi.GenerateJSON()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to generate OpenAPI spec")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		data, err := openapi.GenerateYAML()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to generate OpenAPI spec")
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(data)
	})
	// Interactive API docs (Scalar UI rendered from the live spec)
	r.Get("/docs", openapi.DocsHandler)

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
		api.With(authMiddleware.OptionalSession, resolveLocationIfSession).Post("/orders", orderRateLimiter.Middleware(http.HandlerFunc(orderHandler.Create)).ServeHTTP)
		api.Get("/orders/track", orderHandler.TrackPublic)

		// Locations (public: list with hours/status and Agotados per location)
		api.Get("/locations", locationHandler.ListPublic)
		api.Get("/locations/{id}/availability", locationHandler.GetAvailability)

		// ── Settings routes (mixed public + protected) ──────────────
		api.Route("/settings", func(settings chi.Router) {
			// Public store status (orders kill-switch + active offers)
			settings.Get("/store/public", adminSettingsHandler.GetPublicStoreStatus)

			// Protected settings (admin/employee)
			settings.Group(func(protectedSettings chi.Router) {
				protectedSettings.Use(authMiddleware.RequireSession)
				protectedSettings.Use(inactivityMiddleware.Middleware)
				protectedSettings.Get("/store", adminSettingsHandler.GetStoreSettings)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/store", adminSettingsHandler.UpdateStoreSettings)
				protectedSettings.With(appmiddleware.RequireAdmin).Get("/archive", adminSettingsHandler.GetArchiveSettings)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/archive", adminSettingsHandler.UpdateArchiveSettings)
				protectedSettings.With(appmiddleware.RequireAdmin).Get("/rate-limits", adminSettingsHandler.GetPublicOrderRateLimitConfig)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/rate-limits", adminSettingsHandler.UpdatePublicOrderRateLimitConfig)
				protectedSettings.With(appmiddleware.RequireAdmin).Get("/panel-config", adminSettingsHandler.GetPanelConfig)
				protectedSettings.With(appmiddleware.RequireAdmin).Patch("/panel-config", adminSettingsHandler.UpdatePanelConfig)
			})
		})

		// ── Authenticated routes (admin/employee only) ───────────────
		api.Group(func(protected chi.Router) {
			protected.Use(authMiddleware.RequireSession)
			protected.Use(inactivityMiddleware.Middleware)
			// Every protected route knows its location scope: admins choose one
			// (or all), managers and employees are fixed to their own.
			protected.Use(resolveLocation)
			protected.With(appmiddleware.RequireAdmin).Post("/auth/lockout/reset", authHandler.ResetLoginLockout)

			// Locations: admins manage all; a manager edits hours and the
			// orders switch of their own; any staff marks Agotados at theirs.
			protected.Get("/locations/manage", locationHandler.ListManaged)
			protected.With(appmiddleware.RequireAdmin).Post("/locations", locationHandler.Create)
			protected.With(appmiddleware.RequireRole(auth.RoleAdmin, auth.RoleManager)).Patch("/locations/{id}", locationHandler.Update)
			protected.Put("/locations/{id}/availability", locationHandler.SetAvailability)

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

			// Products management. Catalog writes (prices, names, images) are
			// admin-only; staff may still toggle availability during service.
			protected.With(appmiddleware.RequireAdmin).Post("/products", productHandler.Create)
			protected.With(appmiddleware.RequireAdmin).Patch("/products/{id}", productHandler.Update)
			// Global on/off of a product in the catalog (admin). Per-location
			// "Agotado" is PUT /locations/{id}/availability.
			protected.With(appmiddleware.RequireAdmin).Patch("/products/{id}/availability", productHandler.ToggleAvailability)
			protected.With(appmiddleware.RequireAdmin).Delete("/products/{id}", productHandler.Delete)

			// Product-Flavor linking
			protected.With(appmiddleware.RequireAdmin).Post("/products/{id}/flavors/{flavorId}", productHandler.LinkFlavor)
			protected.With(appmiddleware.RequireAdmin).Delete("/products/{id}/flavors/{flavorId}", productHandler.UnlinkFlavor)

			// Product-Addon linking
			protected.With(appmiddleware.RequireAdmin).Post("/products/{id}/addons/{addonId}", productHandler.LinkAddon)
			protected.With(appmiddleware.RequireAdmin).Delete("/products/{id}/addons/{addonId}", productHandler.UnlinkAddon)

			// Flavors management
			protected.With(appmiddleware.RequireAdmin).Post("/flavors", flavorHandler.Create)
			protected.With(appmiddleware.RequireAdmin).Patch("/flavors/{id}", flavorHandler.Update)
			protected.With(appmiddleware.RequireAdmin).Delete("/flavors/{id}", flavorHandler.Delete)

			// Addons management
			protected.With(appmiddleware.RequireAdmin).Post("/addons", addonHandler.Create)
			protected.With(appmiddleware.RequireAdmin).Patch("/addons/{id}", addonHandler.Update)
			protected.With(appmiddleware.RequireAdmin).Delete("/addons/{id}", addonHandler.Delete)

			// Categories management
			protected.With(appmiddleware.RequireAdmin).Post("/categories", categoryHandler.Create)
			protected.With(appmiddleware.RequireAdmin).Patch("/categories/{id}", categoryHandler.Update)
			protected.With(appmiddleware.RequireAdmin).Delete("/categories/{id}", categoryHandler.Delete)

			// Orders management. Every /orders/{id} route is behind
			// RequireOrderInScope: another location's order is a 404.
			protected.Get("/orders", orderHandler.List)
			protected.Route("/orders/{id}", func(order chi.Router) {
				order.Use(orderHandler.RequireOrderInScope)
				order.Get("/", orderHandler.Get)
				order.Patch("/", orderHandler.Update)
				order.Post("/approve", orderHandler.Approve)
				order.Post("/reject", orderHandler.Reject)
				order.Patch("/status", orderHandler.UpdateStatus)
				order.Patch("/notes", orderHandler.UpdateNotes)
				order.Patch("/archive", orderHandler.Archive)
				order.With(appmiddleware.RequireAdmin).Delete("/", orderHandler.Delete)
			})

			// Internal order creation (staff/back-office, bypasses public rate limits)
			protected.Post("/orders/internal", orderHandler.Create)

			// Admin archive run
			protected.With(appmiddleware.RequireAdmin).Post("/admin/orders/archive/run", orderHandler.RunArchive)

			// Analytics (admin only)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/overview", analyticsHandler.Overview)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/orders-over-time", analyticsHandler.OrdersOverTime)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/top-products", analyticsHandler.TopProducts)
			protected.With(appmiddleware.RequireAdmin).Get("/analytics/dashboard", analyticsHandler.Dashboard)

			// Inventory: admins (any location) and managers (their own).
			protected.With(appmiddleware.RequireRole(auth.RoleAdmin, auth.RoleManager)).Route("/inventory", func(inv chi.Router) {
				inv.Get("/", inventoryHandler.List)
				inv.Get("/dashboard", inventoryHandler.Dashboard)
				inv.Get("/flavors", inventoryHandler.ListFlavorInventory)
				inv.Get("/unit-products", inventoryHandler.ListUnitInventory)
				inv.With(appmiddleware.RequireAdmin).Post("/unit-products", inventoryHandler.CreateUnitInventoryItem)
				inv.Get("/stats", inventoryHandler.GetStats)
				inv.Get("/low-stock", inventoryHandler.GetLowStock)
				inv.Get("/{id}", inventoryHandler.Get)
				inv.Get("/{id}/movements", inventoryHandler.GetMovementHistory)
				inv.Post("/{id}/entries", inventoryHandler.RecordEntry)
				inv.Post("/{id}/unit-entries", inventoryHandler.RecordUnitEntry)
				inv.Post("/{id}/adjustments", inventoryHandler.RecordAdjustment)
				inv.With(appmiddleware.RequireAdmin).Post("/{id}/products/{productId}", inventoryHandler.LinkInventoryItemToProduct)
				inv.With(appmiddleware.RequireAdmin).Delete("/{id}/products/{productId}", inventoryHandler.UnlinkInventoryItemFromProduct)
				inv.Get("/flavors/{id}/stock", inventoryHandler.GetStockForFlavor)
			})

			// Container types: managers read them to record stock entries;
			// only admins change them.
			protected.With(appmiddleware.RequireRole(auth.RoleAdmin, auth.RoleManager)).Route("/container-types", func(ct chi.Router) {
				ct.Get("/", containerTypeHandler.List)
				ct.With(appmiddleware.RequireAdmin).Post("/", containerTypeHandler.Create)
				ct.Get("/{id}", containerTypeHandler.Get)
				ct.With(appmiddleware.RequireAdmin).Patch("/{id}", containerTypeHandler.Update)
				ct.With(appmiddleware.RequireAdmin).Delete("/{id}", containerTypeHandler.Delete)
			})

			// Export and purge (admin only)
			protected.With(appmiddleware.RequireAdmin).Get("/export/orders", exportHandler.ExportOrders)
			protected.With(appmiddleware.RequireAdmin).Delete("/export/orders", exportHandler.PurgeOrders)

			// Upload
			protected.With(appmiddleware.RequireAdmin).Post("/upload/image", uploadHandler.UploadImage)
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
		revokedTokenStore:  revokedTokenStore,
		orderStore:         orderStore,
		archiver:           orderArchiver,
		dispatcher:         orderDispatcher,
		whatsapp:           whatsapp,
	}, nil
}

// jsonBodySizeLimit caps JSON request bodies at 1 MiB and returns 413 before the
// handler runs. Upload endpoints manage their own larger limits, so they are
// skipped to avoid rejecting image uploads.
func jsonBodySizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/upload/") {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		_ = r.Body.Close()
		if len(body) > maxJSONBodyBytes {
			response.Error(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
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
// cleanup of expired tracking tokens and order archiving). Call this after the
// server is running.
func (s *Server) StartBackgroundJobs(ctx context.Context) {
	jobCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		cleanup := func() {
			lockCtx, cancel := context.WithTimeout(jobCtx, 5*time.Second)
			locked, release, err := database.TryAdvisoryLock(lockCtx, s.db, database.AdvisoryLockKey("ishos:hourly-cleanup"))
			cancel()
			if err != nil {
				log.Warn().Err(err).Msg("Advisory lock attempt for cleanup failed")
				return
			}
			if !locked {
				log.Debug().Msg("Hourly cleanup skipped; advisory lock held by another instance")
				return
			}
			defer release()

			now := time.Now()
			if s.trackingTokenStore != nil {
				if err := s.trackingTokenStore.CleanupExpired(jobCtx); err != nil {
					log.Warn().Err(err).Msg("Tracking-token cleanup failed")
				}
			}
			if s.revokedTokenStore != nil {
				if _, err := s.revokedTokenStore.CleanupExpired(jobCtx, now); err != nil {
					log.Warn().Err(err).Msg("Revoked-token cleanup failed")
				}
			}
			if s.orderStore != nil {
				if _, err := s.orderStore.CleanupExpiredIdempotencyKeys(jobCtx, now); err != nil {
					log.Warn().Err(err).Msg("Idempotency-key cleanup failed")
				}
			}
		}

		// Run once shortly after startup to clean any backlog.
		cleanup()

		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()

	if s.archiver != nil {
		archCtx, archCancel := context.WithCancel(ctx)
		s.archiverCancel = archCancel
		go s.archiver.Start(archCtx)
	}
}

// Close gracefully shuts down background jobs, the event dispatcher, the
// WhatsApp worker, and the database connection.
func (s *Server) Close() {
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	if s.archiverCancel != nil {
		s.archiverCancel()
	}

	if s.dispatcher != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.dispatcher.Close(ctx); err != nil {
			log.Warn().Err(err).Msg("Event dispatcher close timed out or failed")
		}
		cancel()
	}

	if s.whatsapp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.whatsapp.Close(ctx); err != nil {
			log.Warn().Err(err).Msg("WhatsApp service close timed out or failed")
		}
		cancel()
	}

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database connection")
		} else {
			log.Info().Msg("Database connection closed")
		}
	}
}
