package handlers

import (
	"net/http"

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
			AuthCookieTTLHours:    cfg.AuthCookieTTLHours,
			AuthTokenTTLHours:     cfg.AuthTokenTTLHours,
			TrackingTokenTTLHours: cfg.TrackingTokenTTLHours,
		},
	}
}

func (h *AdminSettingsHandler) GetTabs(w http.ResponseWriter, r *http.Request) {
	order, err := h.settingsSvc.GetTabOrder(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuracion")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tab_order": order,
	})
}

func (h *AdminSettingsHandler) UpdateTabs(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TabOrder []string `json:"tab_order"`
	}

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud invalido")
		return
	}

	order, err := h.settingsSvc.UpdateTabOrder(r.Context(), body.TabOrder)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "No se pudo guardar configuracion")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tab_order": order,
	})
}

func (h *AdminSettingsHandler) GetPanelConfig(w http.ResponseWriter, r *http.Request) {
	configValues, err := h.settingsSvc.GetTokenTTLConfig(r.Context(), h.defaults)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar configuracion")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"auth_cookie_ttl_hours":    configValues.AuthCookieTTLHours,
		"auth_token_ttl_hours":     configValues.AuthTokenTTLHours,
		"tracking_token_ttl_hours": configValues.TrackingTokenTTLHours,
	})
}

func (h *AdminSettingsHandler) UpdatePanelConfig(w http.ResponseWriter, r *http.Request) {
	var body services.TokenTTLConfig

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud invalido")
		return
	}

	configValues, err := h.settingsSvc.UpdateTokenTTLConfig(r.Context(), body, h.defaults)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "No se pudo guardar configuracion")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"auth_cookie_ttl_hours":    configValues.AuthCookieTTLHours,
		"auth_token_ttl_hours":     configValues.AuthTokenTTLHours,
		"tracking_token_ttl_hours": configValues.TrackingTokenTTLHours,
	})
}
