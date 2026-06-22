// inventory_repo.go — Ent-backed repository for inventory items and stock movements.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/inventoryitem"
	"github.com/cjrdz/ishosBackEnd/ent/stockmovement"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// CreateInventoryItemInput is the data required to create an inventory item.
type CreateInventoryItemInput struct {
	Name              string
	Type              string
	ProductID         *uuid.UUID
	FlavorID          *uuid.UUID
	LowStockThreshold int
	CurrentStock      int
}

// UpdateInventoryItemInput is the data required to update an inventory item.
// A nil ProductID/FlavorID means "do not change" unless the corresponding Clear flag is set.
type UpdateInventoryItemInput struct {
	Name              *string
	Type              *string
	ProductID         *uuid.UUID
	FlavorID          *uuid.UUID
	ClearProductID    bool
	ClearFlavorID     bool
	LowStockThreshold *int
}

// StockDeductionInput is a single stock deduction inside an order.
type StockDeductionInput struct {
	InventoryItemID uuid.UUID
	FlavorID        *uuid.UUID
	Quantity        int
}

// RecordEntryInput is the data required to record a stock entry.
type RecordEntryInput struct {
	InventoryItemID    uuid.UUID
	ContainerTypeID    uuid.UUID
	QuantityContainers int
	TotalBalls         int
	CreatedByID        string
}

// RecordAdjustmentInput is the data required to record a stock adjustment.
type RecordAdjustmentInput struct {
	InventoryItemID uuid.UUID
	Quantity        int
	Reason          string
	CreatedByID     string
}

// RecordUnitEntryInput is the data required to record a unit-based stock entry.
type RecordUnitEntryInput struct {
	InventoryItemID uuid.UUID
	Quantity        int
	CreatedByID     string
}

// InventoryRepository implements the inventory data access contract.
type InventoryRepository struct {
	db *ent.Client
}

