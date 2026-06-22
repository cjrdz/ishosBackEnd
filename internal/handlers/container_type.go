package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type ContainerTypeHandler struct {
	service services.ContainerTypeServiceInterface
}

func NewContainerTypeHandler(svc services.ContainerTypeServiceInterface) *ContainerTypeHandler {
	return &ContainerTypeHandler{
		service: svc,
	}
}

// List returns all container types.
func (h *ContainerTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// Get returns a single container type.
func (h *ContainerTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid container type ID")
		return
	}

	item, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Container type not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch container type")
		return
	}

	response.JSON(w, http.StatusOK, item)
}

// Create adds a new container type.
func (h *ContainerTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name              string `json:"name"`
		BallsPerContainer int    `json:"balls_per_container"`
		IsCustom          bool   `json:"is_custom"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Name == "" {
		response.Error(w, http.StatusBadRequest, "Name is required")
		return
	}

	item, err := h.service.Create(r.Context(), body.Name, body.BallsPerContainer, body.IsCustom)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create container type")
		return
	}

	response.JSON(w, http.StatusCreated, item)
}

// Update modifies a container type.
func (h *ContainerTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid container type ID")
		return
	}

	var body struct {
		Name              *string `json:"name"`
		BallsPerContainer *int    `json:"balls_per_container"`
		IsCustom          *bool   `json:"is_custom"`
		IsActive          *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	item, err := h.service.Update(r.Context(), id, body.Name, body.BallsPerContainer, body.IsCustom, body.IsActive)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Container type not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update container type")
		return
	}

	response.JSON(w, http.StatusOK, item)
}

// Delete removes a container type.
func (h *ContainerTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid container type ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Container type not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to delete container type")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Container type deleted"})
}
