package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AddonHandler struct {
	addonService *services.AddonService
}

func NewAddonHandler(db *ent.Client) *AddonHandler {
	addonRepo := repository.NewAddonRepository(db)
	return &AddonHandler{
		addonService: services.NewAddonService(addonRepo),
	}
}

type addonResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Price        float64   `json:"price"`
	GroupName    string    `json:"group_name"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

func toAddonResponse(a *ent.Addon) addonResponse {
	return addonResponse{
		ID:           a.ID,
		Name:         a.Name,
		Price:        a.Price,
		GroupName:    a.GroupName,
		DisplayOrder: a.DisplayOrder,
		IsActive:     a.IsActive,
		CreatedAt:    a.CreatedAt,
	}
}

// List returns all active addons by default, or all addons if ?all=true.
//
// GET /api/v1/addons
// GET /api/v1/addons?all=true
func (h *AddonHandler) List(w http.ResponseWriter, r *http.Request) {
	showAll := r.URL.Query().Get("all") == "true"
	addons, err := h.addonService.List(r.Context(), !showAll)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list addons")
		response.Error(w, http.StatusInternalServerError, "Failed to fetch addons")
		return
	}

	result := make([]addonResponse, len(addons))
	for i, a := range addons {
		result[i] = toAddonResponse(a)
	}
	response.JSON(w, http.StatusOK, result)
}

// Get returns a single addon by ID.
//
// GET /api/v1/addons/{id}
func (h *AddonHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid addon ID")
		return
	}

	a, err := h.addonService.GetByID(r.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Addon not found")
			return
		}
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to fetch addon")
		response.Error(w, http.StatusInternalServerError, "Failed to fetch addon")
		return
	}

	response.JSON(w, http.StatusOK, toAddonResponse(a))
}

// Create adds a new addon. Admin only.
//
// POST /api/v1/addons
// Body: { "name", "price", "display_order" }
func (h *AddonHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string  `json:"name"`
		Price        float64 `json:"price"`
		GroupName    string  `json:"group_name"`
		DisplayOrder int     `json:"display_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateAddonCreate(body.Name, body.Price, body.GroupName); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	groupName := body.GroupName
	if groupName == "" {
		groupName = "extras"
	}

	a, err := h.addonService.Create(r.Context(), repository.CreateAddonInput{
		Name:         body.Name,
		Price:        body.Price,
		GroupName:    groupName,
		DisplayOrder: body.DisplayOrder,
		IsActive:     true,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create addon")
		response.Error(w, http.StatusInternalServerError, "Failed to create addon")
		return
	}

	response.JSON(w, http.StatusCreated, toAddonResponse(a))
}

// Update modifies an addon. Admin only.
//
// PATCH /api/v1/addons/{id}
func (h *AddonHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid addon ID")
		return
	}

	var body struct {
		Name         *string  `json:"name"`
		Price        *float64 `json:"price"`
		GroupName    *string  `json:"group_name"`
		DisplayOrder *int     `json:"display_order"`
		IsActive     *bool    `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateAddonUpdate(body.Name, body.Price, body.GroupName); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	a, err := h.addonService.Update(r.Context(), id, repository.UpdateAddonInput{
		Name:         body.Name,
		Price:        body.Price,
		GroupName:    body.GroupName,
		DisplayOrder: body.DisplayOrder,
		IsActive:     body.IsActive,
	})
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Addon not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update addon")
		response.Error(w, http.StatusInternalServerError, "Failed to update addon")
		return
	}

	response.JSON(w, http.StatusOK, toAddonResponse(a))
}

// Delete removes an addon. Admin only.
//
// DELETE /api/v1/addons/{id}
func (h *AddonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid addon ID")
		return
	}

	if err := h.addonService.Delete(r.Context(), id); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Addon not found")
			return
		}
		log.Error().Err(err).Msg("Failed to delete addon")
		response.Error(w, http.StatusInternalServerError, "Failed to delete addon")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Addon deleted"})
}
