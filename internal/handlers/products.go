// products.go — Product CRUD handlers, including flavor/addon linking and availability toggle.
// Public users see only available products; admins can manage all products and associations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type ProductHandler struct {
	cfg            *config.Config
	productService *services.ProductService
}

func NewProductHandler(db *ent.Client, cfg *config.Config) *ProductHandler {
	productRepo := repository.NewProductRepository(db)
	return &ProductHandler{
		cfg:            cfg,
		productService: services.NewProductService(productRepo),
	}
}

// productResponse is what the API returns to the Astro frontend.
// image_url is built from the stored image_path + configured public object URL.
type productResponse struct {
	ID           uuid.UUID        `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Price        float64          `json:"price"`
	CategoryID   uuid.UUID        `json:"category_id"`
	CategoryName string           `json:"category_name"`
	ImagePath    *string          `json:"image_path,omitempty"`
	ImageURL     *string          `json:"image_url"` // full public object URL
	IsAvailable  bool             `json:"is_available"`
	Flavors      []flavorResponse `json:"flavors"`
	Addons       []addonResponse  `json:"addons"`
	CreatedAt    time.Time        `json:"created_at"`
}

// buildImageURL converts a stored path like "products/chocolate.jpg"
// into the full public object URL.
func (h *ProductHandler) buildImageURL(path *string) *string {
	if path == nil {
		return nil
	}
	url := h.cfg.PublicObjectURL(*path)
	return &url
}

// List returns all products.
// Public users see only available, in-stock products.
// Admin/employees see everything (checked via query param ?all=true + auth).
//
// GET /api/v1/products
// GET /api/v1/products?category=gelatos
// GET /api/v1/products?available=true
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	products, err := h.productService.List(ctx, r.URL.Query().Get("category"), r.URL.Query().Get("all") == "true")
	if err != nil {
		log.Error().Err(err).Bool("ctx_canceled", errors.Is(err, context.Canceled)).Bool("ctx_deadline", errors.Is(err, context.DeadlineExceeded)).Msg("Failed to list products")
		response.Error(w, http.StatusInternalServerError, "Failed to fetch products")
		return
	}

	sort.Slice(products, func(i, j int) bool {
		return products[i].CreatedAt.Before(products[j].CreatedAt)
	})

	result := make([]productResponse, len(products))
	for i, p := range products {
		catID := uuid.UUID{}
		catName := ""
		if p.Edges.Category != nil {
			catID = p.Edges.Category.ID
			catName = p.Edges.Category.Name
		}

		flavors := make([]flavorResponse, len(p.Edges.Flavors))
		for j, f := range p.Edges.Flavors {
			flavors[j] = toFlavorResponse(f)
		}

		addons := make([]addonResponse, len(p.Edges.Addons))
		for j, a := range p.Edges.Addons {
			addons[j] = toAddonResponse(a)
		}

		result[i] = productResponse{
			ID:           p.ID,
			Name:         p.Name,
			Description:  p.Description,
			Price:        p.Price,
			CategoryID:   catID,
			CategoryName: catName,
			ImagePath:    p.ImagePath,
			ImageURL:     h.buildImageURL(p.ImagePath),
			IsAvailable:  p.IsAvailable,
			Flavors:      flavors,
			Addons:       addons,
			CreatedAt:    p.CreatedAt,
		}
	}
	response.JSON(w, http.StatusOK, result)
}

// Get returns a single product by ID.
//
// GET /api/v1/products/{id}
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	p, err := h.productService.GetByIDWithCategory(r.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch product")
		return
	}

	catID := uuid.UUID{}
	catName := ""
	if p.Edges.Category != nil {
		catID = p.Edges.Category.ID
		catName = p.Edges.Category.Name
	}

	flavors := make([]flavorResponse, len(p.Edges.Flavors))
	for i, f := range p.Edges.Flavors {
		flavors[i] = toFlavorResponse(f)
	}

	addons := make([]addonResponse, len(p.Edges.Addons))
	for i, a := range p.Edges.Addons {
		addons[i] = toAddonResponse(a)
	}

	response.JSON(w, http.StatusOK, productResponse{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Price:        p.Price,
		CategoryID:   catID,
		CategoryName: catName,
		ImagePath:    p.ImagePath,
		ImageURL:     h.buildImageURL(p.ImagePath),
		IsAvailable:  p.IsAvailable,
		Flavors:      flavors,
		Addons:       addons,
		CreatedAt:    p.CreatedAt,
	})
}

// Create adds a new product. Admin only (enforced by router middleware).
//
// POST /api/v1/products
// Body: { "name", "description", "price", "category_id", "image_path" }
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Price       float64   `json:"price"`
		CategoryID  uuid.UUID `json:"category_id"`
		ImagePath   *string   `json:"image_path"`
		StockStatus *string   `json:"stock_status"`
		IsAvailable *bool     `json:"is_available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	stockStatus := normalizeStockStatus(body.StockStatus, body.IsAvailable)
	if errs := validator.ValidateProductCreate(body.Name, body.Description, body.Price, stockStatus); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	p, err := h.productService.Create(r.Context(), repository.CreateProductInput{
		Name:        body.Name,
		Description: body.Description,
		Price:       body.Price,
		CategoryID:  body.CategoryID,
		ImagePath:   body.ImagePath,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create product")
		response.Error(w, http.StatusInternalServerError, "Failed to create product")
		return
	}

	response.JSON(w, http.StatusCreated, p)
}

func normalizeStockStatus(stockStatus *string, isAvailable *bool) string {
	if stockStatus != nil {
		value := strings.TrimSpace(*stockStatus)
		if value != "" {
			return value
		}
	}

	if isAvailable != nil && !*isAvailable {
		return "out_of_stock"
	}

	return "in_stock"
}

// Update modifies a product. Admin only.
//
// PATCH /api/v1/products/{id}
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var body struct {
		Name        *string    `json:"name"`
		Description *string    `json:"description"`
		Price       *float64   `json:"price"`
		CategoryID  *uuid.UUID `json:"category_id"`
		ImagePath   *string    `json:"image_path"`
		IsAvailable *bool      `json:"is_available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateProductUpdate(body.Name, body.Price, nil); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	p, err := h.productService.Update(r.Context(), id, repository.UpdateProductInput{
		Name:        body.Name,
		Description: body.Description,
		Price:       body.Price,
		CategoryID:  body.CategoryID,
		ImagePath:   body.ImagePath,
		IsAvailable: body.IsAvailable,
	})
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update product")
		return
	}
	response.JSON(w, http.StatusOK, p)
}

// Delete removes a product. Admin only.
//
// DELETE /api/v1/products/{id}
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	if err := h.productService.Delete(r.Context(), id); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			response.Error(w, http.StatusConflict, "Cannot delete product with existing order history. Mark it unavailable instead")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to delete product")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"message": "Product deleted"})
}

// ToggleAvailability flips is_available. Admin only.
//
// PATCH /api/v1/products/{id}/availability
func (h *ProductHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	updated, err := h.productService.ToggleAvailability(r.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to toggle availability")
		return
	}
	response.JSON(w, http.StatusOK, updated)
}

// LinkFlavor adds a flavor to a product. Admin only.
//
// POST /api/v1/products/{id}/flavors/{flavorId}
func (h *ProductHandler) LinkFlavor(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	flavorID, err := uuid.Parse(chi.URLParam(r, "flavorId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid flavor ID")
		return
	}

	if err := h.productService.LinkFlavor(r.Context(), productID, flavorID); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product or flavor not found")
			return
		}
		log.Error().Err(err).Msg("Failed to link flavor to product")
		response.Error(w, http.StatusInternalServerError, "Failed to link flavor")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Flavor linked"})
}

// UnlinkFlavor removes a flavor from a product. Admin only.
//
// DELETE /api/v1/products/{id}/flavors/{flavorId}
func (h *ProductHandler) UnlinkFlavor(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	flavorID, err := uuid.Parse(chi.URLParam(r, "flavorId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid flavor ID")
		return
	}

	if err := h.productService.UnlinkFlavor(r.Context(), productID, flavorID); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product or flavor not found")
			return
		}
		log.Error().Err(err).Msg("Failed to unlink flavor from product")
		response.Error(w, http.StatusInternalServerError, "Failed to unlink flavor")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Flavor unlinked"})
}

// LinkAddon adds an addon to a product. Admin only.
//
// POST /api/v1/products/{id}/addons/{addonId}
func (h *ProductHandler) LinkAddon(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	addonID, err := uuid.Parse(chi.URLParam(r, "addonId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid addon ID")
		return
	}

	if err := h.productService.LinkAddon(r.Context(), productID, addonID); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product or addon not found")
			return
		}
		log.Error().Err(err).Msg("Failed to link addon to product")
		response.Error(w, http.StatusInternalServerError, "Failed to link addon")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Addon linked"})
}

// UnlinkAddon removes an addon from a product. Admin only.
//
// DELETE /api/v1/products/{id}/addons/{addonId}
func (h *ProductHandler) UnlinkAddon(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	addonID, err := uuid.Parse(chi.URLParam(r, "addonId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid addon ID")
		return
	}

	if err := h.productService.UnlinkAddon(r.Context(), productID, addonID); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Product or addon not found")
			return
		}
		log.Error().Err(err).Msg("Failed to unlink addon from product")
		response.Error(w, http.StatusInternalServerError, "Failed to unlink addon")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Addon unlinked"})
}
