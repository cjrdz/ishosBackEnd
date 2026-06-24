// order_bun.go — Bun-backed repository for order and order-item persistence.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// OrderBunRepository implements services.OrderRepository using Bun.
type OrderBunRepository struct {
	db *bun.DB
}

// NewOrderBunRepository creates a new Bun-backed order repository.
func NewOrderBunRepository(db *bun.DB) *OrderBunRepository {
	return &OrderBunRepository{db: db}
}

// Create creates a new order.
func (r *OrderBunRepository) Create(ctx context.Context, input CreateOrderInput) (dto.Order, error) {
	now := time.Now()
	order := models.Order{
		ID:            uuid.New(),
		OrderNumber:   input.OrderNumber,
		CustomerID:    input.CustomerID,
		CustomerName:  input.CustomerName,
		CustomerPhone: input.CustomerPhone,
		CustomerEmail: input.CustomerEmail,
		CustomerIP:    input.CustomerIP,
		TotalAmount:   input.TotalAmount,
		Status:        models.OrderStatus(input.Status),
		PaymentMethod: models.OrderPaymentMethod(input.PaymentMethod),
		PaymentStatus: models.OrderPaymentStatusPendiente,
		OrderType:     models.OrderType(input.OrderType),
		TableNumber:   input.TableNumber,
		Notes:         input.Notes,
		CreatedByUserID: input.CreatedByUserID,
		StatusTimestamps: map[string]time.Time{
			string(models.OrderStatusPendienteRevision): now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := r.db.NewInsert().Model(&order).Exec(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	return toOrderDTOFromModel(order, nil), nil
}

// CreateItem creates a new order item.
func (r *OrderBunRepository) CreateItem(ctx context.Context, input CreateOrderItemInput) (dto.OrderItem, error) {
	item := models.OrderItem{
		ID:                uuid.New(),
		OrderID:           input.OrderID,
		ProductID:         input.ProductID,
		SnapshotProductID: &input.ProductID,
		ProductName:       input.ProductName,
		Quantity:          input.Quantity,
		UnitPrice:         input.UnitPrice,
		Subtotal:          input.Subtotal,
		Customizations:    input.Customizations,
	}
	if _, err := r.db.NewInsert().Model(&item).Exec(ctx); err != nil {
		return dto.OrderItem{}, err
	}
	return toOrderItemDTOFromModel(item, nil), nil
}

// ListPaginated returns paginated orders with items.
func (r *OrderBunRepository) ListPaginated(ctx context.Context, status string, page, limit int) ([]dto.Order, int, error) {
	q := r.db.NewSelect().Model((*models.Order)(nil))
	if status != "" {
		q = q.Where("status = ?", status)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var orders []models.Order
	if err := q.Model(&orders).Order("created_at DESC").Limit(limit).Offset(offset).Scan(ctx); err != nil {
		return nil, 0, err
	}

	result := r.attachItemsToOrders(ctx, orders)
	return result, total, nil
}

// FindByIDWithItems returns an order with items and product details.
func (r *OrderBunRepository) FindByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	result := r.attachItemsToOrders(ctx, []models.Order{order})
	return result[0], nil
}

// FindByID returns an order by ID.
func (r *OrderBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	return toOrderDTOFromModel(order, nil), nil
}

// FindByOrderNumber returns an order by order number with items.
func (r *OrderBunRepository) FindByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("order_number = ?", orderNumber).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	result := r.attachItemsToOrders(ctx, []models.Order{order})
	return result[0], nil
}

// FindByOrderNumberAndPhone returns an order by order number and customer phone.
func (r *OrderBunRepository) FindByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("order_number = ? AND customer_phone = ?", orderNumber, customerPhone).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	return toOrderDTOFromModel(order, nil), nil
}

// FindRecentByPhone returns recent orders for a phone number.
func (r *OrderBunRepository) FindRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error) {
	q := r.db.NewSelect().Model((*models.Order)(nil)).Where("customer_phone = ?", customerPhone).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var orders []models.Order
	if err := q.Scan(ctx, &orders); err != nil {
		return nil, err
	}
	return toOrderDTOsFromModels(orders), nil
}

