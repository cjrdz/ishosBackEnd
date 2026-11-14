// order_bun.go — Bun-backed repository for order and order-item persistence.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/auditctx"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// transactionalAuditLogger is the subset of audit.Service needed by repositories
// to write audit records inside an open transaction. It is declared locally to
// avoid an import cycle with the audit package.
type transactionalAuditLogger interface {
	LogWithDB(ctx context.Context, db bun.IDB, action, actorID, resource string, changes map[string]any, ip string) error
}

// OrderBunRepository implements services.OrderRepository using Bun.
type OrderBunRepository struct {
	db       bun.IDB
	auditSvc transactionalAuditLogger
	// storeTimeZone (IANA name) is applied to analytics transactions so
	// day/week/month buckets follow the store's calendar, not UTC.
	storeTimeZone string
}

// revenueStatusesSQL lists the statuses whose totals count as revenue: orders
// staff have approved. Pending-review orders may still be rejected, and
// cancelled ones were never paid.
const revenueStatusesSQL = "status IN ('recibida', 'en_proceso', 'lista', 'entregada')"

// WithStoreTimeZone sets the IANA time zone used to bucket analytics by
// calendar day. It returns the repository for chaining.
func (r *OrderBunRepository) WithStoreTimeZone(name string) *OrderBunRepository {
	r.storeTimeZone = name
	return r
}

// applyStoreTimeZone sets the session time zone for the rest of tx (SET LOCAL
// semantics), so to_char/date_trunc on timestamptz use store-local time.
func (r *OrderBunRepository) applyStoreTimeZone(ctx context.Context, tx bun.Tx) error {
	if r.storeTimeZone == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "SELECT set_config('TimeZone', ?, true)", r.storeTimeZone); err != nil {
		return fmt.Errorf("set analytics time zone: %w", err)
	}
	return nil
}

// NewOrderBunRepository creates a new Bun-backed order repository.
// An optional audit service can be supplied to enable transactional audit
// logging for mutating operations.
func NewOrderBunRepository(db bun.IDB, auditSvc ...transactionalAuditLogger) *OrderBunRepository {
	r := &OrderBunRepository{db: db}
	if len(auditSvc) > 0 {
		r.auditSvc = auditSvc[0]
	}
	return r
}

func (r *OrderBunRepository) maybeLog(ctx context.Context, tx bun.Tx, resource string) error {
	if r.auditSvc == nil {
		return nil
	}
	spec, ok := auditctx.LogSpecFromContext(ctx)
	if !ok {
		return nil
	}
	return r.auditSvc.LogWithDB(ctx, tx, spec.Action, spec.ActorID, resource, spec.Changes, spec.IP)
}

func (r *OrderBunRepository) findOrderModelByID(ctx context.Context, db bun.IDB, id uuid.UUID) (models.Order, error) {
	var order models.Order
	if err := db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return models.Order{}, translateOrderError(err)
	}
	return order, nil
}

