// inventory_service.go — Core inventory logic: stock checks, deductions, entries, adjustments.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

var (
	ErrInsufficientStock = fmt.Errorf("%w: el ajuste dejaría el inventario en negativo", apperror.ErrConflict)
	ErrInventoryNotFound = fmt.Errorf("%w: artículo de inventario no encontrado", apperror.ErrNotFound)
)

// defaultFlavorLowStockThreshold is used when a flavor inventory item is
// auto-created on the first stock entry: below 20 balls the flavour shows as
// "Stock bajo". Staff can change it per item (UpdateLowStockThreshold).
const defaultFlavorLowStockThreshold = 20

// MaxLowStockThreshold bounds the per-item minimum.
const MaxLowStockThreshold = 100000

// StockLevel classifies an item: "out" at 0 or less, "low" below its
// minimum, otherwise "ok". An item exactly at its minimum is still "ok".
func StockLevel(currentStock, lowStockThreshold int) string {
	switch {
	case currentStock <= 0:
		return "out"
	case currentStock < lowStockThreshold:
		return "low"
	default:
		return "ok"
	}
}

// UpdateLowStockThreshold sets the minimum below which an item shows as
// "Stock bajo" at its location.
func (s *InventoryService) UpdateLowStockThreshold(ctx context.Context, loc scope.Location, inventoryItemID uuid.UUID, threshold int) (dto.InventoryItem, error) {
	if threshold < 0 || threshold > MaxLowStockThreshold {
		return dto.InventoryItem{}, fmt.Errorf("%w: el mínimo debe estar entre 0 y %d", apperror.ErrValidation, MaxLowStockThreshold)
	}
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return dto.InventoryItem{}, err
	}
	return s.repo.UpdateLowStockThreshold(ctx, inventoryItemID, threshold)
}

// inventoryTypeBallBased matches the database value for ball-based inventory.
const inventoryTypeBallBased = "ball_based"

// InventoryService handles all inventory-related business logic.
type InventoryService struct {
	repo              InventoryRepository
	containerTypeRepo ContainerTypeRepository
	flavorRepo        FlavorRepository
	locationIDs       func(ctx context.Context) ([]uuid.UUID, error)
	reservingOrders   ReservingOrdersFunc
}

// ReservingStatuses are the order statuses that hold stock without having
// deducted it yet: accepted orders waiting to be prepared. Stock is deducted
// (and the reservation ends) when an order moves to en_proceso; a cancelled
// or rejected order frees it.
var ReservingStatuses = []string{"recibida"}

// ReservingOrdersFunc lists a location's orders in the given statuses with
// their items and products.
type ReservingOrdersFunc func(ctx context.Context, locationID uuid.UUID, statuses []string) ([]dto.Order, error)

// WithReservingOrders lets availability checks subtract stock already
// promised to accepted orders.
func WithReservingOrders(fn ReservingOrdersFunc) InventoryServiceOption {
	return func(s *InventoryService) { s.reservingOrders = fn }
}

// Reserved returns, per inventory item, the stock promised to accepted
// orders at locationID that have not deducted it yet, leaving out
// excludeOrderID (the order being checked).
func (s *InventoryService) Reserved(ctx context.Context, locationID, excludeOrderID uuid.UUID) (map[uuid.UUID]int, error) {
	reserved := map[uuid.UUID]int{}
	if s.reservingOrders == nil {
		return reserved, nil
	}
	orders, err := s.reservingOrders(ctx, locationID, ReservingStatuses)
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		if o.ID == excludeOrderID {
			continue
		}
		deductions, err := s.DeductionsFor(ctx, locationID, o.Items)
		if err != nil {
			return nil, err
		}
		for _, d := range deductions {
			reserved[d.InventoryItemID] += d.Quantity
		}
	}
	return reserved, nil
}

// Unavailable lists the tracked flavours and products at locationID with
// nothing left once accepted orders are served (stock minus reservations
// at or below 0).
func (s *InventoryService) Unavailable(ctx context.Context, locationID uuid.UUID) (flavors, products []uuid.UUID, err error) {
	reserved, err := s.Reserved(ctx, locationID, uuid.Nil)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.repo.ListLinked(ctx, scope.Only(locationID))
	if err != nil {
		return nil, nil, err
	}
	for _, item := range items {
		if item.CurrentStock-reserved[item.ID] > 0 {
			continue
		}
		if item.FlavorID != nil {
			flavors = append(flavors, *item.FlavorID)
		} else if item.ProductID != nil {
			products = append(products, *item.ProductID)
		}
	}
	return flavors, products, nil
}

