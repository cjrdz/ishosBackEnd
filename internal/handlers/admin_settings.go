package handlers

import (
	"net/http"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AdminSettingsHandler struct {
	settingsSvc *services.AdminSettingService
}

func NewAdminSettingsHandler(db *ent.Client) *AdminSettingsHandler {
	repo := repository.NewAdminSettingRepository(db)
	return &AdminSettingsHandler{settingsSvc: services.NewAdminSettingService(repo)}
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
