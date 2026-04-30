// admin_settings.go — Store settings handlers (orders kill-switch + offers).
// Storage is optional: when persistence is disabled, endpoints return defaults and accept no-op updates.
package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AdminSettingsHandler struct {
	settingsSvc *services.AdminSettingService
}

func NewAdminSettingsHandler(settingsSvc *services.AdminSettingService) *AdminSettingsHandler {
	return &AdminSettingsHandler{
		settingsSvc: settingsSvc,
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
	response.JSON(w, http.StatusOK, saved)
}

// getAdminID extracts the authenticated admin's UUID from the request/session.
// You must implement this based on your authentication/session logic.
func getAdminID(r *http.Request) uuid.UUID {
	// TODO: Replace with real session extraction logic
	// Example: return r.Context().Value("admin_id").(uuid.UUID)
	return uuid.Nil // placeholder
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
