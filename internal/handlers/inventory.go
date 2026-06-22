// inventory.go — Inventory management handlers (admin only).
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type InventoryHandler struct {
	inventory *services.InventoryService
}

func NewInventoryHandler(inventory *services.InventoryService) *InventoryHandler {
	return &InventoryHandler{inventory: inventory}
}

// List returns all inventory items.
func (h *InventoryHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.inventory.List(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// Get returns a single inventory item with its movement history.
func (h *InventoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	item, err := h.inventory.GetByID(r.Context(), id)
	if err != nil {
		writeAppError(w, err, "Failed to fetch inventory item")
		return
	}

	response.JSON(w, http.StatusOK, item)
}

// RecordEntry records a stock entry.
func (h *InventoryHandler) RecordEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	var body struct {
		ContainerTypeID    uuid.UUID `json:"container_type_id"`
		QuantityContainers int       `json:"quantity_containers"`
		BallsPerContainer  int       `json:"balls_per_container"` // optional for custom containers
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	createdByID := getUserIDFromContext(r)

	movement, err := h.inventory.RecordEntry(r.Context(), id, body.ContainerTypeID, body.QuantityContainers, body.BallsPerContainer, createdByID)
	if err != nil {
		writeAppError(w, err, "Failed to record entry")
		return
	}

	response.JSON(w, http.StatusCreated, movement)
}

// RecordAdjustment records a manual adjustment.
func (h *InventoryHandler) RecordAdjustment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	var body struct {
		Quantity int    `json:"quantity"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Reason == "" {
		response.Error(w, http.StatusBadRequest, "Reason is required for adjustments")
		return
	}

	createdByID := getUserIDFromContext(r)

	movement, err := h.inventory.RecordAdjustment(r.Context(), id, body.Quantity, body.Reason, createdByID)
	if err != nil {
		writeAppError(w, err, "Failed to record adjustment")
		return
	}

	response.JSON(w, http.StatusCreated, movement)
}

// GetLowStock returns low stock items.
func (h *InventoryHandler) GetLowStock(w http.ResponseWriter, r *http.Request) {
	items, err := h.inventory.GetLowStockItems(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// GetMovementHistory returns stock movements for an inventory item.
func (h *InventoryHandler) GetMovementHistory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	movements, total, err := h.inventory.GetMovementHistory(r.Context(), id, limit, offset)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"data":        movements,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
		"total_pages": (total + limit - 1) / limit,
	})
}

// GetStockForFlavor returns stock status for a flavor.
func (h *InventoryHandler) GetStockForFlavor(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid flavor ID")
		return
	}

	stock, err := h.inventory.GetStockForFlavor(r.Context(), id)
	if err != nil {
		writeAppError(w, err, "Failed to get stock status")
		return
	}

	response.JSON(w, http.StatusOK, stock)
}

// ListFlavorInventory returns only ball-based items linked to flavors.
func (h *InventoryHandler) ListFlavorInventory(w http.ResponseWriter, r *http.Request) {
	items, err := h.inventory.GetFlavorInventory(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// ListUnitInventory returns only unit-based items linked to products.
func (h *InventoryHandler) ListUnitInventory(w http.ResponseWriter, r *http.Request) {
	items, err := h.inventory.GetUnitInventory(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// RecordUnitEntry records a stock entry for a unit-based product.
func (h *InventoryHandler) RecordUnitEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	createdByID := getUserIDFromContext(r)

	movement, err := h.inventory.RecordUnitEntry(r.Context(), id, body.Quantity, createdByID)
	if err != nil {
		writeAppError(w, err, "Failed to record unit entry")
		return
	}

	response.JSON(w, http.StatusCreated, movement)
}

// CreateUnitInventoryItem creates a new unit-based inventory item.
func (h *InventoryHandler) CreateUnitInventoryItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name              string    `json:"name"`
		ProductID         uuid.UUID `json:"product_id"`
		LowStockThreshold int       `json:"low_stock_threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Name == "" || body.ProductID == uuid.Nil {
		response.Error(w, http.StatusBadRequest, "Name and product_id are required")
		return
	}

	item, err := h.inventory.CreateUnitInventoryItem(r.Context(), body.Name, body.ProductID, body.LowStockThreshold)
	if err != nil {
		writeAppError(w, err, "Failed to create inventory item")
		return
	}

	response.JSON(w, http.StatusCreated, item)
}

// GetStats returns aggregate inventory statistics.
func (h *InventoryHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.inventory.GetInventoryStats(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, stats)
}

// LinkInventoryItemToProduct links an inventory item to a product.
func (h *InventoryHandler) LinkInventoryItemToProduct(w http.ResponseWriter, r *http.Request) {
	inventoryItemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	item, err := h.inventory.LinkInventoryItemToProduct(r.Context(), inventoryItemID, productID)
	if err != nil {
		writeAppError(w, err, "Failed to link inventory item")
		return
	}

	response.JSON(w, http.StatusOK, item)
}

// UnlinkInventoryItemFromProduct removes the product link from an inventory item.
func (h *InventoryHandler) UnlinkInventoryItemFromProduct(w http.ResponseWriter, r *http.Request) {
	inventoryItemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	item, err := h.inventory.UnlinkInventoryItemFromProduct(r.Context(), inventoryItemID)
	if err != nil {
		writeAppError(w, err, "Failed to unlink inventory item")
		return
	}

	response.JSON(w, http.StatusOK, item)
}

func getUserIDFromContext(r *http.Request) string {
	// This is a placeholder - implement based on your auth middleware
	return ""
}