// NewInventoryRepository creates a new inventory repository.
func NewInventoryRepository(db *ent.Client) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// List returns all inventory items ordered by name.
func (r *InventoryRepository) List(ctx context.Context) ([]dto.InventoryItem, error) {
	items, err := r.db.InventoryItem.Query().
		Order(ent.Asc(inventoryitem.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOInventoryItems(items), nil
}

// ListByType returns inventory items of a specific type ordered by name.
func (r *InventoryRepository) ListByType(ctx context.Context, itemType string) ([]dto.InventoryItem, error) {
	items, err := r.db.InventoryItem.Query().
		Where(inventoryitem.TypeEQ(inventoryitem.Type(itemType))).
		Order(ent.Asc(inventoryitem.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOInventoryItems(items), nil
}

// ListLinked returns inventory items linked to a flavor or product.
func (r *InventoryRepository) ListLinked(ctx context.Context) ([]dto.InventoryItem, error) {
	items, err := r.db.InventoryItem.Query().
		Where(
			inventoryitem.Or(
				inventoryitem.FlavorIDNotNil(),
				inventoryitem.ProductIDNotNil(),
			),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOInventoryItems(items), nil
}

// FindByID returns a single inventory item by ID.
func (r *InventoryRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.Get(ctx, id)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// FindByFlavorID returns the inventory item linked to a flavor.
func (r *InventoryRepository) FindByFlavorID(ctx context.Context, flavorID uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.Query().
		Where(inventoryitem.FlavorID(flavorID)).
		Only(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// FindByProductID returns the inventory item linked to a product.
func (r *InventoryRepository) FindByProductID(ctx context.Context, productID uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.Query().
		Where(inventoryitem.ProductID(productID)).
		Only(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// Create creates a new inventory item.
func (r *InventoryRepository) Create(ctx context.Context, input CreateInventoryItemInput) (dto.InventoryItem, error) {
	now := time.Now()
	create := r.db.InventoryItem.Create().
		SetName(input.Name).
		SetType(inventoryitem.Type(input.Type)).
		SetCurrentStock(input.CurrentStock).
		SetLowStockThreshold(input.LowStockThreshold).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	if input.ProductID != nil {
		create = create.SetProductID(*input.ProductID)
	}
	if input.FlavorID != nil {
		create = create.SetFlavorID(*input.FlavorID)
	}

	item, err := create.Save(ctx)
	if err != nil {
		return dto.InventoryItem{}, err
	}

	return toDTOInventoryItem(item), nil
}

// Update updates an inventory item.
func (r *InventoryRepository) Update(ctx context.Context, id uuid.UUID, input UpdateInventoryItemInput) (dto.InventoryItem, error) {
	update := r.db.InventoryItem.UpdateOneID(id)

	if input.Name != nil {
		update = update.SetName(*input.Name)
	}
	if input.Type != nil {
		update = update.SetType(inventoryitem.Type(*input.Type))
	}
	if input.LowStockThreshold != nil {
		update = update.SetLowStockThreshold(*input.LowStockThreshold)
	}
	if input.ProductID != nil {
		update = update.SetProductID(*input.ProductID)
	} else if input.ClearProductID {
		update = update.ClearProductID()
	}
	if input.FlavorID != nil {
		update = update.SetFlavorID(*input.FlavorID)
	} else if input.ClearFlavorID {
		update = update.ClearFlavorID()
	}

	item, err := update.SetUpdatedAt(time.Now()).Save(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// Delete removes an inventory item.
func (r *InventoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.InventoryItem.DeleteOneID(id).Exec(ctx)
}

// LinkToProduct links an inventory item to a product.
func (r *InventoryRepository) LinkToProduct(ctx context.Context, inventoryItemID, productID uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.UpdateOneID(inventoryItemID).
		SetProductID(productID).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// UnlinkFromProduct removes the product link from an inventory item.
func (r *InventoryRepository) UnlinkFromProduct(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.UpdateOneID(inventoryItemID).
		ClearProductID().
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// LinkToFlavor links an inventory item to a flavor.
func (r *InventoryRepository) LinkToFlavor(ctx context.Context, inventoryItemID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.UpdateOneID(inventoryItemID).
		SetFlavorID(flavorID).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// UnlinkFromFlavor removes the flavor link from an inventory item.
func (r *InventoryRepository) UnlinkFromFlavor(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	item, err := r.db.InventoryItem.UpdateOneID(inventoryItemID).
		ClearFlavorID().
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}

	return toDTOInventoryItem(item), nil
}

// FindStockMovementsByOrder returns all stock movements for an order.
func (r *InventoryRepository) FindStockMovementsByOrder(ctx context.Context, orderID uuid.UUID) ([]dto.StockMovement, error) {
	movements, err := r.db.StockMovement.Query().
		Where(stockmovement.OrderIDEQ(orderID)).
		Order(ent.Desc(stockmovement.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOStockMovements(movements), nil
}

// FindStockMovementsByInventoryItem returns paginated stock movements for an inventory item.
func (r *InventoryRepository) FindStockMovementsByInventoryItem(ctx context.Context, inventoryItemID uuid.UUID, limit, offset int) ([]dto.StockMovement, int, error) {
	query := r.db.StockMovement.Query().
		Where(stockmovement.InventoryItemID(inventoryItemID)).
		Order(ent.Desc(stockmovement.FieldCreatedAt))

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	movements, err := query.
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return toDTOStockMovements(movements), total, nil
}

// DeductStockForOrder applies a list of stock deductions and creates sale movements atomically.
func (r *InventoryRepository) DeductStockForOrder(ctx context.Context, orderID uuid.UUID, deductions []StockDeductionInput, createdByID string) ([]dto.StockMovement, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	movements := make([]dto.StockMovement, 0, len(deductions))

	for _, d := range deductions {
		invItem, err := tx.InventoryItem.Get(ctx, d.InventoryItemID)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		if invItem.CurrentStock < d.Quantity {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s needs %d but only %d available",
				apperror.ErrConflict, invItem.Name, d.Quantity, invItem.CurrentStock)
		}

		updated, err := tx.InventoryItem.UpdateOneID(invItem.ID).
			SetCurrentStock(invItem.CurrentStock - d.Quantity).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		create := tx.StockMovement.Create().
			SetType("sale").
			SetQuantity(-d.Quantity).
			SetInventoryItemID(invItem.ID).
			SetNillableOrderID(&orderID)
		if d.FlavorID != nil {
			create = create.SetNillableFlavorID(d.FlavorID)
		}
		if createdByID != "" {
			create = create.SetNillableCreatedByUserID(&createdByID)
		}

		movement, err := create.Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		movements = append(movements, toDTOStockMovement(movement))
		_ = updated
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return movements, nil
}

// RestoreStockForOrder reverses all sale movements for an order atomically.
func (r *InventoryRepository) RestoreStockForOrder(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	saleMovements, err := tx.StockMovement.Query().
		Where(
			stockmovement.And(
				stockmovement.OrderIDEQ(orderID),
				stockmovement.TypeEQ("sale"),
			),
		).
		All(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	restored := make([]dto.StockMovement, 0, len(saleMovements))

	for _, m := range saleMovements {
		invItem, err := tx.InventoryItem.Get(ctx, m.InventoryItemID)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		_, err = tx.InventoryItem.UpdateOneID(invItem.ID).
			SetCurrentStock(invItem.CurrentStock + (-m.Quantity)).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		restMovement, err := tx.StockMovement.Create().
			SetType("adjustment").
			SetQuantity(-m.Quantity).
			SetInventoryItemID(invItem.ID).
			SetNillableOrderID(&orderID).
			SetReason("Order cancelled").
			SetNillableCreatedByUserID(&createdByID).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		restored = append(restored, toDTOStockMovement(restMovement))
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return restored, nil
}

// RecordEntry records a stock entry for an inventory item atomically.
func (r *InventoryRepository) RecordEntry(ctx context.Context, input RecordEntryInput) (dto.StockMovement, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return dto.StockMovement{}, err
	}

	invItem, err := tx.InventoryItem.Get(ctx, input.InventoryItemID)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, translateInventoryError(err)
	}

	_, err = tx.InventoryItem.UpdateOneID(invItem.ID).
		SetCurrentStock(invItem.CurrentStock + input.TotalBalls).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, err
	}

	quantityContainers := input.QuantityContainers
	movement, err := tx.StockMovement.Create().
		SetType("entry").
		SetQuantity(input.TotalBalls).
		SetNillableQuantityContainers(&quantityContainers).
		SetInventoryItemID(invItem.ID).
		SetNillableContainerTypeID(&input.ContainerTypeID).
		SetNillableCreatedByUserID(&input.CreatedByID).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, err
	}

	if err := tx.Commit(); err != nil {
		return dto.StockMovement{}, err
	}

	return toDTOStockMovement(movement), nil
}

// RecordAdjustment records a manual stock adjustment atomically.
func (r *InventoryRepository) RecordAdjustment(ctx context.Context, input RecordAdjustmentInput) (dto.StockMovement, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return dto.StockMovement{}, err
	}

	invItem, err := tx.InventoryItem.Get(ctx, input.InventoryItemID)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, translateInventoryError(err)
	}

	newStock := invItem.CurrentStock + input.Quantity
	if newStock < 0 {
		_ = tx.Rollback()
		return dto.StockMovement{}, apperror.ErrConflict
	}

	_, err = tx.InventoryItem.UpdateOneID(invItem.ID).
		SetCurrentStock(newStock).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, err
	}

	movement, err := tx.StockMovement.Create().
		SetType("adjustment").
		SetQuantity(input.Quantity).
		SetInventoryItemID(invItem.ID).
		SetReason(input.Reason).
		SetNillableCreatedByUserID(&input.CreatedByID).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, err
	}

	if err := tx.Commit(); err != nil {
		return dto.StockMovement{}, err
	}

	return toDTOStockMovement(movement), nil
}

// RecordUnitEntry records a stock entry for a unit-based inventory item atomically.
func (r *InventoryRepository) RecordUnitEntry(ctx context.Context, input RecordUnitEntryInput) (dto.StockMovement, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return dto.StockMovement{}, err
	}

	invItem, err := tx.InventoryItem.Get(ctx, input.InventoryItemID)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, translateInventoryError(err)
	}

	_, err = tx.InventoryItem.UpdateOneID(invItem.ID).
		SetCurrentStock(invItem.CurrentStock + input.Quantity).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, err
	}

	movement, err := tx.StockMovement.Create().
		SetType("entry").
		SetQuantity(input.Quantity).
		SetInventoryItemID(invItem.ID).
		SetNillableCreatedByUserID(&input.CreatedByID).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.StockMovement{}, err
	}

	if err := tx.Commit(); err != nil {
		return dto.StockMovement{}, err
	}

	return toDTOStockMovement(movement), nil
}

// translateInventoryError converts Ent not-found errors into apperror.NotFoundError.
func translateInventoryError(err error) error {
	if ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "inventory_item", Cause: err}
	}
	return err
}

// toDTOInventoryItem maps an ent.InventoryItem to a dto.InventoryItem.
func toDTOInventoryItem(item *ent.InventoryItem) dto.InventoryItem {
	if item == nil {
		return dto.InventoryItem{}
	}

	return dto.InventoryItem{
		ID:                item.ID,
		Name:              item.Name,
		Type:              string(item.Type),
		CurrentStock:      item.CurrentStock,
		LowStockThreshold: item.LowStockThreshold,
		ProductID:         item.ProductID,
		FlavorID:          item.FlavorID,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

// toDTOInventoryItems maps a slice of ent.InventoryItem values to DTOs.
func toDTOInventoryItems(items []*ent.InventoryItem) []dto.InventoryItem {
	result := make([]dto.InventoryItem, 0, len(items))
	for _, item := range items {
		result = append(result, toDTOInventoryItem(item))
	}
	return result
}

// toDTOStockMovement maps an ent.StockMovement to a dto.StockMovement.
func toDTOStockMovement(movement *ent.StockMovement) dto.StockMovement {
	if movement == nil {
		return dto.StockMovement{}
	}

	return dto.StockMovement{
		ID:                 movement.ID,
		Type:               string(movement.Type),
		Quantity:           movement.Quantity,
		QuantityContainers: movement.QuantityContainers,
		InventoryItemID:    movement.InventoryItemID,
		ContainerTypeID:    movement.ContainerTypeID,
		FlavorID:           movement.FlavorID,
		OrderID:            movement.OrderID,
		Reason:             movement.Reason,
		CreatedByUserID:    movement.CreatedByUserID,
		CreatedAt:          movement.CreatedAt,
	}
}

// toDTOStockMovements maps a slice of ent.StockMovement values to DTOs.
func toDTOStockMovements(movements []*ent.StockMovement) []dto.StockMovement {
	result := make([]dto.StockMovement, 0, len(movements))
	for _, movement := range movements {
		result = append(result, toDTOStockMovement(movement))
	}
	return result
}
