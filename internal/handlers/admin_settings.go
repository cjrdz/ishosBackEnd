// admin_settings.go — Admin panel configuration handlers (tab order, token TTL settings, store settings).
// Settings are persisted in the database and fall back to env-var defaults when not set.
package handlers

import (
	"net/http"
	"time"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AdminSettingsHandler struct {
	settingsSvc *services.AdminSettingService
	defaults    services.TokenTTLConfig
}

func NewAdminSettingsHandler(db *ent.Client, cfg *config.Config) *AdminSettingsHandler {
	repo := repository.NewAdminSettingRepository(db)
	return &AdminSettingsHandler{
		settingsSvc: services.NewAdminSettingService(repo),
		defaults: services.TokenTTLConfig{
			AuthCookieTTLHours:      cfg.AuthCookieTTLHours,
			AuthTokenTTLHours:       cfg.AuthTokenTTLHours,
			TrackingTokenTTLHours:   cfg.TrackingTokenTTLHours,
			InactivityLogoutSeconds: cfg.InactivityLogoutSeconds,
		},
	}
}

// GetTabs returns the current admin panel tab display order.
//
// GET /api/v1/admin/settings/tabs
func (h *AdminSettingsHandler) GetTabs(w http.ResponseWriter, r *http.Request) {
	order, err := h.settingsSvc.GetTabOrder(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuración")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tab_order": order,
	})
}

// UpdateTabs persists a new tab display order for the admin panel.
//
// PATCH /api/v1/admin/settings/tabs
// Body: { "tab_order": ["orders", "products", ...] }
func (h *AdminSettingsHandler) UpdateTabs(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TabOrder []string `json:"tab_order"`
	}

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	order, err := h.settingsSvc.UpdateTabOrder(r.Context(), body.TabOrder)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "No se pudo guardar configuración")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tab_order": order,
	})
}

// GetPanelConfig returns the active token/session TTL configuration.
// Values reflect DB overrides when present; otherwise env-var defaults are returned.
//
// GET /api/v1/admin/settings/panel
func (h *AdminSettingsHandler) GetPanelConfig(w http.ResponseWriter, r *http.Request) {
	configValues, err := h.settingsSvc.GetTokenTTLConfig(r.Context(), h.defaults)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuración")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"auth_cookie_ttl_hours":     configValues.AuthCookieTTLHours,
		"auth_token_ttl_hours":      configValues.AuthTokenTTLHours,
		"tracking_token_ttl_hours":  configValues.TrackingTokenTTLHours,
		"inactivity_logout_seconds": configValues.InactivityLogoutSeconds,
	})
}

// UpdatePanelConfig overwrites token/session TTL settings in the database.
// A server restart is not required — the service re-reads these values per request.
//
// PATCH /api/v1/admin/settings/panel
func (h *AdminSettingsHandler) UpdatePanelConfig(w http.ResponseWriter, r *http.Request) {
	var body services.TokenTTLConfig

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	configValues, err := h.settingsSvc.UpdateTokenTTLConfig(r.Context(), body, h.defaults)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "No se pudo guardar configuración")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"auth_cookie_ttl_hours":     configValues.AuthCookieTTLHours,
		"auth_token_ttl_hours":      configValues.AuthTokenTTLHours,
		"tracking_token_ttl_hours":  configValues.TrackingTokenTTLHours,
		"inactivity_logout_seconds": configValues.InactivityLogoutSeconds,
	})
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

	saved, err := h.settingsSvc.UpdateStoreSettings(r.Context(), body)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo guardar configuración de tienda")
		return
	}

	response.JSON(w, http.StatusOK, saved)
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
