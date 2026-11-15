// products.go — Product CRUD handlers, including flavor/addon linking and availability toggle.
// Public users see only available products; admins can manage all products and associations.
package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type ProductHandler struct {
	cfg              *config.Config
	productService   services.ProductServiceInterface
	inventoryService *services.InventoryService
	auditSvc         audit.Service
	responseCache    *cache.TTLCache
}

func NewProductHandler(svc services.ProductServiceInterface, cfg *config.Config, inventoryService *services.InventoryService, auditSvc audit.Service) *ProductHandler {
	return &ProductHandler{
		cfg:              cfg,
		productService:   svc,
		inventoryService: inventoryService,
		auditSvc:         auditSvc,
		responseCache:    cache.New(5 * time.Minute),
	}
}

// productResponse is what the API returns to the Astro frontend.
type productResponse struct {
	ID                   uuid.UUID        `json:"id"`
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	Price                float64          `json:"price"`
	CategoryID           uuid.UUID        `json:"category_id"`
	CategoryName         string           `json:"category_name"`
	ImagePath            *string          `json:"image_path,omitempty"`
	ImageURL             *string          `json:"image_url"`
	IsAvailable          bool             `json:"is_available"`
	ExcludeGlobalFlavors bool             `json:"exclude_global_flavors"`
	ExcludeGlobalAddons  bool             `json:"exclude_global_addons"`
	BallBased            bool             `json:"ball_based"`
	BallQuantity         int              `json:"ball_quantity"`
	AllowsMixedFlavors   bool             `json:"allows_mixed_flavors"`
	FreeToppings         int              `json:"free_toppings"`
	StockStatus          string           `json:"stock_status"`
	Flavors              []flavorResponse `json:"flavors"`
	Addons               []addonResponse  `json:"addons"`
	CreatedAt            time.Time        `json:"created_at"`
}

func (h *ProductHandler) invalidateListCache() {
	h.responseCache.InvalidateAll()
}

func (h *ProductHandler) buildImageURL(path *string) *string {
	if path == nil {
		return nil
	}
	url := h.cfg.PublicObjectURL(*path)
	return &url
}

func (h *ProductHandler) toProductResponse(p dto.Product) productResponse {
	catID := uuid.UUID{}
	catName := ""
	if p.Category != nil {
		catID = p.Category.ID
		catName = p.Category.Name
	}

	flavors := make([]flavorResponse, len(p.Flavors))
	for i, f := range p.Flavors {
		flavors[i] = flavorResponse{
			ID:           f.ID,
			Name:         f.Name,
			DisplayOrder: f.DisplayOrder,
			IsSeasonal:   f.IsSeasonal,
			IsActive:     f.IsActive,
			CreatedAt:    f.CreatedAt,
		}
	}

	addons := make([]addonResponse, len(p.Addons))
	for i, a := range p.Addons {
		addons[i] = addonResponse{
			ID:           a.ID,
			Name:         a.Name,
			Price:        a.Price,
			GroupName:    a.GroupName,
			DisplayOrder: a.DisplayOrder,
			IsActive:     a.IsActive,
			CreatedAt:    a.CreatedAt,
		}
	}

	stockStatus := "not_tracked"
	if len(p.InventoryItems) > 0 {
		item := p.InventoryItems[0]
		switch services.StockLevel(item.CurrentStock, item.LowStockThreshold) {
		case "out":
			stockStatus = "out_of_stock"
		case "low":
			stockStatus = "low_stock"
		default:
			stockStatus = "in_stock"
		}
	}

	return productResponse{
		ID:                   p.ID,
		Name:                 p.Name,
		Description:          p.Description,
		Price:                p.Price,
		CategoryID:           catID,
		CategoryName:         catName,
		ImagePath:            p.ImagePath,
		ImageURL:             h.buildImageURL(p.ImagePath),
		IsAvailable:          p.IsAvailable,
		ExcludeGlobalFlavors: p.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  p.ExcludeGlobalAddons,
		BallBased:            p.BallBased,
		BallQuantity:         p.BallQuantity,
		AllowsMixedFlavors:   p.AllowsMixedFlavors,
		FreeToppings:         p.FreeToppings,
		StockStatus:          stockStatus,
		Flavors:              flavors,
		Addons:               addons,
		CreatedAt:            p.CreatedAt,
	}
}

