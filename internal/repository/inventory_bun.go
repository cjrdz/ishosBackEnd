// inventory_bun.go — Bun-backed repository for inventory items and stock movements.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

// InventoryBunRepository implements services.InventoryRepository using Bun.
type InventoryBunRepository struct {
	db bun.IDB
}

// NewInventoryBunRepository creates a new Bun-backed inventory repository.
// Accepts either a *bun.DB or a bun.Tx so the repository can participate in an
// existing transaction.
func NewInventoryBunRepository(db bun.IDB) *InventoryBunRepository {
	return &InventoryBunRepository{db: db}
}

// withTx runs fn against a transaction. If the repository is backed by *bun.DB,
// it starts and commits a new transaction. If it is already backed by bun.Tx, it
// runs fn directly on that transaction without starting a new one.
func (r *InventoryBunRepository) withTx(ctx context.Context, fn func(ctx context.Context, tx bun.Tx) error) error {
	if db, ok := r.db.(*bun.DB); ok {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			return fn(ctx, tx)
		})
	}
	tx, ok := r.db.(bun.Tx)
	if !ok {
		return fmt.Errorf("unsupported db type: %T", r.db)
	}
	return fn(ctx, tx)
}

// List returns the inventory items in scope ordered by name.
func (r *InventoryBunRepository) List(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error) {
	var items []models.InventoryItem
	if err := whereLocation(r.db.NewSelect().Model(&items), "location_id", loc).Order("name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return toDTOInventoryItemsFromModels(items), nil
}

// ListByType returns inventory items of a specific type ordered by name.
func (r *InventoryBunRepository) ListByType(ctx context.Context, loc scope.Location, itemType string) ([]dto.InventoryItem, error) {
	var items []models.InventoryItem
	if err := whereLocation(r.db.NewSelect().Model(&items), "location_id", loc).Where("type = ?", itemType).Order("name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return toDTOInventoryItemsFromModels(items), nil
}

// ListLinked returns inventory items linked to a flavor or product.
func (r *InventoryBunRepository) ListLinked(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error) {
	var items []models.InventoryItem
	if err := whereLocation(r.db.NewSelect().Model(&items), "location_id", loc).Where("flavor_id IS NOT NULL OR product_id IS NOT NULL").Order("name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return toDTOInventoryItemsFromModels(items), nil
}

// FindByID returns a single inventory item by ID.
func (r *InventoryBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error) {
	var item models.InventoryItem
	if err := r.db.NewSelect().Model(&item).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	return toDTOInventoryItemFromModel(item), nil
}

// FindByFlavorID returns a location's inventory item for a flavor.
func (r *InventoryBunRepository) FindByFlavorID(ctx context.Context, locationID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	var item models.InventoryItem
	if err := r.db.NewSelect().Model(&item).Where("location_id = ? AND flavor_id = ?", locationID, flavorID).Scan(ctx); err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	return toDTOInventoryItemFromModel(item), nil
}

// FindByProductID returns a location's inventory item for a product.
func (r *InventoryBunRepository) FindByProductID(ctx context.Context, locationID, productID uuid.UUID) (dto.InventoryItem, error) {
	var item models.InventoryItem
	if err := r.db.NewSelect().Model(&item).Where("location_id = ? AND product_id = ?", locationID, productID).Scan(ctx); err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	return toDTOInventoryItemFromModel(item), nil
}

// Create creates a new inventory item.
func (r *InventoryBunRepository) Create(ctx context.Context, input CreateInventoryItemInput) (dto.InventoryItem, error) {
	now := time.Now()
	item := models.InventoryItem{
		ID:                uuid.New(),
		LocationID:        input.LocationID,
		Name:              input.Name,
		Type:              models.InventoryItemType(input.Type),
		CurrentStock:      input.CurrentStock,
		LowStockThreshold: input.LowStockThreshold,
		ProductID:         input.ProductID,
		FlavorID:          input.FlavorID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if _, err := r.db.NewInsert().Model(&item).Exec(ctx); err != nil {
		return dto.InventoryItem{}, err
	}
	return toDTOInventoryItemFromModel(item), nil
}

// Update updates an inventory item.
func (r *InventoryBunRepository) Update(ctx context.Context, id uuid.UUID, input UpdateInventoryItemInput) (dto.InventoryItem, error) {
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.InventoryItem{}, err
	}

	item := models.InventoryItem{
		ID:                existing.ID,
		LocationID:        existing.LocationID,
		Name:              existing.Name,
		Type:              models.InventoryItemType(existing.Type),
		CurrentStock:      existing.CurrentStock,
		LowStockThreshold: existing.LowStockThreshold,
		ProductID:         existing.ProductID,
		FlavorID:          existing.FlavorID,
		CreatedAt:         existing.CreatedAt,
		UpdatedAt:         time.Now(),
	}

	if input.Name != nil {
		item.Name = *input.Name
	}
	if input.Type != nil {
		item.Type = models.InventoryItemType(*input.Type)
	}
	if input.LowStockThreshold != nil {
		item.LowStockThreshold = *input.LowStockThreshold
	}
	if input.ProductID != nil {
		item.ProductID = input.ProductID
	} else if input.ClearProductID {
		item.ProductID = nil
	}
	if input.FlavorID != nil {
		item.FlavorID = input.FlavorID
	} else if input.ClearFlavorID {
		item.FlavorID = nil
	}

	res, err := r.db.NewUpdate().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}

	return toDTOInventoryItemFromModel(item), nil
}

// Delete removes an inventory item.
func (r *InventoryBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	item := models.InventoryItem{ID: id}
	_, err := r.db.NewDelete().Model(&item).WherePK().Exec(ctx)
	return err
}

// LinkToProduct links an inventory item to a product.
func (r *InventoryBunRepository) LinkToProduct(ctx context.Context, inventoryItemID, productID uuid.UUID) (dto.InventoryItem, error) {
	item := models.InventoryItem{ID: inventoryItemID, ProductID: &productID, UpdatedAt: time.Now()}
	res, err := r.db.NewUpdate().Model(&item).Set("product_id = ?, updated_at = ?", productID, item.UpdatedAt).Where("id = ?", inventoryItemID).Exec(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}
	return r.FindByID(ctx, inventoryItemID)
}

// UnlinkFromProduct removes the product link from an inventory item.
func (r *InventoryBunRepository) UnlinkFromProduct(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	item := models.InventoryItem{ID: inventoryItemID, UpdatedAt: time.Now()}
	res, err := r.db.NewUpdate().Model(&item).Set("product_id = NULL, updated_at = ?", item.UpdatedAt).Where("id = ?", inventoryItemID).Exec(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}
	return r.FindByID(ctx, inventoryItemID)
}

// UpdateLowStockThreshold sets only low_stock_threshold. A full-row update
// would write back a stale current_stock read before a concurrent sale or
// entry.
func (r *InventoryBunRepository) UpdateLowStockThreshold(ctx context.Context, id uuid.UUID, threshold int) (dto.InventoryItem, error) {
	res, err := r.db.NewUpdate().Model((*models.InventoryItem)(nil)).
		Set("low_stock_threshold = ?", threshold).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}
	return r.FindByID(ctx, id)
}

// LinkToFlavor links an inventory item to a flavor.
func (r *InventoryBunRepository) LinkToFlavor(ctx context.Context, inventoryItemID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	item := models.InventoryItem{ID: inventoryItemID, FlavorID: &flavorID, UpdatedAt: time.Now()}
	res, err := r.db.NewUpdate().Model(&item).Set("flavor_id = ?, updated_at = ?", flavorID, item.UpdatedAt).Where("id = ?", inventoryItemID).Exec(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}
	return r.FindByID(ctx, inventoryItemID)
}

// UnlinkFromFlavor removes the flavor link from an inventory item.
func (r *InventoryBunRepository) UnlinkFromFlavor(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	item := models.InventoryItem{ID: inventoryItemID, UpdatedAt: time.Now()}
	res, err := r.db.NewUpdate().Model(&item).Set("flavor_id = NULL, updated_at = ?", item.UpdatedAt).Where("id = ?", inventoryItemID).Exec(ctx)
	if err != nil {
		return dto.InventoryItem{}, translateInventoryError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}
	return r.FindByID(ctx, inventoryItemID)
}

// FindStockMovementsByOrder returns all stock movements for an order.
func (r *InventoryBunRepository) FindStockMovementsByOrder(ctx context.Context, orderID uuid.UUID) ([]dto.StockMovement, error) {
	var movements []models.StockMovement
	if err := r.db.NewSelect().Model(&movements).Where("order_id = ?", orderID).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	return toDTOStockMovementsFromModels(movements), nil
}

// FindStockMovementsByInventoryItem returns paginated stock movements for an inventory item.
func (r *InventoryBunRepository) FindStockMovementsByInventoryItem(ctx context.Context, inventoryItemID uuid.UUID, limit, offset int) ([]dto.StockMovement, int, error) {
	q := r.db.NewSelect().Model((*models.StockMovement)(nil)).Where("inventory_item_id = ?", inventoryItemID)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var movements []models.StockMovement
	if err := q.Model(&movements).Order("created_at DESC").Limit(limit).Offset(offset).Scan(ctx); err != nil {
		return nil, 0, err
	}
	return toDTOStockMovementsFromModels(movements), total, nil
}

// DeductStockForOrder applies a list of stock deductions and creates sale movements atomically.
func (r *InventoryBunRepository) DeductStockForOrder(ctx context.Context, orderID uuid.UUID, deductions []StockDeductionInput, createdByID string) ([]dto.StockMovement, error) {
	var movements []dto.StockMovement
	err := r.withTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		var innerErr error
		movements, innerErr = r.deductStock(ctx, tx, orderID, deductions, createdByID)
		return innerErr
	})
	return movements, err
}