// Create creates a new order.
func (r *OrderBunRepository) Create(ctx context.Context, input CreateOrderInput) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		now := time.Now()
		order := models.Order{
			ID:              uuid.New(),
			OrderNumber:     input.OrderNumber,
			CustomerID:      input.CustomerID,
			CustomerName:    input.CustomerName,
			CustomerPhone:   input.CustomerPhone,
			CustomerEmail:   input.CustomerEmail,
			CustomerIP:      input.CustomerIP,
			TotalAmount:     input.TotalAmount,
			AmountReceived:  input.AmountReceived,
			Status:          models.OrderStatus(input.Status),
			PaymentMethod:   models.OrderPaymentMethod(input.PaymentMethod),
			PaymentStatus:   models.OrderPaymentStatusPendiente,
			OrderType:       models.OrderType(input.OrderType),
			TableNumber:     input.TableNumber,
			Notes:           input.Notes,
			CreatedByUserID: input.CreatedByUserID,
			StatusTimestamps: map[string]time.Time{
				string(models.OrderStatusPendienteRevision): now,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		if _, err := tx.NewInsert().Model(&order).Exec(ctx); err != nil {
			return translateOrderError(err)
		}

		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", order.ID)); err != nil {
			return err
		}

		result = toOrderDTOFromModel(order, nil)
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
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
// Product details are not loaded because the admin list response only needs
// the snapshot product_name stored on each item; skipping the products query
// removes one Neon round-trip per request.
// archived=false excludes orders that have been archived; archived=true returns
// only archived orders.
func (r *OrderBunRepository) ListPaginated(ctx context.Context, status string, archived bool, page, limit int) ([]dto.Order, int, error) {
	q := r.db.NewSelect().Model((*models.Order)(nil)).Where("is_archived = ?", archived)
	// status is a single status or a comma-separated list (validated by the
	// handler), e.g. the active board columns.
	if statuses := strings.Split(status, ","); status != "" && len(statuses) > 1 {
		q = q.Where("status IN (?)", bun.List(statuses))
	} else if status != "" {
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

// FindByIDWithItems returns an order with items. Product details are omitted
// because the admin detail response uses the snapshot product_name only.
func (r *OrderBunRepository) FindByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	result := r.attachItemsToOrders(ctx, []models.Order{order})
	return result[0], nil
}

// FindByIDWithItemsAndProducts returns an order with items and the associated
// product records. Use this when downstream logic (e.g. inventory deductions)
// needs full product metadata.
func (r *OrderBunRepository) FindByIDWithItemsAndProducts(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	result := r.attachItemsAndProductsToOrders(ctx, []models.Order{order})
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

// Approve marks an order as approved. expectedStatus, when non-empty, must match
// the order's status under the row lock or the update fails with a conflict.
func (r *OrderBunRepository) Approve(ctx context.Context, id uuid.UUID, reviewerID *string, expectedStatus string) (dto.Order, error) {
	var result dto.Order
	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		updated, err := r.updateStatusWithTimestamps(ctx, tx, id, string(models.OrderStatusRecibida), expectedStatus, func(order *models.Order, now time.Time) {
			order.Status = models.OrderStatusRecibida
			order.PaymentStatus = models.OrderPaymentStatusConfirmado
			order.RejectionReason = nil
			order.ReviewedByUserID = reviewerID
			order.ReviewedAt = &now
		})
		if err != nil {
			return err
		}
		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", id)); err != nil {
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

// Reject marks an order as rejected (cancelada) and returns any stock the order
// still holds, in the same transaction. Restore is net-based, so rejecting an
// order that never had stock deducted is a no-op for inventory.
// expectedStatus behaves as in Approve.
func (r *OrderBunRepository) Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string, expectedStatus string) (dto.Order, error) {
	var result dto.Order
	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		updated, err := r.updateStatusWithTimestamps(ctx, tx, id, string(models.OrderStatusCancelada), expectedStatus, func(order *models.Order, now time.Time) {
			order.Status = models.OrderStatusCancelada
			order.RejectionReason = &reason
			order.ReviewedByUserID = reviewerID
			order.ReviewedAt = &now
		})
		if err != nil {
			return err
		}
		if err := applyStockAdjustment(ctx, tx, id, nil, true); err != nil {
			return err
		}
		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", id)); err != nil {
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

// UpdateStatus updates an order's status. expectedStatus behaves as in Approve.
func (r *OrderBunRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, expectedStatus string) (dto.Order, error) {
	var result dto.Order
	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		updated, err := r.updateStatusWithTimestamps(ctx, tx, id, status, expectedStatus, func(order *models.Order, now time.Time) {
			order.Status = models.OrderStatus(status)
		})
		if err != nil {
			return err
		}
		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", id)); err != nil {
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

// updateStatusWithTimestamps locks the order row, applies mutate, and writes back
// only the review/status columns mutate may touch, so unrelated fields such as
// amount_received are never overwritten.
func (r *OrderBunRepository) updateStatusWithTimestamps(ctx context.Context, db bun.IDB, id uuid.UUID, status, expectedStatus string, mutate func(*models.Order, time.Time)) (dto.Order, error) {
	order, err := lockOrderForStatusChange(ctx, db, id, expectedStatus)
	if err != nil {
		return dto.Order{}, err
	}

	now := time.Now()
	order.UpdatedAt = now
	if order.StatusTimestamps == nil {
		order.StatusTimestamps = make(map[string]time.Time)
	}
	order.StatusTimestamps[status] = now

	mutate(&order, now)

	res, err := db.NewUpdate().
		Model(&order).
		Column("status", "payment_status", "rejection_reason", "reviewed_by_ory_id", "reviewed_at", "status_timestamps", "updated_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Order{}, &apperror.NotFoundError{Resource: "order"}
	}
	return toOrderDTOFromModel(order, nil), nil
}

// UpdateNotes updates an order's notes.
func (r *OrderBunRepository) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		order, err := r.findOrderModelByID(ctx, tx, id)
		if err != nil {
			return err
		}

		model := models.Order{
			ID:        order.ID,
			Notes:     notes,
			UpdatedAt: time.Now(),
		}
		res, err := tx.NewUpdate().Model(&model).Set("notes = ?, updated_at = ?", notes, model.UpdatedAt).Where("id = ?", id).Exec(ctx)
		if err != nil {
			return translateOrderError(err)
		}
		if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
			return &apperror.NotFoundError{Resource: "order"}
		}

		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", id)); err != nil {
			return err
		}

		// Refetch to return consistent state.
		refetched, err := r.findOrderModelByID(ctx, tx, id)
		if err != nil {
			return err
		}
		result = toOrderDTOFromModel(refetched, nil)
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// SetArchived updates an order's archive flag and returns the refreshed order.
func (r *OrderBunRepository) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error) {
	var result dto.Order

	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().
			Model((*models.Order)(nil)).
			Set("is_archived = ?, updated_at = ?", archived, time.Now()).
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			return translateOrderError(err)
		}
		if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
			return &apperror.NotFoundError{Resource: "order"}
		}

		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", id)); err != nil {
			return err
		}

		refetched, err := r.findOrderModelByID(ctx, tx, id)
		if err != nil {
			return err
		}
		result = toOrderDTOFromModel(refetched, nil)
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return result, nil
}

