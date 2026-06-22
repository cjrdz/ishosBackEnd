// order_repo.go — Ent-backed repository for order and order-item persistence.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
	"github.com/cjrdz/ishosBackEnd/ent/predicate"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type OrderRepository struct {
	db *ent.Client
}

func NewOrderRepository(db *ent.Client) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, input CreateOrderInput) (dto.Order, error) {
	now := time.Now()

	create := r.db.Order.Create().
		SetOrderNumber(input.OrderNumber).
		SetCustomerName(input.CustomerName).
		SetCustomerPhone(input.CustomerPhone).
		SetNillableCustomerEmail(input.CustomerEmail).
		SetNillableCustomerIP(input.CustomerIP).
		SetTotalAmount(input.TotalAmount).
		SetStatus(input.Status).
		SetPaymentMethod(input.PaymentMethod).
		SetOrderType(input.OrderType).
		SetNillableTableNumber(input.TableNumber).
		SetNillableNotes(input.Notes).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	if input.CreatedByUserID != nil {
		create = create.SetNillableCreatedByUserID(input.CreatedByUserID)
	}

	order, err := create.Save(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) CreateItem(ctx context.Context, input CreateOrderItemInput) (dto.OrderItem, error) {
	create := r.db.OrderItem.Create().
		SetOrderID(input.OrderID).
		SetProductID(input.ProductID).
		SetSnapshotProductID(input.ProductID).
		SetProductName(input.ProductName).
		SetQuantity(input.Quantity).
		SetUnitPrice(input.UnitPrice).
		SetSubtotal(input.Subtotal)

	if input.Customizations != nil {
		create = create.SetCustomizations(input.Customizations)
	}

	item, err := create.Save(ctx)
	if err != nil {
		return dto.OrderItem{}, err
	}

	return toDTOOrderItem(item), nil
}

func (r *OrderRepository) ListPaginated(ctx context.Context, status string, page, limit int) ([]dto.Order, int, error) {
	// List view only needs order items; product details are denormalized into
	// snapshot_product_id and product_name, so we avoid an extra product query.
	q := r.db.Order.Query().WithItems()
	if status != "" {
		q = q.Where(entOrder.StatusEQ(entOrder.Status(status)))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	orders, err := q.
		Order(ent.Desc(entOrder.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return toDTOOrders(orders), total, nil
}

func (r *OrderRepository) FindByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	order, err := r.db.Order.Query().
		Where(entOrder.IDEQ(id)).
		WithItems(func(query *ent.OrderItemQuery) {
			query.WithProduct()
		}).
		Only(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	order, err := r.db.Order.Get(ctx, id)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) FindByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error) {
	order, err := r.db.Order.Query().
		Where(entOrder.OrderNumberEQ(orderNumber)).
		WithItems(func(query *ent.OrderItemQuery) {
			query.WithProduct()
		}).
		Only(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) FindByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error) {
	order, err := r.db.Order.Query().
		Where(
			entOrder.OrderNumberEQ(orderNumber),
			entOrder.CustomerPhoneEQ(customerPhone),
		).
		Only(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) FindRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error) {
	query := r.db.Order.Query().
		Where(entOrder.CustomerPhoneEQ(customerPhone)).
		Order(ent.Desc(entOrder.FieldCreatedAt))

	if limit > 0 {
		query = query.Limit(limit)
	}

	orders, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOOrders(orders), nil
}

func (r *OrderRepository) Approve(ctx context.Context, id uuid.UUID, reviewerID *string) (dto.Order, error) {
	now := time.Now()
	order, err := r.db.Order.UpdateOneID(id).
		SetStatus(entOrder.StatusRecibida).
		SetPaymentStatus(entOrder.PaymentStatusConfirmado).
		ClearRejectionReason().
		SetNillableReviewedByUserID(reviewerID).
		SetNillableReviewedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string) (dto.Order, error) {
	now := time.Now()
	order, err := r.db.Order.UpdateOneID(id).
		SetStatus(entOrder.StatusCancelada).
		SetRejectionReason(reason).
		SetNillableReviewedByUserID(reviewerID).
		SetNillableReviewedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (dto.Order, error) {
	order, err := r.db.Order.UpdateOneID(id).
		SetStatus(entOrder.Status(status)).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	update := r.db.Order.UpdateOneID(id).
		SetUpdatedAt(time.Now())

	if notes == nil {
		update = update.ClearNotes()
	} else {
		update = update.SetNotes(*notes)
	}

	order, err := update.Save(ctx)
	if err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	return toDTOOrder(order), nil
}

func (r *OrderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Delete items first; ent does not cascade deletes from order to order items automatically.
	if _, err := r.db.OrderItem.Delete().Where(entOrderItem.HasOrderWith(entOrder.IDEQ(id))).Exec(ctx); err != nil {
		return err
	}

	return r.db.Order.DeleteOneID(id).Exec(ctx)
}

// FindByDateRange returns orders created within a date range, optionally filtered by status.
func (r *OrderRepository) FindByDateRange(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	q := r.db.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(end),
		)

	if len(statuses) > 0 {
		q = q.Where(entOrder.StatusIn(toEntStatuses(statuses)...))
	}

	orders, err := q.All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOOrders(orders), nil
}

// FindByDateRangeWithItems returns orders with their items created within a date range, optionally filtered by status.
func (r *OrderRepository) FindByDateRangeWithItems(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	q := r.db.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(end),
		).
		WithItems(func(query *ent.OrderItemQuery) {
			query.WithProduct()
		})

	if len(statuses) > 0 {
		q = q.Where(entOrder.StatusIn(toEntStatuses(statuses)...))
	}

	orders, err := q.All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOOrders(orders), nil
}

// FindOrderItemsByOrderDateRange returns order items whose parent order falls within the date range
// and whose status is not in excludeStatuses.
func (r *OrderRepository) FindOrderItemsByOrderDateRange(ctx context.Context, start, end time.Time, excludeStatuses []string) ([]dto.OrderItem, error) {
	predicates := []predicate.Order{
		entOrder.CreatedAtGTE(start),
		entOrder.CreatedAtLTE(end),
	}
	for _, status := range excludeStatuses {
		predicates = append(predicates, entOrder.StatusNEQ(entOrder.Status(status)))
	}

	items, err := r.db.OrderItem.Query().
		Where(entOrderItem.HasOrderWith(predicates...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDTOOrderItems(items), nil
}

// Purge deletes orders (and their items) created within a date range and matching the given statuses.
// It returns the number of deleted orders and items.
func (r *OrderRepository) Purge(ctx context.Context, start, end time.Time, statuses []string) (int, int, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return 0, 0, err
	}

	q := tx.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(end),
		)
	if len(statuses) > 0 {
		q = q.Where(entOrder.StatusIn(toEntStatuses(statuses)...))
	}

	ids, err := q.IDs(ctx)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}

	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}

	deletedItems, err := tx.OrderItem.Delete().Where(entOrderItem.HasOrderWith(entOrder.IDIn(ids...))).Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}

	deletedOrders, err := tx.Order.Delete().Where(entOrder.IDIn(ids...)).Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	return deletedOrders, deletedItems, nil
}

