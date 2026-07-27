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
)

var (
	ErrInsufficientStock = fmt.Errorf("%w: insufficient stock", apperror.ErrConflict)
	ErrInventoryNotFound = fmt.Errorf("%w: inventory item not found", apperror.ErrNotFound)
)

// defaultFlavorLowStockThreshold is used when a flavor inventory item is
// auto-created on the first stock entry.
const defaultFlavorLowStockThreshold = 100

// inventoryTypeBallBased matches the database value for ball-based inventory.
const inventoryTypeBallBased = "ball_based"

// InventoryService handles all inventory-related business logic.
type InventoryService struct {
	repo              InventoryRepository
	containerTypeRepo ContainerTypeRepository
	flavorRepo        FlavorRepository
}

// InventoryServiceOption configures an InventoryService.
type InventoryServiceOption func(*InventoryService)

// WithFlavorRepo enables the inventory service to look up flavor details and
// auto-create inventory items for flavors that are not yet tracked.
func WithFlavorRepo(r FlavorRepository) InventoryServiceOption {
	return func(s *InventoryService) { s.flavorRepo = r }
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
func (s *InventoryService) List(ctx context.Context) ([]dto.InventoryItem, error) {
	return s.repo.List(ctx)
}

// GetFlavorInventory returns only ball-based items linked to a flavor.
func (s *InventoryService) GetFlavorInventory(ctx context.Context) ([]dto.InventoryItem, error) {
	items, err := s.repo.ListByType(ctx, "ball_based")
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
func (s *InventoryService) GetUnitInventory(ctx context.Context) ([]dto.InventoryItem, error) {
	items, err := s.repo.ListByType(ctx, "unit_based")
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
func (s *InventoryService) GetByID(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.FindByID(ctx, id)
}

// GetLowStockItems returns items below their threshold, excluding orphaned items.
func (s *InventoryService) GetLowStockItems(ctx context.Context) ([]dto.InventoryItem, error) {
	items, err := s.repo.ListLinked(ctx)
	if err != nil {
		return nil, err
	}

	lowStock := make([]dto.InventoryItem, 0)
	for _, item := range items {
		if item.CurrentStock <= item.LowStockThreshold {
			lowStock = append(lowStock, item)
		}
	}
	return lowStock, nil
}

// CheckStock verifies if there is enough stock for a specific flavor at a given quantity.
func (s *InventoryService) CheckStock(ctx context.Context, flavorID uuid.UUID, quantity int) (bool, error) {
	invItem, err := s.repo.FindByFlavorID(ctx, flavorID)
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
func (s *InventoryService) DeductionsFor(ctx context.Context, items []dto.OrderItem) ([]repository.StockDeductionInput, error) {
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
				invItem, err := s.repo.FindByFlavorID(ctx, flavorID)
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
			invItem, err := s.repo.FindByProductID(ctx, item.ProductID)
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

// DeductForOrder deducts stock for an order's items.
// Returns the created stock movements for potential rollback.
func (s *InventoryService) DeductForOrder(ctx context.Context, orderID uuid.UUID, items []dto.OrderItem, createdByID string) ([]dto.StockMovement, error) {
	deductions, err := s.DeductionsFor(ctx, items)
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
func (s *InventoryService) RecordEntry(ctx context.Context, flavorID, containerTypeID uuid.UUID, quantityContainers int, ballsPerContainer int, createdByID string) (dto.StockMovement, error) {
	invItem, err := s.repo.FindByFlavorID(ctx, flavorID)
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
				return dto.StockMovement{}, fmt.Errorf("%w: flavor not found", apperror.ErrValidation)
			}
			return dto.StockMovement{}, flavorErr
		}

		created, createErr := s.repo.Create(ctx, repository.CreateInventoryItemInput{
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
			return dto.StockMovement{}, fmt.Errorf("%w: container type not found", apperror.ErrValidation)
		}
		return dto.StockMovement{}, err
	}

	effectiveBallsPerContainer := ct.BallsPerContainer
	if ct.IsCustom {
		effectiveBallsPerContainer = ballsPerContainer
	}

	if effectiveBallsPerContainer <= 0 {
		return dto.StockMovement{}, fmt.Errorf("%w: balls per container must be positive", apperror.ErrValidation)
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
func (s *InventoryService) RecordUnitEntry(ctx context.Context, inventoryItemID uuid.UUID, quantity int, createdByID string) (dto.StockMovement, error) {
	if quantity <= 0 {
		return dto.StockMovement{}, fmt.Errorf("%w: quantity must be positive", apperror.ErrValidation)
	}

	return s.repo.RecordUnitEntry(ctx, repository.RecordUnitEntryInput{
		InventoryItemID: inventoryItemID,
		Quantity:        quantity,
		CreatedByID:     createdByID,
	})
}

// CreateUnitInventoryItem creates a new unit-based inventory item linked to a product.
func (s *InventoryService) CreateUnitInventoryItem(ctx context.Context, name string, productID uuid.UUID, lowStockThreshold int) (dto.InventoryItem, error) {
	return s.repo.Create(ctx, repository.CreateInventoryItemInput{
		Name:              name,
		Type:              "unit_based",
		ProductID:         &productID,
		LowStockThreshold: lowStockThreshold,
		CurrentStock:      0,
	})
}

// RecordAdjustment records a manual adjustment for an inventory item.
func (s *InventoryService) RecordAdjustment(ctx context.Context, inventoryItemID uuid.UUID, quantity int, reason string, createdByID string) (dto.StockMovement, error) {
	invItem, err := s.repo.FindByID(ctx, inventoryItemID)
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
func (s *InventoryService) GetStockForFlavor(ctx context.Context, flavorID uuid.UUID) (*StockStatus, error) {
	invItem, err := s.repo.FindByFlavorID(ctx, flavorID)
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
	if invItem.CurrentStock <= invItem.LowStockThreshold {
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
func (s *InventoryService) GetMovementHistory(ctx context.Context, inventoryItemID uuid.UUID, limit, offset int) ([]dto.StockMovement, int, error) {
	return s.repo.FindStockMovementsByInventoryItem(ctx, inventoryItemID, limit, offset)
}

// GetInventoryItemByProductID returns the inventory item for a unit-based product.
func (s *InventoryService) GetInventoryItemByProductID(ctx context.Context, productID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.FindByProductID(ctx, productID)
}

// GetInventoryItemByFlavorID returns the inventory item for a flavor.
func (s *InventoryService) GetInventoryItemByFlavorID(ctx context.Context, flavorID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.FindByFlavorID(ctx, flavorID)
}

// LinkInventoryItemToProduct links an inventory item to a product.
func (s *InventoryService) LinkInventoryItemToProduct(ctx context.Context, inventoryItemID, productID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.LinkToProduct(ctx, inventoryItemID, productID)
}

// UnlinkInventoryItemFromProduct removes the product link from an inventory item.
func (s *InventoryService) UnlinkInventoryItemFromProduct(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.UnlinkFromProduct(ctx, inventoryItemID)
}

// LinkInventoryItemToFlavor links an inventory item to a flavor.
func (s *InventoryService) LinkInventoryItemToFlavor(ctx context.Context, inventoryItemID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.LinkToFlavor(ctx, inventoryItemID, flavorID)
}

// UnlinkInventoryItemFromFlavor removes the flavor link from an inventory item.
func (s *InventoryService) UnlinkInventoryItemFromFlavor(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	return s.repo.UnlinkFromFlavor(ctx, inventoryItemID)
}

// GetInventoryStats returns aggregate inventory statistics for linked items only.
func (s *InventoryService) GetInventoryStats(ctx context.Context) (*InventoryStats, error) {
	items, err := s.repo.ListLinked(ctx)
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

		switch {
		case item.CurrentStock <= 0:
			stats.OutOfStockItems++
		case item.CurrentStock <= item.LowStockThreshold:
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
func (s *InventoryService) GetInventoryDashboard(ctx context.Context) (*InventoryDashboard, error) {
	items, err := s.repo.ListLinked(ctx)
	if err != nil {
		return nil, err
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

		switch {
		case item.CurrentStock <= 0:
			dashboard.Stats.OutOfStockItems++
		case item.CurrentStock <= item.LowStockThreshold:
			dashboard.Stats.LowStockItems++
		default:
			dashboard.Stats.HealthyItems++
		}
	}

	return dashboard, nil
}