// Approve marks an order as approved.
func (r *OrderBunRepository) Approve(ctx context.Context, id uuid.UUID, reviewerID *string) (dto.Order, error) {
	return r.updateStatusWithTimestamps(ctx, id, string(models.OrderStatusRecibida), func(order *models.Order, now time.Time) {
		order.Status = models.OrderStatusRecibida
		order.PaymentStatus = models.OrderPaymentStatusConfirmado
		order.RejectionReason = nil
		order.ReviewedByUserID = reviewerID
		order.ReviewedAt = &now
	})
}

// Reject marks an order as rejected.
func (r *OrderBunRepository) Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string) (dto.Order, error) {
	return r.updateStatusWithTimestamps(ctx, id, string(models.OrderStatusCancelada), func(order *models.Order, now time.Time) {
		order.Status = models.OrderStatusCancelada
		order.RejectionReason = &reason
		order.ReviewedByUserID = reviewerID
		order.ReviewedAt = &now
	})
}

// UpdateStatus updates an order's status.
func (r *OrderBunRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (dto.Order, error) {
	return r.updateStatusWithTimestamps(ctx, id, status, func(order *models.Order, now time.Time) {
		order.Status = models.OrderStatus(status)
	})
}

func (r *OrderBunRepository) updateStatusWithTimestamps(ctx context.Context, id uuid.UUID, status string, mutate func(*models.Order, time.Time)) (dto.Order, error) {
	order, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	now := time.Now()
	model := models.Order{
		ID:               order.ID,
		OrderNumber:      order.OrderNumber,
		CustomerID:       order.CustomerID,
		CustomerName:     order.CustomerName,
		CustomerPhone:    order.CustomerPhone,
		CustomerEmail:    order.CustomerEmail,
		CustomerIP:       order.CustomerIP,
		TotalAmount:      order.TotalAmount,
		Status:           models.OrderStatus(order.Status),
		PaymentMethod:    models.OrderPaymentMethod(order.PaymentMethod),
		PaymentStatus:    models.OrderPaymentStatus(order.PaymentStatus),
		OrderType:        models.OrderType(order.OrderType),
		TableNumber:      order.TableNumber,
		Notes:            order.Notes,
		RejectionReason:  order.RejectionReason,
		ReviewedByUserID: order.ReviewedByUserID,
		ReviewedAt:       order.ReviewedAt,
		CreatedByUserID:  order.CreatedByUserID,
		IsArchived:       order.IsArchived,
		StatusTimestamps: order.StatusTimestamps,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        now,
	}
	if model.StatusTimestamps == nil {
		model.StatusTimestamps = make(map[string]time.Time)
	}
	model.StatusTimestamps[status] = now

	mutate(&model, now)

	res, err := r.db.NewUpdate().Model(&model).WherePK().Exec(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Order{}, &apperror.NotFoundError{Resource: "order"}
	}
	return toOrderDTOFromModel(model, nil), nil
}

// UpdateNotes updates an order's notes.
func (r *OrderBunRepository) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	order, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	model := models.Order{
		ID:     order.ID,
		Notes:  notes,
		UpdatedAt: time.Now(),
	}
	res, err := r.db.NewUpdate().Model(&model).Set("notes = ?, updated_at = ?", notes, model.UpdatedAt).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Order{}, &apperror.NotFoundError{Resource: "order"}
	}

	// Refetch to return consistent state.
	return r.FindByID(ctx, id)
}

