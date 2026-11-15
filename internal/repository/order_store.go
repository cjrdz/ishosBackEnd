// order_store.go — Transactional order persistence seam.
//
// OrderStore hides transaction orchestration, product validation, and
// customization enrichment behind a small interface so that services do not
// need to open transactions or import builders.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/auditctx"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

const idempotencyKeyTTL = 30 * 24 * time.Hour // 30 days

// StoreOrderItemInput is the raw order item supplied to OrderStore.
type StoreOrderItemInput struct {
	ProductID      uuid.UUID
	Quantity       int
	Customizations map[string]any
}

// StoreOrderInput is the raw data required to create an order transactionally.
type StoreOrderInput struct {
	OrderNumber string
	// LocationID is the sucursal the order belongs to (required).
	LocationID     uuid.UUID
	IdempotencyKey *uuid.UUID
	CustomerID     *uuid.UUID
	CustomerName   string
	CustomerPhone  string
	CustomerEmail  *string
	CustomerIP     *string
	PaymentMethod  string
	AmountReceived *float64
	OrderType      string
	TableNumber    *int
	Notes          *string
	CreatedByID    *string
	Items          []StoreOrderItemInput
	// BasePrices overrides a product's base price (active store offers).
	BasePrices map[uuid.UUID]float64
	// Accepted creates the order already accepted ("recibida"), reviewed by
	// CreatedByID: counter orders need no review. Online orders leave it false
	// and start in "pendiente_revision".
	Accepted bool
}

// StoreUpdateOrderInput is the raw data required to update an order transactionally.
type StoreUpdateOrderInput struct {
	CustomerID *uuid.UUID
	// ClearCustomer unlinks the customer record (the phone was removed).
	ClearCustomer  bool
	CustomerName   string
	CustomerPhone  string
	CustomerEmail  *string
	PaymentMethod  string
	AmountReceived *float64
	OrderType      string
	TableNumber    *int
	Notes          *string
	Items          []StoreOrderItemInput
	// ExpectedStatus, when non-empty, must match the order's status under the
	// row lock or the update fails with a conflict.
	ExpectedStatus string
	// BasePrices overrides a product's base price (active store offers).
	BasePrices map[uuid.UUID]float64
}

// EnrichmentService validates and enriches order customizations with pricing.
// The production implementation lives in the services package; this interface
// lets OrderStore stay in the repository package without creating an import cycle.
type EnrichmentService interface {
	EnrichCustomizations(product dto.Product, raw map[string]any) (map[string]any, float64, error)
}

// productResolver is the minimal surface OrderStore needs from product lookup.
type productResolver interface {
	FindByIDWithCategoryAndOptions(ctx context.Context, id uuid.UUID) (dto.Product, error)
}

// OrderStore implements transactional order creation and update.
type OrderStore struct {
	db          bun.IDB
	productRepo productResolver
	enrichment  EnrichmentService
	auditSvc    transactionalAuditLogger
}

// NewOrderStore creates a new transactional order store.
// An optional audit service can be supplied to enable transactional audit
// logging for mutating operations.
func NewOrderStore(db bun.IDB, productRepo productResolver, enrichment EnrichmentService, auditSvc ...transactionalAuditLogger) *OrderStore {
	s := &OrderStore{db: db, productRepo: productRepo, enrichment: enrichment}
	if len(auditSvc) > 0 {
		s.auditSvc = auditSvc[0]
	}
	return s
}

// validateAmountReceived returns a validation error when the provided cash
// amount is insufficient to cover the order total.
func validateAmountReceived(amountReceived *float64, total float64) error {
	if amountReceived != nil && *amountReceived < total {
		return &apperror.ValidationError{Fields: map[string]string{"amount_received": "El monto recibido no cubre el total de la orden"}}
	}
	return nil
}

