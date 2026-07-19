package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type ContainerTypeHandler struct {
	service  services.ContainerTypeServiceInterface
	auditSvc audit.Service
}

func NewContainerTypeHandler(svc services.ContainerTypeServiceInterface, auditSvc audit.Service) *ContainerTypeHandler {
	return &ContainerTypeHandler{
		service:  svc,
		auditSvc: auditSvc,
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
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Name == "" {
		response.Error(w, http.StatusBadRequest, "Name is required")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	item, err := h.service.Create(ctx, body.Name, body.BallsPerContainer, body.IsCustom)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create container type")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionContainerTypeCreated, actorID, fmt.Sprintf("container_type:%s", item.ID.String()), map[string]any{
		"name":                item.Name,
		"balls_per_container": item.BallsPerContainer,
		"is_custom":           item.IsCustom,
	}, ip)

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
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	item, err := h.service.Update(ctx, id, body.Name, body.BallsPerContainer, body.IsCustom, body.IsActive)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Container type not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update container type")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionContainerTypeUpdated, actorID, fmt.Sprintf("container_type:%s", id.String()), map[string]any{
		"name":                item.Name,
		"balls_per_container": item.BallsPerContainer,
		"is_custom":           item.IsCustom,
		"is_active":           item.IsActive,
	}, ip)

	response.JSON(w, http.StatusOK, item)
}

// Delete removes a container type.
func (h *ContainerTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid container type ID")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Container type not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to delete container type")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionContainerTypeDeleted, actorID, fmt.Sprintf("container_type:%s", id.String()), nil, ip)

	response.JSON(w, http.StatusOK, map[string]string{"message": "Container type deleted"})
}