func (r *InventoryBunRepository) deductStock(ctx context.Context, tx bun.Tx, orderID uuid.UUID, deductions []StockDeductionInput, createdByID string) ([]dto.StockMovement, error) {
	movements := make([]dto.StockMovement, 0, len(deductions))
	if len(deductions) == 0 {
		return movements, nil
	}

	var orderLocation uuid.UUID
	if err := tx.NewSelect().Model((*models.Order)(nil)).Column("location_id").Where("id = ?", orderID).Scan(ctx, &orderLocation); err != nil {
		return nil, translateOrderError(err)
	}

	for _, d := range deductions {
		var invItem models.InventoryItem
		if err := tx.NewSelect().Model(&invItem).Where("id = ?", d.InventoryItemID).For("UPDATE").Scan(ctx); err != nil {
			return nil, translateInventoryError(err)
		}
		// Defence in depth: an order only ever consumes its own location's stock.
		if invItem.LocationID != orderLocation {
			return nil, fmt.Errorf("%w: el artículo de inventario %s es de otra sucursal", apperror.ErrConflict, invItem.ID)
		}

		if invItem.CurrentStock < d.Quantity {
			unit := "unidad"
			if invItem.Type == "ball_based" {
				unit = "bola"
			}
			return nil, &apperror.InsufficientStockError{
				Name: invItem.Name, Unit: unit, Needed: d.Quantity, Available: max(invItem.CurrentStock, 0),
			}
		}

		invItem.CurrentStock -= d.Quantity
		invItem.UpdatedAt = time.Now()
		if _, err := tx.NewUpdate().Model(&invItem).WherePK().Exec(ctx); err != nil {
			return nil, err
		}

		movement := models.StockMovement{
			ID:              uuid.New(),
			Type:            models.StockMovementTypeSale,
			Quantity:        -d.Quantity,
			InventoryItemID: invItem.ID,
			LocationID:      invItem.LocationID,
			OrderID:         &orderID,
			FlavorID:        d.FlavorID,
			CreatedAt:       time.Now(),
		}
		if createdByID != "" {
			movement.CreatedByUserID = &createdByID
		}
		if _, err := tx.NewInsert().Model(&movement).Exec(ctx); err != nil {
			return nil, err
		}

		movements = append(movements, toDTOStockMovementFromModel(movement))
	}

	return movements, nil
}