func (s *OrderStore) maybeLog(ctx context.Context, tx bun.Tx, orderID uuid.UUID) error {
	if s.auditSvc == nil {
		return nil
	}
	spec, ok := auditctx.LogSpecFromContext(ctx)
	if !ok {
		return nil
	}
	return s.auditSvc.LogWithDB(ctx, tx, spec.Action, spec.ActorID, fmt.Sprintf("order:%s", orderID), spec.Changes, spec.IP)
}

// FindOrderByIdempotencyKey returns the order previously created for the
// given idempotency key, if one exists. Returns apperror.ErrNotFound when
// no order is associated with the key yet.
func (s *OrderStore) FindOrderByIdempotencyKey(ctx context.Context, key uuid.UUID) (dto.Order, error) {
	var ik models.IdempotencyKey
	if err := s.db.NewSelect().Model(&ik).Where("key = ?", key).Scan(ctx); err != nil {
		return dto.Order{}, &apperror.NotFoundError{Resource: "idempotency_key", Cause: err}
	}

	var order models.Order
	if err := s.db.NewSelect().Model(&order).Where("id = ?", ik.OrderID).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	var items []models.OrderItem
	if err := s.db.NewSelect().Model(&items).Where("order_items = ?", ik.OrderID).Scan(ctx); err != nil {
		return dto.Order{}, err
	}

	dtoItems := make([]dto.OrderItem, len(items))
	for i, item := range items {
		dtoItems[i] = toOrderItemDTOFromModel(item, nil)
	}
	return toOrderDTOFromModel(order, dtoItems), nil
}

