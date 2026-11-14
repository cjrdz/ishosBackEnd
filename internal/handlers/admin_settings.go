// admin_settings.go — Store settings handlers (orders kill-switch + offers).
// Storage is optional: when persistence is disabled, endpoints return defaults and accept no-op updates.
package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AdminSettingsHandler struct {
	settingsSvc *services.AdminSettingService
	locations   *services.LocationService
	auditSvc    audit.Service
}

func NewAdminSettingsHandler(settingsSvc *services.AdminSettingService, locations *services.LocationService, auditSvc audit.Service) *AdminSettingsHandler {
	return &AdminSettingsHandler{
		settingsSvc: settingsSvc,
		locations:   locations,
		auditSvc:    auditSvc,
	}
}

// GetStoreSettings returns the full store settings (orders enabled + offers).
//
// GET /api/v1/settings/store
func (h *AdminSettingsHandler) GetStoreSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsSvc.GetStoreSettings(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuración de tienda")
		return
	}

	response.JSON(w, http.StatusOK, settings)
}

// UpdateStoreSettings persists orders-enabled flag and special offers.
//
// PATCH /api/v1/settings/store
// Body: { "orders_enabled": bool, "offers": [...] }
func (h *AdminSettingsHandler) UpdateStoreSettings(w http.ResponseWriter, r *http.Request) {
	var body services.StoreSettings
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}
	adminID := getAdminID(r)
	saved, err := h.settingsSvc.UpdateStoreSettings(r.Context(), body, adminID)
	if err != nil {
		if errors.Is(err, apperror.ErrValidation) {
			response.Error(w, http.StatusBadRequest, "Revisa las ofertas: cada una necesita un artículo del menú, un precio y una fecha de vencimiento válidos")
			return
		}
		response.Error(w, http.StatusInternalServerError, "No se pudo guardar configuración de tienda")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionSettingsUpdated, actorID, "settings:store", map[string]any{
		"orders_enabled": body.OrdersEnabled,
		"offers_count":   len(body.Offers),
	}, ip)

	response.JSON(w, http.StatusOK, saved)
}

// getAdminID extracts the authenticated employee's ID from the request context.
// Returns an empty string when the request is not authenticated.
func getAdminID(r *http.Request) string {
	if record, ok := auth.GetRecord(r.Context()); ok && record != nil {
		return auth.RecordID(record)
	}
	return ""
}

// GetPanelConfig returns the persisted panel/session configuration.
//
// GET /api/v1/settings/panel-config
func (h *AdminSettingsHandler) GetPanelConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.settingsSvc.GetTokenTTLConfig(r.Context(), services.TokenTTLConfig{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuración del panel")
		return
	}
	response.JSON(w, http.StatusOK, cfg)
}

// UpdatePanelConfig persists the panel/session configuration.
//
// PATCH /api/v1/settings/panel-config
// Body: { "auth_cookie_ttl_hours": int, "auth_token_ttl_hours": int,
//
//	"tracking_token_ttl_hours": int, "inactivity_logout_seconds": int }
func (h *AdminSettingsHandler) UpdatePanelConfig(w http.ResponseWriter, r *http.Request) {
	var body services.TokenTTLConfig
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	adminID := getAdminID(r)
	saved, err := h.settingsSvc.UpdateTokenTTLConfig(r.Context(), body, adminID, services.TokenTTLConfig{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo guardar configuración del panel")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionSettingsUpdated, actorID, "settings:panel-config", map[string]any{
		"auth_cookie_ttl_hours":     saved.AuthCookieTTLHours,
		"auth_token_ttl_hours":      saved.AuthTokenTTLHours,
		"tracking_token_ttl_hours":  saved.TrackingTokenTTLHours,
		"inactivity_logout_seconds": saved.InactivityLogoutSeconds,
	}, ip)

	response.JSON(w, http.StatusOK, saved)
}

// GetPublicStoreStatus returns public-facing store status.
// No authentication required. Filters expired offers.
//
// orders_enabled is true when at least one location takes online orders;
// per-location hours and status come from GET /locations.
//
// GET /api/v1/settings/store/public
func (h *AdminSettingsHandler) GetPublicStoreStatus(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsSvc.GetStoreSettings(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar estado de tienda")
		return
	}
	locations, err := h.locations.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar estado de tienda")
		return
	}
	online := false
	for _, l := range locations {
		if l.IsActive && l.OrdersEnabled && l.OnlineOrdersEnabled {
			online = true
		}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"orders_enabled": online,
		"offers":         services.ActiveOffers(settings.Offers, time.Now()),
	})
}

// validationFields extracts per-field messages from a validation error.
func validationFields(err error) map[string]string {
	var vErr *apperror.ValidationError
	if errors.As(err, &vErr) {
		return vErr.Fields
	}
	return nil
}

// GetArchiveSettings returns the persisted order archive configuration.
//
// GET /api/v1/settings/archive
func (h *AdminSettingsHandler) GetArchiveSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.settingsSvc.GetArchiveSettings(r.Context(), services.ArchiveConfig{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuración de archivo")
		return
	}
	response.JSON(w, http.StatusOK, cfg)
}

// UpdateArchiveSettings persists the order archive configuration.
//
// PATCH /api/v1/settings/archive
// Body: { "enabled": bool, "age_days": int, "interval_minutes": int }
func (h *AdminSettingsHandler) UpdateArchiveSettings(w http.ResponseWriter, r *http.Request) {
	var body services.ArchiveConfig
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	adminID := getAdminID(r)
	saved, err := h.settingsSvc.UpdateArchiveSettings(r.Context(), body, adminID, services.ArchiveConfig{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo guardar configuración de archivo")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionSettingsUpdated, actorID, "settings:archive", map[string]any{
		"enabled":          saved.Enabled,
		"age_days":         saved.AgeDays,
		"interval_minutes": saved.IntervalMinutes,
	}, ip)

	response.JSON(w, http.StatusOK, saved)
}

// GetPublicOrderRateLimitConfig returns the public order rate-limit configuration.
//
// GET /api/v1/settings/rate-limits
func (h *AdminSettingsHandler) GetPublicOrderRateLimitConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.settingsSvc.GetPublicOrderRateLimitConfig(r.Context(), services.PublicOrderRateLimitConfig{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuración de límites")
		return
	}
	response.JSON(w, http.StatusOK, cfg)
}

// UpdatePublicOrderRateLimitConfig persists the public order rate-limit configuration.
//
// PATCH /api/v1/settings/rate-limits
// Body: { "phone_burst": int, "phone_window_minutes": int, ... }
func (h *AdminSettingsHandler) UpdatePublicOrderRateLimitConfig(w http.ResponseWriter, r *http.Request) {
	var body services.PublicOrderRateLimitConfig
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	adminID := getAdminID(r)
	saved, err := h.settingsSvc.UpdatePublicOrderRateLimitConfig(r.Context(), body, adminID, services.PublicOrderRateLimitConfig{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo guardar configuración de límites")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionSettingsUpdated, actorID, "settings:rate-limits", map[string]any{
		"phone_burst":          saved.PhoneBurst,
		"phone_window_minutes": saved.PhoneWindowMinutes,
		"phone_block_minutes":  saved.PhoneBlockMinutes,
		"ip_burst":             saved.IPBurst,
		"ip_window_minutes":    saved.IPWindowMinutes,
		"ip_block_minutes":     saved.IPBlockMinutes,
		"staff_burst":          saved.StaffBurst,
		"staff_window_minutes": saved.StaffWindowMinutes,
	}, ip)

	response.JSON(w, http.StatusOK, saved)
}