// InventoryServiceOption configures an InventoryService.
type InventoryServiceOption func(*InventoryService)

// WithFlavorRepo enables the inventory service to look up flavor details and
// auto-create inventory items for flavors that are not yet tracked.
func WithFlavorRepo(r FlavorRepository) InventoryServiceOption {
	return func(s *InventoryService) { s.flavorRepo = r }
}

// WithLocationIDs lets the service create a tracked product's inventory row
// at every location at once.
func WithLocationIDs(fn func(ctx context.Context) ([]uuid.UUID, error)) InventoryServiceOption {
	return func(s *InventoryService) { s.locationIDs = fn }
}

// itemInScope loads an inventory item and hides it when it belongs to a
// location outside loc.
func (s *InventoryService) itemInScope(ctx context.Context, loc scope.Location, id uuid.UUID) (dto.InventoryItem, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return dto.InventoryItem{}, err
	}
	if err := loc.Check("inventory_item", item.LocationID); err != nil {
		return dto.InventoryItem{}, err
	}
	return item, nil
}

// NewInventoryService creates a new inventory service.
func NewInventoryService(repo InventoryRepository, containerTypeRepo ContainerTypeRepository, opts ...InventoryServiceOption) *InventoryService {
	s := &InventoryService{repo: repo, containerTypeRepo: containerTypeRepo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// List returns all inventory items.
func (s *InventoryService) List(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error) {
	return s.repo.List(ctx, loc)
}

// GetFlavorInventory returns only ball-based items linked to a flavor.
func (s *InventoryService) GetFlavorInventory(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error) {
	items, err := s.repo.ListByType(ctx, loc, "ball_based")
	if err != nil {
		return nil, err
	}

	result := make([]dto.InventoryItem, 0, len(items))
	for _, item := range items {
		if item.FlavorID != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

// GetUnitInventory returns only unit-based items linked to a product.
func (s *InventoryService) GetUnitInventory(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error) {
	items, err := s.repo.ListByType(ctx, loc, "unit_based")
	if err != nil {
		return nil, err
	}

	result := make([]dto.InventoryItem, 0, len(items))
	for _, item := range items {
		if item.ProductID != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

// GetByID returns a single inventory item by ID.
func (s *InventoryService) GetByID(ctx context.Context, loc scope.Location, id uuid.UUID) (dto.InventoryItem, error) {
	return s.itemInScope(ctx, loc, id)
}

// GetLowStockItems returns items below their threshold, excluding orphaned items.
func (s *InventoryService) GetLowStockItems(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error) {
	items, err := s.repo.ListLinked(ctx, loc)
	if err != nil {
		return nil, err
	}

	lowStock := make([]dto.InventoryItem, 0)
	for _, item := range items {
		if StockLevel(item.CurrentStock, item.LowStockThreshold) != "ok" {
			lowStock = append(lowStock, item)
		}
	}
	return lowStock, nil
}

// CheckStock verifies if there is enough stock for a specific flavor at a given quantity.
func (s *InventoryService) CheckStock(ctx context.Context, locationID, flavorID uuid.UUID, quantity int) (bool, error) {
	invItem, err := s.repo.FindByFlavorID(ctx, locationID, flavorID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return true, nil // No inventory tracking = always available
		}
		return false, err
	}

	return invItem.CurrentStock >= quantity, nil
}

// DeductionsFor builds the list of stock deductions required for an order's
// items without mutating stock. Untracked products are skipped silently; real
// lookup errors are returned so callers can decide how to handle them.
// Only locationID's inventory is used: each location has its own stock.
func (s *InventoryService) DeductionsFor(ctx context.Context, locationID uuid.UUID, items []dto.OrderItem) ([]repository.StockDeductionInput, error) {
	deductions := make([]repository.StockDeductionInput, 0)

	for _, item := range items {
		if item.Product == nil {
			continue
		}

		p := item.Product

		if p.BallBased {
			flavorIDs := extractFlavorIDs(item.Customizations)
			if len(flavorIDs) == 0 {
				continue
			}

			// Count how many balls of each flavor are needed.
			flavorCounts := make(map[uuid.UUID]int)
			for _, flavorID := range flavorIDs {
				flavorCounts[flavorID]++
			}
			for flavorID, count := range flavorCounts {
				invItem, err := s.repo.FindByFlavorID(ctx, locationID, flavorID)
				if err != nil {
					if errors.Is(err, apperror.ErrNotFound) {
						continue
					}
					return nil, err
				}

				deductions = append(deductions, repository.StockDeductionInput{
					InventoryItemID: invItem.ID,
					FlavorID:        &flavorID,
					Quantity:        count * item.Quantity,
				})
			}
		} else {
			invItem, err := s.repo.FindByProductID(ctx, locationID, item.ProductID)
			if err != nil {
				if errors.Is(err, apperror.ErrNotFound) {
					continue
				}
				return nil, err
			}

			deductions = append(deductions, repository.StockDeductionInput{
				InventoryItemID: invItem.ID,
				Quantity:        item.Quantity,
			})
		}
	}

	return deductions, nil
}

// stockUnit names one unit of an inventory item for messages to staff.
func stockUnit(itemType string) string {
	if itemType == inventoryTypeBallBased {
		return "bola"
	}
	return "unidad"
}

// EnsureAvailable refuses an order whose tracked flavours or products need
// more than locationID has in stock right now, so staff find out while
// taking the order instead of when they press Preparar. Untracked items are
// always available. Stock is still deducted (and checked again under a row
// lock) when the order moves to en_proceso.
//
// Stock promised to other accepted orders (Reserved) is not available.
// orderID is the order being checked (uuid.Nil for a new one): its own
// reservation is not counted against it.
//
// previous holds the order's current items when it is being edited: only
// items whose need grows are checked, so fixing the name or table of an
// accepted order is never blocked by stock that ran low in the meantime.
func (s *InventoryService) EnsureAvailable(ctx context.Context, locationID, orderID uuid.UUID, items, previous []dto.OrderItem) error {
	deductions, err := s.DeductionsFor(ctx, locationID, items)
	if err != nil {
		return err
	}
	had := map[uuid.UUID]int{}
	if len(previous) > 0 {
		before, err := s.DeductionsFor(ctx, locationID, previous)
		if err != nil {
			return err
		}
		for _, d := range before {
			had[d.InventoryItemID] += d.Quantity
		}
	}
	needed := make(map[uuid.UUID]int, len(deductions))
	order := make([]uuid.UUID, 0, len(deductions))
	for _, d := range deductions {
		if _, seen := needed[d.InventoryItemID]; !seen {
			order = append(order, d.InventoryItemID)
		}
		needed[d.InventoryItemID] += d.Quantity
	}
	var reserved map[uuid.UUID]int
	for _, id := range order {
		if needed[id] <= had[id] {
			continue
		}
		if reserved == nil {
			if reserved, err = s.Reserved(ctx, locationID, orderID); err != nil {
				return err
			}
		}
		item, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		available := item.CurrentStock - reserved[id]
		if available < needed[id] {
			return &apperror.InsufficientStockError{
				Name:      item.Name,
				Unit:      stockUnit(item.Type),
				Needed:    needed[id],
				Available: max(available, 0),
				Reserved:  reserved[id],
			}
		}
	}
	return nil
}

// DeductForOrder deducts stock for an order's items.
// Returns the created stock movements for potential rollback.
func (s *InventoryService) DeductForOrder(ctx context.Context, locationID, orderID uuid.UUID, items []dto.OrderItem, createdByID string) ([]dto.StockMovement, error) {
	deductions, err := s.DeductionsFor(ctx, locationID, items)
	if err != nil {
		return nil, err
	}
	return s.repo.DeductStockForOrder(ctx, orderID, deductions, createdByID)
}

// extractFlavorIDs parses flavor IDs from order item customizations.
func extractFlavorIDs(customizations interface{}) []uuid.UUID {
	if customizations == nil {
		return nil
	}

	var raw map[string]interface{}
	switch v := customizations.(type) {
	case map[string]interface{}:
		raw = v
	case []byte:
		if err := json.Unmarshal(v, &raw); err != nil {
			return nil
		}
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil
		}
	}

	var ids []uuid.UUID

	if flavorIDs, ok := raw["flavor_ids"]; ok {
		switch v := flavorIDs.(type) {
		case []interface{}:
			for _, id := range v {
				if str, ok := id.(string); ok {
					if parsed, err := uuid.Parse(str); err == nil {
						ids = append(ids, parsed)
					}
				}
			}
		case []string:
			for _, str := range v {
				if parsed, err := uuid.Parse(str); err == nil {
					ids = append(ids, parsed)
				}
			}
		}
	}

	if len(ids) == 0 {
		if flavorID, ok := raw["flavor_id"]; ok {
			if v, ok := flavorID.(string); ok {
				if parsed, err := uuid.Parse(v); err == nil {
					ids = append(ids, parsed)
				}
			}
		}
	}

	return ids
}

// RestoreForOrder restores stock for an order's items (used on cancellation).
func (s *InventoryService) RestoreForOrder(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error) {
	return s.repo.RestoreStockForOrder(ctx, orderID, createdByID)
}

// RecordEntry records a stock entry for a flavor (containers received).
// If the flavor does not have an inventory item yet, it is created automatically
// provided the service was configured with a FlavorRepository.
// The entry goes to locationID's stock.
func (s *InventoryService) RecordEntry(ctx context.Context, locationID, flavorID, containerTypeID uuid.UUID, quantityContainers int, ballsPerContainer int, createdByID string) (dto.StockMovement, error) {
	invItem, err := s.repo.FindByFlavorID(ctx, locationID, flavorID)
	if err != nil {
		if !errors.Is(err, apperror.ErrNotFound) {
			return dto.StockMovement{}, err
		}

		// Flavor not tracked yet. Auto-create an inventory item when possible.
		if s.flavorRepo == nil {
			return dto.StockMovement{}, ErrInventoryNotFound
		}

		flavor, flavorErr := s.flavorRepo.FindByID(ctx, flavorID)
		if flavorErr != nil {
			if errors.Is(flavorErr, apperror.ErrNotFound) {
				return dto.StockMovement{}, fmt.Errorf("%w: sabor no encontrado", apperror.ErrValidation)
			}
			return dto.StockMovement{}, flavorErr
		}

		created, createErr := s.repo.Create(ctx, repository.CreateInventoryItemInput{
			LocationID:        locationID,
			Name:              flavor.Name,
			Type:              inventoryTypeBallBased,
			CurrentStock:      0,
			LowStockThreshold: defaultFlavorLowStockThreshold,
			FlavorID:          &flavorID,
		})
		if createErr != nil {
			return dto.StockMovement{}, createErr
		}
		invItem = created
	}

	ct, err := s.containerTypeRepo.GetByID(ctx, containerTypeID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return dto.StockMovement{}, fmt.Errorf("%w: tipo de envase no encontrado", apperror.ErrValidation)
		}
		return dto.StockMovement{}, err
	}

	effectiveBallsPerContainer := ct.BallsPerContainer
	if ct.IsCustom {
		effectiveBallsPerContainer = ballsPerContainer
	}

	if effectiveBallsPerContainer <= 0 {
		return dto.StockMovement{}, fmt.Errorf("%w: las bolas por envase deben ser más de 0", apperror.ErrValidation)
	}

	totalBalls := quantityContainers * effectiveBallsPerContainer

	return s.repo.RecordEntry(ctx, repository.RecordEntryInput{
		InventoryItemID:    invItem.ID,
		ContainerTypeID:    containerTypeID,
		QuantityContainers: quantityContainers,
		TotalBalls:         totalBalls,
		CreatedByID:        createdByID,
	})
}

// RecordUnitEntry records a stock entry for a unit-based product.
func (s *InventoryService) RecordUnitEntry(ctx context.Context, loc scope.Location, inventoryItemID uuid.UUID, quantity int, createdByID string) (dto.StockMovement, error) {
	if quantity <= 0 {
		return dto.StockMovement{}, fmt.Errorf("%w: la cantidad debe ser mayor que 0", apperror.ErrValidation)
	}
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return dto.StockMovement{}, err
	}

	return s.repo.RecordUnitEntry(ctx, repository.RecordUnitEntryInput{
		InventoryItemID: inventoryItemID,
		Quantity:        quantity,
		CreatedByID:     createdByID,
	})
}

// CreateUnitInventoryItem starts tracking a unit-based product at every
// location (zero stock each) and returns the created rows. Locations that
// already track the product are skipped.
func (s *InventoryService) CreateUnitInventoryItem(ctx context.Context, name string, productID uuid.UUID, lowStockThreshold int) ([]dto.InventoryItem, error) {
	if s.locationIDs == nil {
		return nil, fmt.Errorf("inventory service has no location source")
	}
	ids, err := s.locationIDs(ctx)
	if err != nil {
		return nil, err
	}
	created := make([]dto.InventoryItem, 0, len(ids))
	for _, locationID := range ids {
		if _, err := s.repo.FindByProductID(ctx, locationID, productID); err == nil {
			continue
		} else if !errors.Is(err, apperror.ErrNotFound) {
			return nil, err
		}
		item, err := s.repo.Create(ctx, repository.CreateInventoryItemInput{
			LocationID:        locationID,
			Name:              name,
			Type:              "unit_based",
			ProductID:         &productID,
			LowStockThreshold: lowStockThreshold,
			CurrentStock:      0,
		})
		if err != nil {
			return nil, err
		}
		created = append(created, item)
	}
	return created, nil
}

// RecordAdjustment records a manual adjustment for an inventory item.
func (s *InventoryService) RecordAdjustment(ctx context.Context, loc scope.Location, inventoryItemID uuid.UUID, quantity int, reason string, createdByID string) (dto.StockMovement, error) {
	invItem, err := s.itemInScope(ctx, loc, inventoryItemID)
	if err != nil {
		return dto.StockMovement{}, ErrInventoryNotFound
	}

	newStock := invItem.CurrentStock + quantity
	if newStock < 0 {
		return dto.StockMovement{}, ErrInsufficientStock
	}

	return s.repo.RecordAdjustment(ctx, repository.RecordAdjustmentInput{
		InventoryItemID: inventoryItemID,
		Quantity:        quantity,
		Reason:          reason,
		CreatedByID:     createdByID,
	})
}

// GetStockForFlavor returns the stock status for a flavor.
func (s *InventoryService) GetStockForFlavor(ctx context.Context, locationID, flavorID uuid.UUID) (*StockStatus, error) {
	invItem, err := s.repo.FindByFlavorID(ctx, locationID, flavorID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return &StockStatus{Status: "not_tracked"}, nil
		}
		return nil, err
	}

	if invItem.CurrentStock <= 0 {
		return &StockStatus{
			Status:       "out_of_stock",
			CurrentStock: invItem.CurrentStock,
			Threshold:    invItem.LowStockThreshold,
		}, nil
	}
	if StockLevel(invItem.CurrentStock, invItem.LowStockThreshold) == "low" {
		return &StockStatus{
			Status:       "low_stock",
			CurrentStock: invItem.CurrentStock,
			Threshold:    invItem.LowStockThreshold,
		}, nil
	}

	return &StockStatus{
		Status:       "in_stock",
		CurrentStock: invItem.CurrentStock,
		Threshold:    invItem.LowStockThreshold,
	}, nil
}

// StockStatus represents the stock status of a flavor or product.
type StockStatus struct {
	Status       string `json:"status"`
	CurrentStock int    `json:"current_stock,omitempty"`
	Threshold    int    `json:"threshold,omitempty"`
}

// GetMovementHistory returns stock movements for an inventory item.
func (s *InventoryService) GetMovementHistory(ctx context.Context, loc scope.Location, inventoryItemID uuid.UUID, limit, offset int) ([]dto.StockMovement, int, error) {
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return nil, 0, err
	}
	return s.repo.FindStockMovementsByInventoryItem(ctx, inventoryItemID, limit, offset)
}