// toEntStatuses converts string statuses to Ent order statuses.
func toEntStatuses(statuses []string) []entOrder.Status {
	result := make([]entOrder.Status, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, entOrder.Status(status))
	}
	return result
}

// toDTOOrderItems maps a slice of ent.OrderItem values to dto.OrderItem values.
func toDTOOrderItems(items []*ent.OrderItem) []dto.OrderItem {
	result := make([]dto.OrderItem, 0, len(items))
	for _, item := range items {
		result = append(result, toDTOOrderItem(item))
	}
	return result
}

type CreateOrderInput struct {
	OrderNumber     string
	CustomerName    string
	CustomerPhone   string
	CustomerEmail   *string
	CustomerIP      *string
	TotalAmount     float64
	Status          entOrder.Status
	PaymentMethod   entOrder.PaymentMethod
	OrderType       entOrder.OrderType
	TableNumber     *int
	Notes           *string
	CreatedByUserID *string
}

type CreateOrderItemInput struct {
	OrderID        uuid.UUID
	ProductID      uuid.UUID
	ProductName    string
	Quantity       int
	UnitPrice      float64
	Subtotal       float64
	Customizations map[string]interface{}
}

// translateOrderError converts Ent not-found errors into apperror.NotFoundError.
func translateOrderError(err error) error {
	if ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "order", Cause: err}
	}
	return err
}

// toDTOOrder maps an ent.Order to a dto.Order, loading items when present.
func toDTOOrder(order *ent.Order) dto.Order {
	if order == nil {
		return dto.Order{}
	}

	items := make([]dto.OrderItem, 0)
	if order.Edges.Items != nil {
		items = make([]dto.OrderItem, 0, len(order.Edges.Items))
		for _, item := range order.Edges.Items {
			items = append(items, toDTOOrderItem(item))
		}
	}

	return dto.Order{
		ID:               order.ID,
		OrderNumber:      order.OrderNumber,
		CustomerName:     order.CustomerName,
		CustomerPhone:    order.CustomerPhone,
		CustomerEmail:    order.CustomerEmail,
		CustomerIP:       order.CustomerIP,
		TotalAmount:      order.TotalAmount,
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
		Items:            items,
	}
}

// toDTOOrders maps a slice of ent.Order values to dto.Order values.
func toDTOOrders(orders []*ent.Order) []dto.Order {
	result := make([]dto.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, toDTOOrder(order))
	}
	return result
}

// toDTOOrderItem maps an ent.OrderItem to a dto.OrderItem, loading order/product edges when present.
func toDTOOrderItem(item *ent.OrderItem) dto.OrderItem {
	if item == nil {
		return dto.OrderItem{}
	}

	dtoItem := dto.OrderItem{
		ID:             item.ID,
		ProductName:    item.ProductName,
		Quantity:       item.Quantity,
		UnitPrice:      item.UnitPrice,
		Subtotal:       item.Subtotal,
		Customizations: item.Customizations,
	}

	if item.Edges.Order != nil {
		dtoItem.OrderID = item.Edges.Order.ID
	}
	// Prefer the denormalized snapshot_product_id for the ID; always populate
	// the Product DTO when the edge is available so downstream code (stock
	// deduction, enrichment) can read BallBased / BallQuantity.
	if item.SnapshotProductID != nil && *item.SnapshotProductID != uuid.Nil {
		dtoItem.ProductID = *item.SnapshotProductID
	} else if item.Edges.Product != nil {
		dtoItem.ProductID = item.Edges.Product.ID
	}
	if item.Edges.Product != nil {
		p := toProductDTO(item.Edges.Product)
		dtoItem.Product = &p
	}

	return dtoItem
}