// FindEligibleForArchive returns terminal-status orders that are not yet
// archived and whose terminal timestamp (or updated_at fallback) is older than
// the provided cutoff. Only orders whose current status is terminal are
// considered, and the per-status timestamp in status_timestamps is preferred.
func (r *OrderBunRepository) FindEligibleForArchive(ctx context.Context, terminalStatuses []string, cutoff time.Time, limit int) ([]dto.Order, error) {
	if len(terminalStatuses) == 0 {
		return nil, nil
	}

	q := r.db.NewSelect().Model((*models.Order)(nil)).
		Where("is_archived = ?", false).
		Where("status IN (?)", bun.List(terminalStatuses)).
		Where(`COALESCE(
			(status_timestamps->>'entregada')::timestamptz,
			(status_timestamps->>'cancelada')::timestamptz,
			updated_at
		) < ?`, cutoff).
		Order("created_at ASC").
		Limit(limit)

	var orders []models.Order
	if err := q.Scan(ctx, &orders); err != nil {
		return nil, err
	}
	return toOrderDTOsFromModels(orders), nil
}

// ArchiveOrders marks the provided orders as archived and returns the number
// of rows actually changed.
func (r *OrderBunRepository) ArchiveOrders(ctx context.Context, ids []uuid.UUID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	res, err := r.db.NewUpdate().
		Model((*models.Order)(nil)).
		Set("is_archived = ?, updated_at = ?", true, time.Now()).
		Where("id IN (?)", bun.List(ids)).
		Where("is_archived = ?", false).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	changed, _ := res.RowsAffected()
	return int(changed), nil
}

// Delete deletes an order and its items.
func (r *OrderBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := r.maybeLog(ctx, tx, fmt.Sprintf("order:%s", id)); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*models.OrderItem)(nil)).Where("order_items = ?", id).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewDelete().Model((*models.Order)(nil)).Where("id = ?", id).Exec(ctx)
		return err
	})
}