// CreateOrder creates a new order with its items inside a single transaction.
// It validates products, enriches customizations, and computes the total amount.
func (s *OrderStore) CreateOrder(ctx context.Context, input StoreOrderInput) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, s.db, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.LocationID == uuid.Nil {
			return &apperror.ValidationError{Fields: map[string]string{"location_id": "Elige una sucursal"}}
		}
		total, snapshots, err := s.resolveItems(ctx, tx, input.LocationID, input.Items, input.BasePrices)
		if err != nil {
			return err
		}

		if err := validateAmountReceived(input.AmountReceived, total); err != nil {
			return err
		}

		now := time.Now()
		order := models.Order{
			ID:              uuid.New(),
			OrderNumber:     input.OrderNumber,
			LocationID:      input.LocationID,
			CustomerID:      input.CustomerID,
			CustomerName:    input.CustomerName,
			CustomerPhone:   input.CustomerPhone,
			CustomerEmail:   input.CustomerEmail,
			CustomerIP:      input.CustomerIP,
			TotalAmount:     total,
			AmountReceived:  input.AmountReceived,
			Status:          models.OrderStatusPendienteRevision,
			PaymentMethod:   models.OrderPaymentMethod(input.PaymentMethod),
			PaymentStatus:   models.OrderPaymentStatusPendiente,
			OrderType:       models.OrderType(input.OrderType),
			TableNumber:     input.TableNumber,
			Notes:           input.Notes,
			CreatedByUserID: input.CreatedByID,
			StatusTimestamps: map[string]time.Time{
				string(models.OrderStatusPendienteRevision): now,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if input.Accepted {
			order.Status = models.OrderStatusRecibida
			order.StatusTimestamps[string(models.OrderStatusRecibida)] = now
			order.ReviewedByUserID = input.CreatedByID
			order.ReviewedAt = &now
		}

		if _, err := tx.NewInsert().Model(&order).Exec(ctx); err != nil {
			return err
		}

		// Record the idempotency key in the same transaction as the order. A
		// unique-constraint violation here means a concurrent request with the
		// same key already won the race; the caller can fetch the winning order.
		if input.IdempotencyKey != nil {
			ikRow := models.IdempotencyKey{
				Key:       *input.IdempotencyKey,
				OrderID:   order.ID,
				CreatedAt: now,
				ExpiresAt: now.Add(idempotencyKeyTTL),
			}
			if _, err := tx.NewInsert().Model(&ikRow).Exec(ctx); err != nil {
				return err
			}
		}

		if err := s.createItems(ctx, tx, order.ID, snapshots); err != nil {
			return err
		}

		if err := s.maybeLog(ctx, tx, order.ID); err != nil {
			return err
		}

		result = s.toDTOOrder(order, snapshots)
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// UpdateOrder updates an order and replaces its items inside a single transaction.
// When Items is empty, only the order metadata is updated and existing items are kept.
func (s *OrderStore) UpdateOrder(ctx context.Context, id uuid.UUID, input StoreUpdateOrderInput) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, s.db, nil, func(ctx context.Context, tx bun.Tx) error {
		existingOrder, snapshots, err := s.updateOrderOnDB(ctx, tx, id, input)
		if err != nil {
			return err
		}
		if err := s.maybeLog(ctx, tx, id); err != nil {
			return err
		}
		result = s.toDTOOrder(existingOrder, snapshots)
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// UpdateOrderWithStockAdjustment updates an order and replaces its items, then
// applies an inventory adjustment inside the same transaction. This keeps order
// data and stock consistent when an in-progress order's items change.
func (s *OrderStore) UpdateOrderWithStockAdjustment(ctx context.Context, id uuid.UUID, input StoreUpdateOrderInput, deductions []StockDeductionInput, restore bool) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, s.db, nil, func(ctx context.Context, tx bun.Tx) error {
		existingOrder, snapshots, err := s.updateOrderOnDB(ctx, tx, id, input)
		if err != nil {
			return err
		}

		if err := applyStockAdjustment(ctx, tx, id, deductions, restore); err != nil {
			return err
		}

		if err := s.maybeLog(ctx, tx, id); err != nil {
			return err
		}

		result = s.toDTOOrder(existingOrder, snapshots)
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// updateOrderOnDB performs the actual order metadata and item replacement
// against the supplied DB or transaction. Callers are responsible for committing.
func (s *OrderStore) updateOrderOnDB(ctx context.Context, db bun.IDB, id uuid.UUID, input StoreUpdateOrderInput) (models.Order, []itemSnapshot, error) {
	var existingOrder models.Order
	if err := db.NewSelect().Model(&existingOrder).Where("id = ?", id).For("UPDATE").Scan(ctx); err != nil {
		return models.Order{}, nil, translateOrderError(err)
	}
	if input.ExpectedStatus != "" && string(existingOrder.Status) != input.ExpectedStatus {
		return models.Order{}, nil, fmt.Errorf("%w: la orden cambió de %s a %s mientras la editabas; actualiza e intenta de nuevo",
			apperror.ErrConflict, input.ExpectedStatus, existingOrder.Status)
	}

	now := time.Now()
	existingOrder.CustomerName = input.CustomerName
	existingOrder.CustomerPhone = input.CustomerPhone
	existingOrder.CustomerEmail = input.CustomerEmail
	existingOrder.PaymentMethod = models.OrderPaymentMethod(input.PaymentMethod)
	existingOrder.OrderType = models.OrderType(input.OrderType)
	existingOrder.TableNumber = input.TableNumber
	existingOrder.Notes = input.Notes
	existingOrder.UpdatedAt = now
	if input.CustomerID != nil {
		existingOrder.CustomerID = input.CustomerID
	} else if input.ClearCustomer {
		existingOrder.CustomerID = nil
	}

	if input.PaymentMethod != "efectivo" {
		existingOrder.AmountReceived = nil
	} else {
		existingOrder.AmountReceived = input.AmountReceived
	}

	if len(input.Items) == 0 {
		if err := validateAmountReceived(existingOrder.AmountReceived, existingOrder.TotalAmount); err != nil {
			return models.Order{}, nil, err
		}
		if _, err := db.NewUpdate().Model(&existingOrder).WherePK().Exec(ctx); err != nil {
			return models.Order{}, nil, err
		}
		return existingOrder, nil, nil
	}

	total, snapshots, err := s.resolveItems(ctx, db, existingOrder.LocationID, input.Items, input.BasePrices)
	if err != nil {
		return models.Order{}, nil, err
	}
	existingOrder.TotalAmount = total

	if err := validateAmountReceived(existingOrder.AmountReceived, total); err != nil {
		return models.Order{}, nil, err
	}

	if _, err := db.NewUpdate().Model(&existingOrder).WherePK().Exec(ctx); err != nil {
		return models.Order{}, nil, err
	}

	if _, err := db.NewDelete().Model((*models.OrderItem)(nil)).Where("order_items = ?", existingOrder.ID).Exec(ctx); err != nil {
		return models.Order{}, nil, err
	}

	if err := s.createItems(ctx, db, existingOrder.ID, snapshots); err != nil {
		return models.Order{}, nil, err
	}

	return existingOrder, snapshots, nil
}

// UpdateStatus updates an order's status.
//
// expectedStatus, when non-empty, is the status the caller validated the
// transition against; the update fails with a conflict if the order has moved
// on since (e.g. a concurrent request from another device).
func (s *OrderStore) UpdateStatus(ctx context.Context, id uuid.UUID, status, expectedStatus string) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, s.db, nil, func(ctx context.Context, tx bun.Tx) error {
		updated, err := s.updateStatusOnDB(ctx, tx, id, status, expectedStatus)
		if err != nil {
			return err
		}
		if err := s.maybeLog(ctx, tx, id); err != nil {
			return err
		}
		result = updated
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// UpdateStatusWithStockAdjustment updates an order's status and applies an
// inventory adjustment inside the same transaction so that stock and status are
// committed atomically. If the adjustment fails, the status change is rolled back.
//
// When restore is set the order's outstanding stock is returned first, then any
// deductions are applied. expectedStatus behaves as in UpdateStatus.
func (s *OrderStore) UpdateStatusWithStockAdjustment(ctx context.Context, id uuid.UUID, status, expectedStatus string, deductions []StockDeductionInput, restore bool) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, s.db, nil, func(ctx context.Context, tx bun.Tx) error {
		updated, err := s.updateStatusOnDB(ctx, tx, id, status, expectedStatus)
		if err != nil {
			return err
		}

		if err := applyStockAdjustment(ctx, tx, id, deductions, restore); err != nil {
			return err
		}

		if err := s.maybeLog(ctx, tx, id); err != nil {
			return err
		}

		result = updated
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// updateStatusOnDB updates an order's status against the supplied DB or transaction.
// The row is locked for the rest of the transaction and only the status columns
// are written, so concurrent edits to other fields (e.g. amount_received) are kept.
func (s *OrderStore) updateStatusOnDB(ctx context.Context, db bun.IDB, id uuid.UUID, status, expectedStatus string) (dto.Order, error) {
	order, err := lockOrderForStatusChange(ctx, db, id, expectedStatus)
	if err != nil {
		return dto.Order{}, err
	}

	now := time.Now()
	if order.StatusTimestamps == nil {
		order.StatusTimestamps = make(map[string]time.Time)
	}
	order.StatusTimestamps[status] = now
	order.Status = models.OrderStatus(status)
	order.UpdatedAt = now

	res, err := db.NewUpdate().Model(&order).Column("status", "status_timestamps", "updated_at").WherePK().Exec(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Order{}, &apperror.NotFoundError{Resource: "order"}
	}
	return toOrderDTOFromModel(order, nil), nil
}

// lockOrderForStatusChange loads an order with SELECT ... FOR UPDATE and, when
// expectedStatus is non-empty, verifies it still has that status. Callers must
// be inside a transaction for the lock to be meaningful.
func lockOrderForStatusChange(ctx context.Context, db bun.IDB, id uuid.UUID, expectedStatus string) (models.Order, error) {
	var order models.Order
	if err := db.NewSelect().Model(&order).Where("id = ?", id).For("UPDATE").Scan(ctx); err != nil {
		return models.Order{}, translateOrderError(err)
	}
	if expectedStatus != "" && string(order.Status) != expectedStatus {
		return models.Order{}, fmt.Errorf("%w: la orden cambió de %s a %s mientras la editabas; actualiza e intenta de nuevo",
			apperror.ErrConflict, expectedStatus, order.Status)
	}
	return order, nil
}

// applyStockAdjustment returns the order's outstanding stock (when restore is
// set) and then applies any new deductions, inside the caller's transaction.
// Restore is net-based and idempotent, so restoring before deducting is always
// safe, including when an in-progress order's items are replaced.
func applyStockAdjustment(ctx context.Context, tx bun.Tx, orderID uuid.UUID, deductions []StockDeductionInput, restore bool) error {
	invRepo := NewInventoryBunRepository(tx)
	if restore {
		if _, err := invRepo.RestoreStockForOrder(ctx, orderID, ""); err != nil {
			return err
		}
	}
	if len(deductions) > 0 {
		if _, err := invRepo.DeductStockForOrder(ctx, orderID, deductions, ""); err != nil {
			return err
		}
	}
	return nil
}

// itemSnapshot holds resolved item data ready for persistence.
type itemSnapshot struct {
	product        dto.Product
	quantity       int
	unitPrice      float64
	customizations map[string]any
}

// resolveItems validates products, enriches customizations, and computes
// pricing. basePrices, when it has an entry for a product, replaces that
// product's base price (active offers); add-on extras are charged on top.
// Products and flavours marked sold out ("Agotado") at locationID are refused.
func (s *OrderStore) resolveItems(ctx context.Context, db bun.IDB, locationID uuid.UUID, items []StoreOrderItemInput, basePrices map[uuid.UUID]float64) (float64, []itemSnapshot, error) {
	total := 0.0
	snapshots := make([]itemSnapshot, 0, len(items))

	soldOut, err := loadSoldOut(ctx, db, locationID)
	if err != nil {
		return 0, nil, err
	}
	soldOutProducts := make(map[uuid.UUID]bool, len(soldOut.Products))
	for _, id := range soldOut.Products {
		soldOutProducts[id] = true
	}
	soldOutFlavors := make(map[uuid.UUID]bool, len(soldOut.Flavors))
	for _, id := range soldOut.Flavors {
		soldOutFlavors[id] = true
	}

	for _, item := range items {
		p, err := s.productRepo.FindByIDWithCategoryAndOptions(ctx, item.ProductID)
		if err != nil {
			return 0, nil, fmt.Errorf("%w: un producto de la orden ya no existe; quítalo e intenta de nuevo", apperror.ErrNotFound)
		}
		if !p.IsAvailable || soldOutProducts[p.ID] {
			return 0, nil, &apperror.SoldOutError{Name: p.Name}
		}

		enriched, addonExtra, err := s.enrichment.EnrichCustomizations(p, item.Customizations)
		if err != nil {
			return 0, nil, err
		}
		for _, flavorID := range customizationFlavorIDs(enriched) {
			if soldOutFlavors[flavorID] {
				return 0, nil, &apperror.SoldOutError{Name: soldOutFlavorName(enriched, flavorID)}
			}
		}
		basePrice := p.Price
		if offerPrice, ok := basePrices[p.ID]; ok && offerPrice > 0 {
			basePrice = offerPrice
		}
		effectivePrice := basePrice + addonExtra

		total += effectivePrice * float64(item.Quantity)
		snapshots = append(snapshots, itemSnapshot{
			product:        p,
			quantity:       item.Quantity,
			unitPrice:      effectivePrice,
			customizations: enriched,
		})
	}

	return total, snapshots, nil
}

// createItems persists resolved item snapshots against an open transaction using
// a single bulk insert instead of one query per item.
func (s *OrderStore) createItems(ctx context.Context, db bun.IDB, orderID uuid.UUID, snapshots []itemSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	items := make([]models.OrderItem, len(snapshots))
	for i, item := range snapshots {
		items[i] = models.OrderItem{
			ID:                uuid.New(),
			OrderID:           orderID,
			ProductID:         item.product.ID,
			SnapshotProductID: &item.product.ID,
			ProductName:       item.product.Name,
			Quantity:          item.quantity,
			UnitPrice:         item.unitPrice,
			Subtotal:          item.unitPrice * float64(item.quantity),
			Customizations:    item.customizations,
		}
	}

	if _, err := db.NewInsert().Model(&items).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// CleanupExpiredIdempotencyKeys removes idempotency keys past their TTL.
func (s *OrderStore) CleanupExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.NewDelete().Model((*models.IdempotencyKey)(nil)).Where("expires_at <= ?", now.UTC()).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired idempotency keys: %w", err)
	}
	return res.RowsAffected()
}

// toDTOOrder builds a dto.Order from a persisted models.Order and resolved snapshots.
func (s *OrderStore) toDTOOrder(order models.Order, snapshots []itemSnapshot) dto.Order {
	items := make([]dto.OrderItem, 0, len(snapshots))
	for _, snap := range snapshots {
		items = append(items, dto.OrderItem{
			OrderID:        order.ID,
			ProductID:      snap.product.ID,
			ProductName:    snap.product.Name,
			Quantity:       snap.quantity,
			UnitPrice:      snap.unitPrice,
			Subtotal:       snap.unitPrice * float64(snap.quantity),
			Customizations: snap.customizations,
			Product:        &snap.product,
		})
	}

	return dto.Order{
		ID:               order.ID,
		OrderNumber:      order.OrderNumber,
		LocationID:       order.LocationID,
		CustomerID:       order.CustomerID,
		CustomerName:     order.CustomerName,
		CustomerPhone:    order.CustomerPhone,
		CustomerEmail:    order.CustomerEmail,
		CustomerIP:       order.CustomerIP,
		TotalAmount:      order.TotalAmount,
		AmountReceived:   order.AmountReceived,
		Status:           string(order.Status),
		PaymentMethod:    string(order.PaymentMethod),
		PaymentStatus:    string(order.PaymentStatus),
		OrderType:        string(order.OrderType),
		TableNumber:      order.TableNumber,
		Notes:            order.Notes,
		RejectionReason:  order.RejectionReason,
		ReviewedByUserID: order.ReviewedByUserID,
		ReviewedAt:       order.ReviewedAt,
		CreatedByUserID:  order.CreatedByUserID,
		IsArchived:       order.IsArchived,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
		StatusTimestamps: order.StatusTimestamps,
		Items:            items,
	}
}

// customizationFlavorIDs reads the flavour IDs an item uses ("flavor_ids" or
// the legacy single "flavor_id").
func customizationFlavorIDs(customizations map[string]any) []uuid.UUID {
	var ids []uuid.UUID
	add := func(v any) {
		if str, ok := v.(string); ok {
			if id, err := uuid.Parse(str); err == nil {
				ids = append(ids, id)
			}
		}
	}
	switch v := customizations["flavor_ids"].(type) {
	case []any:
		for _, x := range v {
			add(x)
		}
	case []string:
		for _, x := range v {
			add(x)
		}
	}
	add(customizations["flavor_id"])
	return ids
}

// soldOutFlavorName finds a flavour's display name in enriched customizations
// ("flavor_names" parallels "flavor_ids"); empty when unknown.
func soldOutFlavorName(customizations map[string]any, flavorID uuid.UUID) string {
	ids := customizationFlavorIDs(customizations)
	var names []string
	switch v := customizations["flavor_names"].(type) {
	case []string:
		names = v
	case []any:
		for _, n := range v {
			if s, ok := n.(string); ok {
				names = append(names, s)
			}
		}
	}
	for i, id := range ids {
		if id == flavorID && i < len(names) {
			return "El sabor " + names[i]
		}
	}
	if name, ok := customizations["flavor_name"].(string); ok && name != "" {
		return "El sabor " + name
	}
	return ""
}