// GetInventoryItemByProductID returns the inventory item for a unit-based product.
func (s *InventoryService) GetInventoryItemByProductID(ctx context.Context, locationID, productID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.FindByProductID(ctx, locationID, productID)
}

// GetInventoryItemByFlavorID returns the inventory item for a flavor.
func (s *InventoryService) GetInventoryItemByFlavorID(ctx context.Context, locationID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.FindByFlavorID(ctx, locationID, flavorID)
}

// LinkInventoryItemToProduct links an inventory item to a product.
func (s *InventoryService) LinkInventoryItemToProduct(ctx context.Context, loc scope.Location, inventoryItemID, productID uuid.UUID) (dto.InventoryItem, error) {
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return dto.InventoryItem{}, err
	}
	return s.repo.LinkToProduct(ctx, inventoryItemID, productID)
}

// UnlinkInventoryItemFromProduct removes the product link from an inventory item.
func (s *InventoryService) UnlinkInventoryItemFromProduct(ctx context.Context, loc scope.Location, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return dto.InventoryItem{}, err
	}
	return s.repo.UnlinkFromProduct(ctx, inventoryItemID)
}

// LinkInventoryItemToFlavor links an inventory item to a flavor.
func (s *InventoryService) LinkInventoryItemToFlavor(ctx context.Context, loc scope.Location, inventoryItemID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return dto.InventoryItem{}, err
	}
	return s.repo.LinkToFlavor(ctx, inventoryItemID, flavorID)
}