// RestoreStockForOrder reverses all sale movements for an order atomically.
func (r *InventoryBunRepository) RestoreStockForOrder(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error) {
	var restored []dto.StockMovement
	err := r.withTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		var innerErr error
		restored, innerErr = r.restoreStock(ctx, tx, orderID, createdByID, nil, "")
		return innerErr
	})
	return restored, err
}

// OrderStockLine is what an order still holds of one inventory item.
type OrderStockLine struct {
	InventoryItemID uuid.UUID `bun:"inventory_item_id" json:"inventory_item_id"`
	Name            string    `bun:"name" json:"name"`
	Type            string    `bun:"type" json:"type"`
	Quantity        int       `bun:"quantity" json:"quantity"`
}

// orderOutstandingQuery sums an order's net deduction per inventory item:
// sales are negative, restores positive and waste settles what was lost.
func orderOutstandingQuery(db bun.IDB, orderID uuid.UUID) *bun.SelectQuery {
	return db.NewSelect().
		TableExpr("stock_movements").
		ColumnExpr("inventory_item_id").
		ColumnExpr("SUM(quantity) AS net").
		Where("order_id = ?", orderID).
		Where("type IN (?)", bun.List([]models.StockMovementType{
			models.StockMovementTypeSale, models.StockMovementTypeAdjustment, models.StockMovementTypeWaste,
		})).
		Group("inventory_item_id").
		Having("SUM(quantity) < 0")
}

