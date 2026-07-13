// flavors.go — Flavor CRUD handlers (ice cream flavors associated with products).
// Public users can list active flavors; only admins can create, update, or delete.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type FlavorHandler struct {
	flavorService services.FlavorServiceInterface
}

func NewFlavorHandler(svc services.FlavorServiceInterface) *FlavorHandler {
	return &FlavorHandler{flavorService: svc}
}

type flavorResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
	IsSeasonal   bool      `json:"is_seasonal"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

func toFlavorResponse(f dto.Flavor) flavorResponse {
	return flavorResponse{
		ID:           f.ID,
		Name:         f.Name,
		DisplayOrder: f.DisplayOrder,
		IsSeasonal:   f.IsSeasonal,
		IsActive:     f.IsActive,
		CreatedAt:    f.CreatedAt,
	}
}

// List returns all active flavors by default, or all flavors if ?all=true.
//
// GET /api/v1/flavors
// GET /api/v1/flavors?all=true
func (h *FlavorHandler) List(w http.ResponseWriter, r *http.Request) {
	showAll := r.URL.Query().Get("all") == "true"
	flavors, err := h.flavorService.List(r.Context(), !showAll)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	result := make([]flavorResponse, len(flavors))
	for i, f := range flavors {
		result[i] = toFlavorResponse(f)
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(result, 1, len(result), len(result)))
}

// Get returns a single flavor by ID.
//
// GET /api/v1/flavors/{id}
func (h *FlavorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUID(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	f, err := h.flavorService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Flavor not found")
			return
		}
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to fetch flavor")
		response.Error(w, http.StatusInternalServerError, "Failed to fetch flavor")
		return
	}

	response.JSON(w, http.StatusOK, toFlavorResponse(f))
}

// Create adds a new flavor. Admin only.
//
// POST /api/v1/flavors
// Body: { "name", "display_order", "is_seasonal" }
func (h *FlavorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		DisplayOrder *int   `json:"display_order"`
		IsSeasonal   bool   `json:"is_seasonal"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateFlavorCreate(body.Name); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	f, err := h.flavorService.Create(r.Context(), repository.CreateFlavorInput{
		Name:         body.Name,
		DisplayOrder: body.DisplayOrder,
		IsSeasonal:   body.IsSeasonal,
		IsActive:     true,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create flavor")
		response.Error(w, http.StatusInternalServerError, "Failed to create flavor")
		return
	}

	response.JSON(w, http.StatusCreated, toFlavorResponse(f))
}

// Update modifies a flavor. Admin only.
//
// PATCH /api/v1/flavors/{id}
func (h *FlavorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUID(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Name         *string `json:"name"`
		DisplayOrder *int    `json:"display_order"`
		IsSeasonal   *bool   `json:"is_seasonal"`
		IsActive     *bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateFlavorUpdate(body.Name); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	f, err := h.flavorService.Update(r.Context(), id, repository.UpdateFlavorInput{
		Name:         body.Name,
		DisplayOrder: body.DisplayOrder,
		IsSeasonal:   body.IsSeasonal,
		IsActive:     body.IsActive,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Flavor not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update flavor")
		response.Error(w, http.StatusInternalServerError, "Failed to update flavor")
		return
	}

	response.JSON(w, http.StatusOK, toFlavorResponse(f))
}

// Delete removes a flavor. Admin only.
//
// DELETE /api/v1/flavors/{id}
func (h *FlavorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUID(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.flavorService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Flavor not found")
			return
		}
		log.Error().Err(err).Msg("Failed to delete flavor")
		response.Error(w, http.StatusInternalServerError, "Failed to delete flavor")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Flavor deleted"})
}