// UnlinkInventoryItemFromFlavor removes the flavor link from an inventory item.
func (s *InventoryService) UnlinkInventoryItemFromFlavor(ctx context.Context, loc scope.Location, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	if _, err := s.itemInScope(ctx, loc, inventoryItemID); err != nil {
		return dto.InventoryItem{}, err
	}
	return s.repo.UnlinkFromFlavor(ctx, inventoryItemID)
}

// GetInventoryStats returns aggregate inventory statistics for linked items only.
func (s *InventoryService) GetInventoryStats(ctx context.Context, loc scope.Location) (*InventoryStats, error) {
	items, err := s.repo.ListLinked(ctx, loc)
	if err != nil {
		return nil, err
	}

	stats := &InventoryStats{
		TotalItems:      len(items),
		OutOfStockItems: 0,
		LowStockItems:   0,
		HealthyItems:    0,
	}

	for _, item := range items {
		switch item.Type {
		case "ball_based":
			stats.BallBasedItems++
		case "unit_based":
			stats.UnitBasedItems++
		}

		switch StockLevel(item.CurrentStock, item.LowStockThreshold) {
		case "out":
			stats.OutOfStockItems++
		case "low":
			stats.LowStockItems++
		default:
			stats.HealthyItems++
		}
	}

	return stats, nil
}