// OrderStock lists what an order took from inventory and has not given back.
func (r *InventoryBunRepository) OrderStock(ctx context.Context, orderID uuid.UUID) ([]OrderStockLine, error) {
	lines := []OrderStockLine{}
	err := r.db.NewSelect().
		TableExpr("(?) AS o", orderOutstandingQuery(r.db, orderID)).
		Join("JOIN inventory_items AS i ON i.id = o.inventory_item_id").
		ColumnExpr("o.inventory_item_id, i.name, i.type, -o.net AS quantity").
		OrderExpr("i.name").
		Scan(ctx, &lines)
	return lines, err
}

// restoreStock gives an order's outstanding stock back. restock nil returns
// everything; otherwise only restock[item] units of each item go back
// (clamped to what the order holds) and the rest is recorded as waste with
// wasteReason, so the order is fully settled either way.
func (r *InventoryBunRepository) restoreStock(ctx context.Context, tx bun.Tx, orderID uuid.UUID, createdByID string, restock map[uuid.UUID]int, wasteReason string) ([]dto.StockMovement, error) {
	// Restore only the order's net outstanding deduction per inventory item:
	// sales are negative and earlier restores are positive, so their sum is what
	// is still owed back. Replaying every historical sale movement instead would
	// over-restock when an order is restored more than once (edit, then cancel;
	// cancel, force-approve, re-process, cancel again).
	var outstanding []struct {
		InventoryItemID uuid.UUID `bun:"inventory_item_id"`
		Net             int       `bun:"net"`
	}
	if err := orderOutstandingQuery(tx, orderID).
		OrderExpr("inventory_item_id").
		Scan(ctx, &outstanding); err != nil {
		return nil, err
	}

	restored := make([]dto.StockMovement, 0, len(outstanding))

	for _, o := range outstanding {
		var invItem models.InventoryItem
		if err := tx.NewSelect().Model(&invItem).Where("id = ?", o.InventoryItemID).For("UPDATE").Scan(ctx); err != nil {
			return nil, translateInventoryError(err)
		}

		held := -o.Net
		back := held
		if restock != nil {
			back = max(0, min(restock[o.InventoryItemID], held))
		}
		lost := held - back

		if back > 0 {
			invItem.CurrentStock += back
			invItem.UpdatedAt = time.Now()
			if _, err := tx.NewUpdate().Model(&invItem).WherePK().Exec(ctx); err != nil {
				return nil, err
			}
			reason := "Devuelto al inventario (orden cancelada)"
			m, err := insertOrderMovement(ctx, tx, models.StockMovementTypeAdjustment, back, invItem, orderID, reason, createdByID)
			if err != nil {
				return nil, err
			}
			restored = append(restored, m)
		}
		if lost > 0 {
			reason := "Merma: no se devolvió al inventario (orden cancelada)"
			if wasteReason != "" {
				reason += ". " + wasteReason
			}
			m, err := insertOrderMovement(ctx, tx, models.StockMovementTypeWaste, lost, invItem, orderID, reason, createdByID)
			if err != nil {
				return nil, err
			}
			restored = append(restored, m)
		}
	}

	return restored, nil
}

