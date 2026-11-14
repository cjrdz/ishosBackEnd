// inventory.go — Inventory management handlers (admin only).
package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type InventoryHandler struct {
	inventory *services.InventoryService
	auditSvc  audit.Service
}

func NewInventoryHandler(inventory *services.InventoryService, auditSvc audit.Service) *InventoryHandler {
	return &InventoryHandler{inventory: inventory, auditSvc: auditSvc}
}

// List returns all inventory items.
func (h *InventoryHandler) List(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	items, err := h.inventory.List(r.Context(), loc)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// Dashboard returns flavor inventory, unit inventory and stats in a single
// response so the admin UI can render the inventory tab with one request.
func (h *InventoryHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	dashboard, err := h.inventory.GetInventoryDashboard(r.Context(), loc)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, dashboard)
}

// Get returns a single inventory item with its movement history.
func (h *InventoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	item, err := h.inventory.GetByID(r.Context(), loc, id)
	if err != nil {
		writeAppError(w, err, "Failed to fetch inventory item")
		return
	}

	response.JSON(w, http.StatusOK, item)
}

// RecordEntry records a stock entry.
func (h *InventoryHandler) RecordEntry(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	locationID, err := loc.Require()
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
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
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	createdByID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, createdByID)

	movement, err := h.inventory.RecordEntry(ctx, locationID, id, body.ContainerTypeID, body.QuantityContainers, body.BallsPerContainer, createdByID)
	if err != nil {
		writeAppError(w, err, "Failed to record entry")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionInventoryEntryRecorded, createdByID, fmt.Sprintf("inventory_item:%s", id.String()), map[string]any{
		"movement_id": movement.ID,
		"quantity":    body.QuantityContainers,
	}, ip)

	response.JSON(w, http.StatusCreated, movement)
}

// RecordAdjustment records a manual adjustment.
func (h *InventoryHandler) RecordAdjustment(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	var body struct {
		Quantity int    `json:"quantity"`
		Reason   string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Reason == "" {
		response.Error(w, http.StatusBadRequest, "Reason is required for adjustments")
		return
	}

	createdByID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, createdByID)

	movement, err := h.inventory.RecordAdjustment(ctx, loc, id, body.Quantity, body.Reason, createdByID)
	if err != nil {
		writeAppError(w, err, "Failed to record adjustment")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionInventoryAdjustmentRecorded, createdByID, fmt.Sprintf("inventory_item:%s", id.String()), map[string]any{
		"movement_id": movement.ID,
		"quantity":    body.Quantity,
		"reason":      body.Reason,
	}, ip)

	response.JSON(w, http.StatusCreated, movement)
}

// GetLowStock returns low stock items.
func (h *InventoryHandler) GetLowStock(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	items, err := h.inventory.GetLowStockItems(r.Context(), loc)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// GetMovementHistory returns stock movements for an inventory item.
func (h *InventoryHandler) GetMovementHistory(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	const maxLimit = 200

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			if parsed > maxLimit {
				parsed = maxLimit
			}
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	movements, total, err := h.inventory.GetMovementHistory(r.Context(), loc, id, limit, offset)
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
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	locationID, err := loc.Require()
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid flavor ID")
		return
	}

	stock, err := h.inventory.GetStockForFlavor(r.Context(), locationID, id)
	if err != nil {
		writeAppError(w, err, "Failed to get stock status")
		return
	}

	response.JSON(w, http.StatusOK, stock)
}

// ListFlavorInventory returns only ball-based items linked to flavors.
func (h *InventoryHandler) ListFlavorInventory(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	items, err := h.inventory.GetFlavorInventory(r.Context(), loc)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// ListUnitInventory returns only unit-based items linked to products.
func (h *InventoryHandler) ListUnitInventory(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	items, err := h.inventory.GetUnitInventory(r.Context(), loc)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, response.NewPaginated(items, 1, len(items), len(items)))
}

// RecordUnitEntry records a stock entry for a unit-based product.
func (h *InventoryHandler) RecordUnitEntry(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	createdByID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, createdByID)

	movement, err := h.inventory.RecordUnitEntry(ctx, loc, id, body.Quantity, createdByID)
	if err != nil {
		writeAppError(w, err, "Failed to record unit entry")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionInventoryUnitEntryRecorded, createdByID, fmt.Sprintf("inventory_item:%s", id.String()), map[string]any{
		"movement_id": movement.ID,
		"quantity":    body.Quantity,
	}, ip)

	response.JSON(w, http.StatusCreated, movement)
}

// CreateUnitInventoryItem creates a new unit-based inventory item.
func (h *InventoryHandler) CreateUnitInventoryItem(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	var body struct {
		Name              string    `json:"name"`
		ProductID         uuid.UUID `json:"product_id"`
		LowStockThreshold int       `json:"low_stock_threshold"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Name == "" || body.ProductID == uuid.Nil {
		response.Error(w, http.StatusBadRequest, "Name and product_id are required")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	created, err := h.inventory.CreateUnitInventoryItem(ctx, body.Name, body.ProductID, body.LowStockThreshold)
	if err != nil {
		writeAppError(w, err, "Failed to create inventory item")
		return
	}
	if len(created) == 0 {
		response.Error(w, http.StatusConflict, "Ya se lleva inventario de este artículo en todas las sucursales")
		return
	}
	// Answer with the row of the location being viewed (or the first one).
	item := created[0]
	for _, c := range created {
		if id, one := loc.ID(); one && c.LocationID == id {
			item = c
		}
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionInventoryUnitItemCreated, actorID, fmt.Sprintf("inventory_item:%s", item.ID.String()), map[string]any{
		"name":                item.Name,
		"product_id":          body.ProductID.String(),
		"low_stock_threshold": item.LowStockThreshold,
	}, ip)

	response.JSON(w, http.StatusCreated, item)
}

// GetStats returns aggregate inventory statistics.
func (h *InventoryHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	stats, err := h.inventory.GetInventoryStats(r.Context(), loc)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, stats)
}

// LinkInventoryItemToProduct links an inventory item to a product.
func (h *InventoryHandler) LinkInventoryItemToProduct(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
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

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	item, err := h.inventory.LinkInventoryItemToProduct(ctx, loc, inventoryItemID, productID)
	if err != nil {
		writeAppError(w, err, "Failed to link inventory item")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionInventoryItemLinked, actorID, fmt.Sprintf("inventory_item:%s", inventoryItemID.String()), map[string]any{
		"product_id": productID.String(),
	}, ip)

	response.JSON(w, http.StatusOK, item)
}

// UnlinkInventoryItemFromProduct removes the product link from an inventory item.
func (h *InventoryHandler) UnlinkInventoryItemFromProduct(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	inventoryItemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid inventory item ID")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	item, err := h.inventory.UnlinkInventoryItemFromProduct(ctx, loc, inventoryItemID)
	if err != nil {
		writeAppError(w, err, "Failed to unlink inventory item")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionInventoryItemUnlinked, actorID, fmt.Sprintf("inventory_item:%s", inventoryItemID.String()), nil, ip)

	response.JSON(w, http.StatusOK, item)
}
