// admin_settings.go — Store settings handlers (orders kill-switch + offers).
// Storage is optional: when persistence is disabled, endpoints return defaults and accept no-op updates.
package handlers

import (
	"net/http"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AdminSettingsHandler struct {
	settingsSvc *services.AdminSettingService
	auditSvc    audit.Service
}

func NewAdminSettingsHandler(settingsSvc *services.AdminSettingService, auditSvc audit.Service) *AdminSettingsHandler {
	return &AdminSettingsHandler{
		settingsSvc: settingsSvc,
		auditSvc:    auditSvc,
	}
}

// SettingsService returns the underlying service so other handlers can reference it.
func (h *AdminSettingsHandler) SettingsService() *services.AdminSettingService {
	return h.settingsSvc
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

// GetPublicStoreStatus returns public-facing store status.
// No authentication required. Filters expired offers.
//
// GET /api/v1/settings/store/public
func (h *AdminSettingsHandler) GetPublicStoreStatus(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsSvc.GetStoreSettings(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar estado de tienda")
		return
	}

	now := time.Now()
	activeOffers := make([]services.OfferItem, 0, len(settings.Offers))
	for _, o := range settings.Offers {
		if o.ExpiresAt != "" {
			t, parseErr := time.Parse(time.RFC3339, o.ExpiresAt)
			if parseErr == nil && t.Before(now) {
				continue
			}
		}
		activeOffers = append(activeOffers, o)
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"orders_enabled": settings.OrdersEnabled,
		"offers":         activeOffers,
	})
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
		"enabled":           saved.Enabled,
		"age_days":          saved.AgeDays,
		"interval_minutes":  saved.IntervalMinutes,
	}, ip)

	response.JSON(w, http.StatusOK, saved)
}
