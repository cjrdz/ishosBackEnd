// order_repo.go — Ent-backed repository for order and order-item persistence.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
)

type OrderRepository struct {
	db *ent.Client
}

func NewOrderRepository(db *ent.Client) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, input CreateOrderInput) (*ent.Order, error) {
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

	return create.Save(ctx)
}

func (r *OrderRepository) CreateItem(ctx context.Context, input CreateOrderItemInput) (*ent.OrderItem, error) {
	create := r.db.OrderItem.Create().
		SetOrderID(input.OrderID).
		SetProductID(input.ProductID).
		SetProductName(input.ProductName).
		SetQuantity(input.Quantity).
		SetUnitPrice(input.UnitPrice).
		SetSubtotal(input.Subtotal)

	if input.Customizations != nil {
		create = create.SetCustomizations(input.Customizations)
	}

	return create.Save(ctx)
}

func (r *OrderRepository) ListPaginated(ctx context.Context, status string, page, limit int) ([]*ent.Order, int, error) {
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

	return orders, total, nil
}

func (r *OrderRepository) FindByIDWithItems(ctx context.Context, id uuid.UUID) (*ent.Order, error) {
	return r.db.Order.Query().
		Where(entOrder.IDEQ(id)).
		WithItems().
		Only(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, id uuid.UUID) (*ent.Order, error) {
	return r.db.Order.Get(ctx, id)
}

func (r *OrderRepository) FindByOrderNumber(ctx context.Context, orderNumber string) (*ent.Order, error) {
	return r.db.Order.Query().
		Where(entOrder.OrderNumberEQ(orderNumber)).
		WithItems().
		Only(ctx)
}

func (r *OrderRepository) FindByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (*ent.Order, error) {
	return r.db.Order.Query().
		Where(
			entOrder.OrderNumberEQ(orderNumber),
			entOrder.CustomerPhoneEQ(customerPhone),
		).
		Only(ctx)
}

func (r *OrderRepository) FindRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]*ent.Order, error) {
	query := r.db.Order.Query().
		Where(entOrder.CustomerPhoneEQ(customerPhone)).
		Order(ent.Desc(entOrder.FieldCreatedAt))

	if limit > 0 {
		query = query.Limit(limit)
	}

	return query.All(ctx)
}

func (r *OrderRepository) Approve(ctx context.Context, id uuid.UUID, reviewerID *string) (*ent.Order, error) {
	now := time.Now()
	return r.db.Order.UpdateOneID(id).
		SetStatus(entOrder.StatusRecibida).
		SetPaymentStatus(entOrder.PaymentStatusConfirmado).
		ClearRejectionReason().
		SetNillableReviewedByUserID(reviewerID).
		SetNillableReviewedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
}

func (r *OrderRepository) Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string) (*ent.Order, error) {
	now := time.Now()
	return r.db.Order.UpdateOneID(id).
		SetStatus(entOrder.StatusCancelada).
		SetRejectionReason(reason).
		SetNillableReviewedByUserID(reviewerID).
		SetNillableReviewedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*ent.Order, error) {
	return r.db.Order.UpdateOneID(id).
		SetStatus(entOrder.Status(status)).
		SetUpdatedAt(time.Now()).
		Save(ctx)
}

func (r *OrderRepository) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (*ent.Order, error) {
	update := r.db.Order.UpdateOneID(id).
		SetUpdatedAt(time.Now())

	if notes == nil {
		update = update.ClearNotes()
	} else {
		update = update.SetNotes(*notes)
	}

	return update.Save(ctx)
}

func (r *OrderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Delete items first; ent does not cascade deletes from order to order items automatically.
	if _, err := r.db.OrderItem.Delete().Where(entOrderItem.HasOrderWith(entOrder.IDEQ(id))).Exec(ctx); err != nil {
		return err
	}

	return r.db.Order.DeleteOneID(id).Exec(ctx)
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