// InventoryStats represents aggregate inventory statistics.
type InventoryStats struct {
	TotalItems      int `json:"total_items"`
	BallBasedItems  int `json:"ball_based_items"`
	UnitBasedItems  int `json:"unit_based_items"`
	OutOfStockItems int `json:"out_of_stock_items"`
	LowStockItems   int `json:"low_stock_items"`
	HealthyItems    int `json:"healthy_items"`
}

// InventoryDashboard bundles the data the admin inventory tab needs on first
// load. Computing it from a single ListLinked query removes two extra Neon
// round trips compared with fetching flavors, units and stats separately.
type InventoryDashboard struct {
	FlavorItems []dto.InventoryItem `json:"flavor_items"`
	UnitItems   []dto.InventoryItem `json:"unit_items"`
	Stats       InventoryStats      `json:"stats"`
}

// GetInventoryDashboard returns flavor inventory, unit inventory and aggregate
// stats in one shot.
func (s *InventoryService) GetInventoryDashboard(ctx context.Context, loc scope.Location) (*InventoryDashboard, error) {
	items, err := s.repo.ListLinked(ctx, loc)
	if err != nil {
		return nil, err
	}

	// Show what accepted orders hold, per location.
	reservedByLocation := map[uuid.UUID]map[uuid.UUID]int{}
	for i := range items {
		lid := items[i].LocationID
		if _, done := reservedByLocation[lid]; !done {
			r, err := s.Reserved(ctx, lid, uuid.Nil)
			if err != nil {
				return nil, err
			}
			reservedByLocation[lid] = r
		}
		items[i].Reserved = reservedByLocation[lid][items[i].ID]
	}

	dashboard := &InventoryDashboard{
		FlavorItems: make([]dto.InventoryItem, 0, len(items)/2),
		UnitItems:   make([]dto.InventoryItem, 0, len(items)/2),
		Stats: InventoryStats{
			TotalItems: len(items),
		},
	}

	for _, item := range items {
		switch item.Type {
		case "ball_based":
			dashboard.Stats.BallBasedItems++
			if item.FlavorID != nil {
				dashboard.FlavorItems = append(dashboard.FlavorItems, item)
			}
		case "unit_based":
			dashboard.Stats.UnitBasedItems++
			if item.ProductID != nil {
				dashboard.UnitItems = append(dashboard.UnitItems, item)
			}
		}

		switch StockLevel(item.CurrentStock, item.LowStockThreshold) {
		case "out":
			dashboard.Stats.OutOfStockItems++
		case "low":
			dashboard.Stats.LowStockItems++
		default:
			dashboard.Stats.HealthyItems++
		}
	}

	return dashboard, nil
}