// RecordEntry records a stock entry for an inventory item atomically.
func (r *InventoryBunRepository) RecordEntry(ctx context.Context, input RecordEntryInput) (dto.StockMovement, error) {
	var movement dto.StockMovement
	err := r.withTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		var innerErr error
		movement, innerErr = r.recordEntry(ctx, tx, input)
		return innerErr
	})
	return movement, err
}

func (r *InventoryBunRepository) recordEntry(ctx context.Context, tx bun.Tx, input RecordEntryInput) (dto.StockMovement, error) {
	var invItem models.InventoryItem
	if err := tx.NewSelect().Model(&invItem).Where("id = ?", input.InventoryItemID).For("UPDATE").Scan(ctx); err != nil {
		return dto.StockMovement{}, translateInventoryError(err)
	}

	invItem.CurrentStock += input.TotalBalls
	invItem.UpdatedAt = time.Now()
	if _, err := tx.NewUpdate().Model(&invItem).WherePK().Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}

	movement := models.StockMovement{
		ID:                 uuid.New(),
		Type:               models.StockMovementTypeEntry,
		Quantity:           input.TotalBalls,
		QuantityContainers: &input.QuantityContainers,
		InventoryItemID:    invItem.ID,
		LocationID:         invItem.LocationID,
		ContainerTypeID:    &input.ContainerTypeID,
		CreatedByUserID:    &input.CreatedByID,
		CreatedAt:          time.Now(),
	}
	if _, err := tx.NewInsert().Model(&movement).Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}

	return toDTOStockMovementFromModel(movement), nil
}

// RecordAdjustment records a manual stock adjustment atomically.
func (r *InventoryBunRepository) RecordAdjustment(ctx context.Context, input RecordAdjustmentInput) (dto.StockMovement, error) {
	var movement dto.StockMovement
	err := r.withTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		var innerErr error
		movement, innerErr = r.recordAdjustment(ctx, tx, input)
		return innerErr
	})
	return movement, err
}

func (r *InventoryBunRepository) recordAdjustment(ctx context.Context, tx bun.Tx, input RecordAdjustmentInput) (dto.StockMovement, error) {
	var invItem models.InventoryItem
	if err := tx.NewSelect().Model(&invItem).Where("id = ?", input.InventoryItemID).For("UPDATE").Scan(ctx); err != nil {
		return dto.StockMovement{}, translateInventoryError(err)
	}

	newStock := invItem.CurrentStock + input.Quantity
	if newStock < 0 {
		return dto.StockMovement{}, apperror.ErrConflict
	}

	invItem.CurrentStock = newStock
	invItem.UpdatedAt = time.Now()
	if _, err := tx.NewUpdate().Model(&invItem).WherePK().Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}

	movement := models.StockMovement{
		ID:              uuid.New(),
		Type:            models.StockMovementTypeAdjustment,
		Quantity:        input.Quantity,
		InventoryItemID: invItem.ID,
		LocationID:      invItem.LocationID,
		Reason:          &input.Reason,
		CreatedByUserID: &input.CreatedByID,
		CreatedAt:       time.Now(),
	}
	if _, err := tx.NewInsert().Model(&movement).Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}

	return toDTOStockMovementFromModel(movement), nil
}

