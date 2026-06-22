// categories.go — Category CRUD handlers.
//
// Categories organise products (e.g. "Gelatos", "Sorbetes", "Extras").
// Public users can list/get categories; only admins can create/update/delete.

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type CategoryHandler struct {
	categorySvc services.CategoryServiceInterface
}

func NewCategoryHandler(svc services.CategoryServiceInterface) *CategoryHandler {
	return &CategoryHandler{categorySvc: svc}
}

// categoryResponse is the JSON shape returned to the Astro frontend.
type categoryResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	ImagePath    *string   `json:"image_path,omitempty"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

// productSummaryResponse is a lightweight product shape embedded in category detail responses.
type productSummaryResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	ImagePath   *string   `json:"image_path,omitempty"`
	IsAvailable bool      `json:"is_available"`
	CreatedAt   time.Time `json:"created_at"`
}

// categoryDetailResponse extends categoryResponse with products for the slug lookup endpoint.
type categoryDetailResponse struct {
	categoryResponse
	Products []productSummaryResponse `json:"products"`
}

func toCategoryResponse(c dto.Category) categoryResponse {
	return categoryResponse{
		ID:           c.ID,
		Name:         c.Name,
		Slug:         c.Slug,
		Description:  c.Description,
		ImagePath:    c.ImagePath,
		DisplayOrder: c.DisplayOrder,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt,
	}
}

func toCategoryDetailResponse(c dto.Category) categoryDetailResponse {
	detail := categoryDetailResponse{
		categoryResponse: toCategoryResponse(c),
		Products:         make([]productSummaryResponse, len(c.Products)),
	}
	for i, p := range c.Products {
		detail.Products[i] = productSummaryResponse{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			ImagePath:   p.ImagePath,
			IsAvailable: p.IsAvailable,
			CreatedAt:   p.CreatedAt,
		}
	}
	return detail
}

// List returns active categories by default, ordered by display_order.
// If ?all=true is sent, it returns all categories (including inactive).
//
// GET /api/v1/categories
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	showAll := r.URL.Query().Get("all") == "true"

	cats, err := h.categorySvc.List(r.Context(), showAll)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	result := make([]categoryResponse, len(cats))
	for i, c := range cats {
		result[i] = toCategoryResponse(c)
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(result, 1, len(result), len(result)))
}

// GetBySlug returns a single category by its URL slug, with its products.
//
// GET /api/v1/categories/{slug}
func (h *CategoryHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	c, err := h.categorySvc.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Category not found")
			return
		}
		log.Error().Err(err).Str("slug", slug).Msg("Failed to fetch category")
		response.Error(w, http.StatusInternalServerError, "Failed to fetch category")
		return
	}

	response.JSON(w, http.StatusOK, toCategoryDetailResponse(c))
}

// Create adds a new category. Admin only (enforced by router middleware).
//
// POST /api/v1/categories
// Body: { "name", "slug", "display_order" }
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string  `json:"name"`
		Slug         string  `json:"slug"`
		Description  string  `json:"description"`
		ImagePath    *string `json:"image_path"`
		DisplayOrder int     `json:"display_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateCategoryCreate(body.Name, body.Slug, body.DisplayOrder); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	c, err := h.categorySvc.Create(r.Context(), repository.CreateCategoryInput{
		Name:         body.Name,
		Slug:         body.Slug,
		Description:  body.Description,
		ImagePath:    body.ImagePath,
		DisplayOrder: body.DisplayOrder,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create category")
		response.Error(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	response.JSON(w, http.StatusCreated, toCategoryResponse(c))
}

// Update modifies a category. Admin only.
//
// PATCH /api/v1/categories/{id}
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUID(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id format")
		return
	}

	var body struct {
		Name         *string `json:"name"`
		Slug         *string `json:"slug"`
		Description  *string `json:"description"`
		ImagePath    *string `json:"image_path"`
		DisplayOrder *int    `json:"display_order"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateCategoryUpdate(body.Name, body.Slug, body.DisplayOrder); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	c, err := h.categorySvc.Update(r.Context(), id, repository.UpdateCategoryInput{
		Name:         body.Name,
		Slug:         body.Slug,
		Description:  body.Description,
		ImagePath:    body.ImagePath,
		DisplayOrder: body.DisplayOrder,
		IsActive:     body.IsActive,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Category not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update category")
		response.Error(w, http.StatusInternalServerError, "Failed to update category")
		return
	}

	response.JSON(w, http.StatusOK, toCategoryResponse(c))
}

// Delete removes a category. Admin only.
// Products in this category are NOT deleted — they become uncategorised.
//
// DELETE /api/v1/categories/{id}
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUID(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid id format")
		return
	}

	if err := h.categorySvc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Category not found")
			return
		}
		log.Error().Err(err).Msg("Failed to delete category")
		response.Error(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Category deleted"})
}