// FindByDateRange returns orders within a date range.
func (r *OrderBunRepository) FindByDateRange(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	q := r.db.NewSelect().Model((*models.Order)(nil)).Where("created_at >= ? AND created_at <= ?", start, end)
	if len(statuses) > 0 {
		q = q.Where("status IN (?)", bun.List(statuses))
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
		q = q.Where("status IN (?)", bun.List(statuses))
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
		orderQ = orderQ.Where("status NOT IN (?)", bun.List(excludeStatuses))
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

// AnalyticsOverview aggregates order metrics for a date range in PostgreSQL.
func (r *OrderBunRepository) AnalyticsOverview(ctx context.Context, start, end time.Time) (dto.AnalyticsOverview, error) {
	overview := dto.AnalyticsOverview{
		StatusBreakdown: make(map[string]int),
	}

	if err := runInTx(ctx, r.db, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := r.applyStoreTimeZone(ctx, tx); err != nil {
			return err
		}
		var totals struct {
			TotalOrders   int64   `bun:"total_orders"`
			TotalRevenue  float64 `bun:"total_revenue"`
			AvgOrderValue float64 `bun:"avg_order_value"`
		}
		err := tx.NewSelect().
			Model((*models.Order)(nil)).
			ColumnExpr("COUNT(*) AS total_orders").
			ColumnExpr("COALESCE(SUM(total_amount) FILTER (WHERE "+revenueStatusesSQL+"), 0) AS total_revenue").
			ColumnExpr("COALESCE(AVG(total_amount) FILTER (WHERE "+revenueStatusesSQL+"), 0) AS avg_order_value").
			Where("created_at >= ? AND created_at <= ?", start, end).
			Scan(ctx, &totals)
		if err != nil {
			return err
		}
		overview.TotalOrders = int(totals.TotalOrders)
		overview.TotalRevenue = totals.TotalRevenue
		overview.AvgOrderValue = totals.AvgOrderValue

		var statusRows []struct {
			Status string `bun:"status"`
			Count  int64  `bun:"count"`
		}
		err = tx.NewSelect().
			Model((*models.Order)(nil)).
			Column("status").
			ColumnExpr("COUNT(*) AS count").
			Where("created_at >= ? AND created_at <= ?", start, end).
			Group("status").
			Scan(ctx, &statusRows)
		if err != nil {
			return err
		}
		for _, row := range statusRows {
			overview.StatusBreakdown[row.Status] = int(row.Count)
		}

		var paymentRows []struct {
			PaymentMethod string  `bun:"payment_method"`
			Count         int64   `bun:"count"`
			Revenue       float64 `bun:"revenue"`
		}
		err = tx.NewSelect().
			Model((*models.Order)(nil)).
			Column("payment_method").
			ColumnExpr("COUNT(*) AS count").
			ColumnExpr("COALESCE(SUM(total_amount) FILTER (WHERE "+revenueStatusesSQL+"), 0) AS revenue").
			Where("created_at >= ? AND created_at <= ?", start, end).
			Group("payment_method").
			Scan(ctx, &paymentRows)
		if err != nil {
			return err
		}
		overview.PaymentMethodBreakdown = make(map[string]dto.AnalyticsPaymentMethodSummary, len(paymentRows))
		for _, row := range paymentRows {
			overview.PaymentMethodBreakdown[row.PaymentMethod] = dto.AnalyticsPaymentMethodSummary{
				Revenue: row.Revenue,
				Count:   int(row.Count),
			}
		}
		return nil
	}); err != nil {
		return overview, err
	}

	return overview, nil
}

// AnalyticsOrdersOverTime aggregates order counts and revenue by day/week/month.
func (r *OrderBunRepository) AnalyticsOrdersOverTime(ctx context.Context, start, end time.Time, groupBy string) ([]dto.AnalyticsTimelinePoint, error) {
	var result []dto.AnalyticsTimelinePoint

	if err := runInTx(ctx, r.db, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := r.applyStoreTimeZone(ctx, tx); err != nil {
			return err
		}
		var rows []struct {
			Date    string  `bun:"date"`
			Count   int64   `bun:"count"`
			Revenue float64 `bun:"revenue"`
		}

		var dateExpr string
		switch groupBy {
		case "week":
			dateExpr = "to_char(created_at, 'IYYY') || '-W' || to_char(created_at, 'IW')"
		case "month":
			dateExpr = "to_char(created_at, 'YYYY-MM')"
		default:
			dateExpr = "to_char(created_at, 'YYYY-MM-DD')"
		}

		err := tx.NewSelect().
			Model((*models.Order)(nil)).
			ColumnExpr(dateExpr+" AS date").
			ColumnExpr("COUNT(*) AS count").
			ColumnExpr("COALESCE(SUM(total_amount) FILTER (WHERE "+revenueStatusesSQL+"), 0) AS revenue").
			Where("created_at >= ? AND created_at <= ?", start, end).
			GroupExpr(dateExpr).
			OrderExpr(dateExpr).
			Scan(ctx, &rows)
		if err != nil {
			return err
		}

		result = make([]dto.AnalyticsTimelinePoint, len(rows))
		for i, row := range rows {
			result[i] = dto.AnalyticsTimelinePoint{
				Date:    row.Date,
				Count:   int(row.Count),
				Revenue: row.Revenue,
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// AnalyticsTopProducts aggregates top products by product_id for a date range.
func (r *OrderBunRepository) AnalyticsTopProducts(ctx context.Context, start, end time.Time, limit int) ([]dto.AnalyticsTopProduct, error) {
	var result []dto.AnalyticsTopProduct

	if err := runInTx(ctx, r.db, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := r.applyStoreTimeZone(ctx, tx); err != nil {
			return err
		}
		var rows []struct {
			Name       string  `bun:"name"`
			TotalSold  int64   `bun:"total_sold"`
			TotalValue float64 `bun:"total_value"`
		}

		query := `
SELECT COALESCE(MAX(p.name), MAX(oi.product_name)) AS name,
       SUM(oi.quantity)::bigint AS total_sold,
       SUM(oi.subtotal) AS total_value
FROM order_items oi
JOIN orders o ON o.id = oi.order_items
LEFT JOIN products p ON p.id = oi.product_order_items
WHERE o.created_at >= ? AND o.created_at <= ?
  AND o.` + revenueStatusesSQL + `
GROUP BY oi.product_order_items
ORDER BY total_sold DESC, total_value DESC
LIMIT ?
`
		err := tx.NewRaw(query, start, end, limit).Scan(ctx, &rows)
		if err != nil {
			return err
		}

		result = make([]dto.AnalyticsTopProduct, len(rows))
		for i, row := range rows {
			result[i] = dto.AnalyticsTopProduct{
				Name:       row.Name,
				TotalSold:  int(row.TotalSold),
				TotalValue: row.TotalValue,
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// StreamOrderExportRows streams order export rows in batches without loading the full range into memory.
func (r *OrderBunRepository) StreamOrderExportRows(ctx context.Context, start, end time.Time, statuses []string, batchSize int, cb func(dto.ExportOrderRow) error) error {
	if batchSize <= 0 {
		batchSize = 500
	}

	return r.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		baseQuery := `
SELECT
  o.id,
  o.order_number,
  o.customer_name,
  o.customer_phone,
  o.status,
  o.payment_method,
  o.payment_status,
  o.order_type,
  o.table_number,
  o.total_amount,
  COUNT(oi.id) AS items_count,
  COALESCE(STRING_AGG(oi.quantity::text || 'x ' || oi.product_name, '; ' ORDER BY oi.product_name), '') AS items_summary,
  o.notes,
  o.created_at,
  o.updated_at
FROM orders o
LEFT JOIN order_items oi ON oi.order_items = o.id
WHERE o.created_at >= ? AND o.created_at <= ?
`

		for offset := 0; ; offset += batchSize {
			args := []interface{}{start, end}
			query := baseQuery
			if len(statuses) > 0 {
				query += " AND o.status IN (?)"
				args = append(args, bun.List(statuses))
			}
			query += " GROUP BY o.id ORDER BY o.created_at DESC LIMIT ? OFFSET ?"
			args = append(args, batchSize, offset)

			var rows []dto.ExportOrderRow
			if err := tx.NewRaw(query, args...).Scan(ctx, &rows); err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}

			for _, row := range rows {
				if err := cb(row); err != nil {
					return err
				}
			}
		}
	})
}

// Purge deletes orders and items within a date range matching statuses.
func (r *OrderBunRepository) Purge(ctx context.Context, start, end time.Time, statuses []string) (int, int, error) {
	var deletedOrders, deletedItems int

	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		q := tx.NewSelect().Model((*models.Order)(nil)).Column("id").Where("created_at >= ? AND created_at <= ?", start, end)
		if len(statuses) > 0 {
			q = q.Where("status IN (?)", bun.List(statuses))
		}

		var ids []uuid.UUID
		if err := q.Scan(ctx, &ids); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		resItems, err := tx.NewDelete().Model((*models.OrderItem)(nil)).Where("order_items IN (?)", bun.List(ids)).Exec(ctx)
		if err != nil {
			return err
		}
		deletedItems64, _ := resItems.RowsAffected()
		deletedItems = int(deletedItems64)

		resOrders, err := tx.NewDelete().Model((*models.Order)(nil)).Where("id IN (?)", bun.List(ids)).Exec(ctx)
		if err != nil {
			return err
		}
		deletedOrders64, _ := resOrders.RowsAffected()
		deletedOrders = int(deletedOrders64)

		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return deletedOrders, deletedItems, nil
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
	if err := r.db.NewSelect().Model(&items).Where("order_items IN (?)", bun.List(orderIDs)).Scan(ctx); err != nil {
		// Fall back to empty items on error; caller will return the error if needed.
		items = nil
	}

	itemsByOrder := make(map[uuid.UUID][]dto.OrderItem)
	for _, item := range items {
		dtoItem := toOrderItemDTOFromModel(item, nil)
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], dtoItem)
	}

	result := make([]dto.Order, len(orders))
	for i, o := range orders {
		result[i] = toOrderDTOFromModel(o, itemsByOrder[o.ID])
	}
	return result
}

func (r *OrderBunRepository) attachItemsAndProductsToOrders(ctx context.Context, orders []models.Order) []dto.Order {
	if len(orders) == 0 {
		return nil
	}

	orderIDs := make([]uuid.UUID, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}

	var items []models.OrderItem
	if err := r.db.NewSelect().Model(&items).Where("order_items IN (?)", bun.List(orderIDs)).Scan(ctx); err != nil {
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
	if err := r.db.NewSelect().Model(&products).Where("id IN (?)", bun.List(unique)).Scan(ctx); err != nil {
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