// List returns products, paginated by default. Use ?all=true for the full list
// (backward-compatible admin behavior).
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	includeAll := r.URL.Query().Get("all") == "true"

	ctx := r.Context()
	var result []productResponse
	var page, perPage, total int

	if includeAll {
		cacheKey := fmt.Sprintf("products:%s:%t", category, includeAll)
		if cached, ok := h.responseCache.Get(cacheKey); ok {
			response.JSON(w, http.StatusOK, cached)
			return
		}

		products, err := h.productService.List(ctx, category, true)
		if err != nil {
			response.WriteOrError(w, nil, err)
			return
		}

		sort.Slice(products, func(i, j int) bool {
			return products[i].CreatedAt.Before(products[j].CreatedAt)
		})

		result = make([]productResponse, len(products))
		for i, p := range products {
			result[i] = h.toProductResponse(p)
		}

		page = 1
		perPage = len(result)
		total = len(result)

		paginated := response.NewPaginated(result, page, perPage, total)
		h.responseCache.Set(cacheKey, paginated)
		response.JSON(w, http.StatusOK, paginated)
		return
	}

	params := response.ParsePaginationParams(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	products, total, err := h.productService.ListPaginated(ctx, category, false, params.Page, params.PerPage)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	result = make([]productResponse, len(products))
	for i, p := range products {
		result[i] = h.toProductResponse(p)
	}

	response.JSON(w, http.StatusOK, response.NewPaginated(result, params.Page, params.PerPage, total))
}

// Get returns a single product by ID.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	p, err := h.productService.GetByIDWithCategory(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch product")
		return
	}

	response.JSON(w, http.StatusOK, h.toProductResponse(p))
}