// Delete deletes an order and its items.
func (r *OrderBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.NewDelete().Model((*models.OrderItem)(nil)).Where("order_items = ?", id).Exec(ctx); err != nil {
		return err
	}
	_, err := r.db.NewDelete().Model((*models.Order)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// FindByDateRange returns orders within a date range.
func (r *OrderBunRepository) FindByDateRange(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	q := r.db.NewSelect().Model((*models.Order)(nil)).Where("created_at >= ? AND created_at <= ?", start, end)
	if len(statuses) > 0 {
		q = q.Where("status IN (?)", bun.In(statuses))
	}

	var orders []models.Order
	if err := q.Scan(ctx, &orders); err != nil {
		return nil, err
	}
	return toOrderDTOsFromModels(orders), nil
}

// FindByDateRangeWithItems returns orders with items within a date range.
func (r *OrderBunRepository) FindByDateRangeWithItems(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	q := r.db.NewSelect().Model((*models.Order)(nil)).Where("created_at >= ? AND created_at <= ?", start, end)
	if len(statuses) > 0 {
		q = q.Where("status IN (?)", bun.In(statuses))
	}

	var orders []models.Order
	if err := q.Scan(ctx, &orders); err != nil {
		return nil, err
	}
	return r.attachItemsToOrders(ctx, orders), nil
}

// FindOrderItemsByOrderDateRange returns order items for orders in a date range excluding statuses.
func (r *OrderBunRepository) FindOrderItemsByOrderDateRange(ctx context.Context, start, end time.Time, excludeStatuses []string) ([]dto.OrderItem, error) {
	orderQ := r.db.NewSelect().Model((*models.Order)(nil)).Column("id").Where("created_at >= ? AND created_at <= ?", start, end)
	if len(excludeStatuses) > 0 {
		orderQ = orderQ.Where("status NOT IN (?)", bun.In(excludeStatuses))
	}

	var items []models.OrderItem
	if err := r.db.NewSelect().Model(&items).Where("order_items IN (?)", orderQ).Scan(ctx); err != nil {
		return nil, err
	}

	productIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		productIDs = append(productIDs, item.ProductID)
	}
	products := r.loadProductsByIDs(ctx, productIDs)

	result := make([]dto.OrderItem, len(items))
	for i, item := range items {
		result[i] = toOrderItemDTOFromModel(item, products[item.ProductID])
	}
	return result, nil
}

// Purge deletes orders and items within a date range matching statuses.
func (r *OrderBunRepository) Purge(ctx context.Context, start, end time.Time, statuses []string) (int, int, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	q := tx.NewSelect().Model((*models.Order)(nil)).Column("id").Where("created_at >= ? AND created_at <= ?", start, end)
	if len(statuses) > 0 {
		q = q.Where("status IN (?)", bun.In(statuses))
	}

	var ids []uuid.UUID
	if err := q.Scan(ctx, &ids); err != nil {
		return 0, 0, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}

	resItems, err := tx.NewDelete().Model((*models.OrderItem)(nil)).Where("order_items IN (?)", bun.In(ids)).Exec(ctx)
	if err != nil {
		return 0, 0, err
	}
	deletedItems, _ := resItems.RowsAffected()

	resOrders, err := tx.NewDelete().Model((*models.Order)(nil)).Where("id IN (?)", bun.In(ids)).Exec(ctx)
	if err != nil {
		return 0, 0, err
	}
	deletedOrders, _ := resOrders.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return int(deletedOrders), int(deletedItems), nil
}

func (r *OrderBunRepository) attachItemsToOrders(ctx context.Context, orders []models.Order) []dto.Order {
	if len(orders) == 0 {
		return nil
	}

	orderIDs := make([]uuid.UUID, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}

	var items []models.OrderItem
	if err := r.db.NewSelect().Model(&items).Where("order_items IN (?)", bun.In(orderIDs)).Scan(ctx); err != nil {
		// Fall back to empty items on error; caller will return the error if needed.
		items = nil
	}

	productIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		productIDs = append(productIDs, item.ProductID)
	}
	products := r.loadProductsByIDs(ctx, productIDs)

	itemsByOrder := make(map[uuid.UUID][]dto.OrderItem)
	for _, item := range items {
		dtoItem := toOrderItemDTOFromModel(item, products[item.ProductID])
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], dtoItem)
	}

	result := make([]dto.Order, len(orders))
	for i, o := range orders {
		result[i] = toOrderDTOFromModel(o, itemsByOrder[o.ID])
	}
	return result
}

func (r *OrderBunRepository) loadProductsByIDs(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]*dto.Product {
	result := make(map[uuid.UUID]*dto.Product)
	if len(ids) == 0 {
		return result
	}

	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{})
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}

	var products []models.Product
	if err := r.db.NewSelect().Model(&products).Where("id IN (?)", bun.In(unique)).Scan(ctx); err != nil {
		return result
	}

	for _, p := range products {
		prod := toProductDTOFromModel(p, nil)
		result[p.ID] = &prod
	}
	return result
}

func translateOrderError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &apperror.NotFoundError{Resource: "order", Cause: err}
	}
	return err
}

func toOrderDTOsFromModels(orders []models.Order) []dto.Order {
	result := make([]dto.Order, len(orders))
	for i, o := range orders {
		result[i] = toOrderDTOFromModel(o, nil)
	}
	return result
}