// RecordUnitEntry records a stock entry for a unit-based inventory item atomically.
func (r *InventoryBunRepository) RecordUnitEntry(ctx context.Context, input RecordUnitEntryInput) (dto.StockMovement, error) {
	var movement dto.StockMovement
	err := r.withTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		var innerErr error
		movement, innerErr = r.recordUnitEntry(ctx, tx, input)
		return innerErr
	})
	return movement, err
}

func (r *InventoryBunRepository) recordUnitEntry(ctx context.Context, tx bun.Tx, input RecordUnitEntryInput) (dto.StockMovement, error) {
	var invItem models.InventoryItem
	if err := tx.NewSelect().Model(&invItem).Where("id = ?", input.InventoryItemID).For("UPDATE").Scan(ctx); err != nil {
		return dto.StockMovement{}, translateInventoryError(err)
	}

	invItem.CurrentStock += input.Quantity
	invItem.UpdatedAt = time.Now()
	if _, err := tx.NewUpdate().Model(&invItem).WherePK().Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}

	movement := models.StockMovement{
		ID:              uuid.New(),
		Type:            models.StockMovementTypeEntry,
		Quantity:        input.Quantity,
		InventoryItemID: invItem.ID,
		LocationID:      invItem.LocationID,
		CreatedByUserID: &input.CreatedByID,
		CreatedAt:       time.Now(),
	}
	if _, err := tx.NewInsert().Model(&movement).Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}

	return toDTOStockMovementFromModel(movement), nil
}

func translateInventoryError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &apperror.NotFoundError{Resource: "inventory_item", Cause: err}
	}
	return err
}

func toDTOInventoryItemFromModel(item models.InventoryItem) dto.InventoryItem {
	return dto.InventoryItem{
		ID:                item.ID,
		LocationID:        item.LocationID,
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

func toDTOInventoryItemsFromModels(items []models.InventoryItem) []dto.InventoryItem {
	result := make([]dto.InventoryItem, 0, len(items))
	for _, item := range items {
		result = append(result, toDTOInventoryItemFromModel(item))
	}
	return result
}

func toDTOStockMovementFromModel(movement models.StockMovement) dto.StockMovement {
	return dto.StockMovement{
		ID:                 movement.ID,
		Type:               string(movement.Type),
		Quantity:           movement.Quantity,
		QuantityContainers: movement.QuantityContainers,
		InventoryItemID:    movement.InventoryItemID,
		LocationID:         movement.LocationID,
		ContainerTypeID:    movement.ContainerTypeID,
		FlavorID:           movement.FlavorID,
		OrderID:            movement.OrderID,
		Reason:             movement.Reason,
		CreatedByUserID:    movement.CreatedByUserID,
		CreatedAt:          movement.CreatedAt,
	}
}

func toDTOStockMovementsFromModels(movements []models.StockMovement) []dto.StockMovement {
	result := make([]dto.StockMovement, 0, len(movements))
	for _, movement := range movements {
		result = append(result, toDTOStockMovementFromModel(movement))
	}
	return result
}

// insertOrderMovement records a stock movement tied to an order.
func insertOrderMovement(ctx context.Context, tx bun.Tx, t models.StockMovementType, qty int, item models.InventoryItem, orderID uuid.UUID, reason, createdByID string) (dto.StockMovement, error) {
	m := models.StockMovement{
		ID:              uuid.New(),
		Type:            t,
		Quantity:        qty,
		InventoryItemID: item.ID,
		LocationID:      item.LocationID,
		OrderID:         &orderID,
		Reason:          &reason,
		CreatedAt:       time.Now(),
	}
	if createdByID != "" {
		m.CreatedByUserID = &createdByID
	}
	if _, err := tx.NewInsert().Model(&m).Exec(ctx); err != nil {
		return dto.StockMovement{}, err
	}
	return toDTOStockMovementFromModel(m), nil
}