// Create adds a new product. Admin only.
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name                 string    `json:"name"`
		Description          string    `json:"description"`
		Price                float64   `json:"price"`
		CategoryID           uuid.UUID `json:"category_id"`
		ImagePath            *string   `json:"image_path"`
		StockStatus          *string   `json:"stock_status"`
		IsAvailable          *bool     `json:"is_available"`
		ExcludeGlobalFlavors *bool     `json:"exclude_global_flavors"`
		ExcludeGlobalAddons  *bool     `json:"exclude_global_addons"`
		BallBased            *bool     `json:"ball_based"`
		BallQuantity         *int      `json:"ball_quantity"`
		AllowsMixedFlavors   *bool     `json:"allows_mixed_flavors"`
		FreeToppings         *int      `json:"free_toppings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	stockStatus := normalizeStockStatus(body.StockStatus, body.IsAvailable)
	if errs := validator.ValidateProductCreate(body.Name, body.Description, body.Price, stockStatus); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	h.invalidateListCache()
	p, err := h.productService.Create(ctx, repository.CreateProductInput{
		Name:                 body.Name,
		Description:          body.Description,
		Price:                body.Price,
		CategoryID:           body.CategoryID,
		ImagePath:            body.ImagePath,
		ExcludeGlobalFlavors: body.ExcludeGlobalFlavors != nil && *body.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  body.ExcludeGlobalAddons != nil && *body.ExcludeGlobalAddons,
		BallBased:            body.BallBased,
		BallQuantity:         body.BallQuantity,
		AllowsMixedFlavors:   body.AllowsMixedFlavors,
		FreeToppings:         body.FreeToppings,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create product")
		response.Error(w, http.StatusInternalServerError, "Failed to create product")
		return
	}

	// Auto-create inventory item for unit-based products
	if body.BallBased != nil && !*body.BallBased {
		_, invErr := h.inventoryService.CreateUnitInventoryItem(ctx, p.Name, p.ID, 10)
		if invErr != nil {
			log.Warn().Err(invErr).Str("product", p.Name).Msg("Failed to auto-create inventory item for product")
		}
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionProductCreated, actorID, fmt.Sprintf("product:%s", p.ID.String()), map[string]any{
		"name":  p.Name,
		"price": p.Price,
	}, ip)

	response.JSON(w, http.StatusCreated, h.toProductResponse(p))
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
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var body struct {
		Name                 *string    `json:"name"`
		Description          *string    `json:"description"`
		Price                *float64   `json:"price"`
		CategoryID           *uuid.UUID `json:"category_id"`
		ImagePath            *string    `json:"image_path"`
		IsAvailable          *bool      `json:"is_available"`
		ExcludeGlobalFlavors *bool      `json:"exclude_global_flavors"`
		ExcludeGlobalAddons  *bool      `json:"exclude_global_addons"`
		BallBased            *bool      `json:"ball_based"`
		BallQuantity         *int       `json:"ball_quantity"`
		AllowsMixedFlavors   *bool      `json:"allows_mixed_flavors"`
		FreeToppings         *int       `json:"free_toppings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateProductUpdate(body.Name, body.Price, nil); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	h.invalidateListCache()
	p, err := h.productService.Update(ctx, id, repository.UpdateProductInput{
		Name:                 body.Name,
		Description:          body.Description,
		Price:                body.Price,
		CategoryID:           body.CategoryID,
		ImagePath:            body.ImagePath,
		IsAvailable:          body.IsAvailable,
		ExcludeGlobalFlavors: body.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  body.ExcludeGlobalAddons,
		BallBased:            body.BallBased,
		BallQuantity:         body.BallQuantity,
		AllowsMixedFlavors:   body.AllowsMixedFlavors,
		FreeToppings:         body.FreeToppings,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update product")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionProductUpdated, actorID, fmt.Sprintf("product:%s", id.String()), map[string]any{
		"name": p.Name,
	}, ip)

	response.JSON(w, http.StatusOK, h.toProductResponse(p))
}

// Delete removes a product. Admin only.
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	h.invalidateListCache()
	if err := h.productService.Delete(ctx, id); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			response.Error(w, http.StatusConflict, "Cannot delete product with existing order history. Mark it unavailable instead")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to delete product")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionProductDeleted, actorID, fmt.Sprintf("product:%s", id.String()), nil, ip)

	response.JSON(w, http.StatusOK, map[string]string{"message": "Product deleted"})
}

// ToggleAvailability flips is_available. Open to staff (sold-out items during service).
func (h *ProductHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	h.invalidateListCache()
	updated, err := h.productService.ToggleAvailability(ctx, id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Product not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to toggle availability")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionProductAvailabilityToggled, actorID, fmt.Sprintf("product:%s", id.String()), map[string]any{
		"is_available": updated.IsAvailable,
	}, ip)

	response.JSON(w, http.StatusOK, h.toProductResponse(updated))
}

// LinkFlavor adds a flavor to a product. Admin only.
func (h *ProductHandler) LinkFlavor(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusGone, "Legacy product-flavor linking is disabled. Use product exclusion flags with global flavors instead")
}

// UnlinkFlavor removes a flavor from a product. Admin only.
func (h *ProductHandler) UnlinkFlavor(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusGone, "Legacy product-flavor unlinking is disabled. Use product exclusion flags with global flavors instead")
}

// LinkAddon adds an addon to a product. Admin only.
func (h *ProductHandler) LinkAddon(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusGone, "Legacy product-addon linking is disabled. Use product exclusion flags with global addons instead")
}

// UnlinkAddon removes an addon from a product. Admin only.
func (h *ProductHandler) UnlinkAddon(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusGone, "Legacy product-addon unlinking is disabled. Use product exclusion flags with global addons instead")
}
