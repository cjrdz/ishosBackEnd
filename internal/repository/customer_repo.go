// customer_repo.go — Ent-backed repository for customer CRUD and order lookup by phone.
package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entCustomer "github.com/cjrdz/ishosBackEnd/ent/customer"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type CustomerRepository struct {
	db *ent.Client
}

func NewCustomerRepository(db *ent.Client) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) List(ctx context.Context, status, search string) ([]dto.Customer, error) {
	q := r.db.Customer.Query()

	if status != "" {
		q = q.Where(entCustomer.StatusEQ(entCustomer.Status(status)))
	}

	if search != "" {
		term := strings.TrimSpace(search)
		if term != "" {
			q = q.Where(
				entCustomer.Or(
					entCustomer.NameContainsFold(term),
					entCustomer.PhoneContainsFold(term),
					entCustomer.EmailContainsFold(term),
				),
			)
		}
	}

	rows, err := q.Order(ent.Desc(entCustomer.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}
	return toCustomerDTOs(rows), nil
}

func (r *CustomerRepository) ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error) {
	q := r.db.Customer.Query()

	if status != "" {
		q = q.Where(entCustomer.StatusEQ(entCustomer.Status(status)))
	}

	if search != "" {
		term := strings.TrimSpace(search)
		if term != "" {
			q = q.Where(
				entCustomer.Or(
					entCustomer.NameContainsFold(term),
					entCustomer.PhoneContainsFold(term),
					entCustomer.EmailContainsFold(term),
				),
			)
		}
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := q.Order(ent.Desc(entCustomer.FieldCreatedAt)).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return toCustomerDTOs(rows), total, nil
}

func (r *CustomerRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Customer, error) {
	row, err := r.db.Customer.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Customer{}, &apperror.NotFoundError{Resource: "customer", Cause: err}
		}
		return dto.Customer{}, err
	}
	return toCustomerDTO(row), nil
}

func (r *CustomerRepository) FindByPhone(ctx context.Context, phone string) (dto.Customer, error) {
	row, err := r.db.Customer.Query().
		Where(entCustomer.PhoneEQ(phone)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Customer{}, &apperror.NotFoundError{Resource: "customer", Cause: err}
		}
		return dto.Customer{}, err
	}
	return toCustomerDTO(row), nil
}

func (r *CustomerRepository) Create(ctx context.Context, input CreateCustomerInput) (dto.Customer, error) {
	create := r.db.Customer.Create().
		SetName(input.Name).
		SetUserType(entCustomer.UserType(input.UserType)).
		SetPhone(input.Phone).
		SetStatus(entCustomer.Status(input.Status))

	if input.Email != nil {
		create = create.SetEmail(*input.Email)
	}

	row, err := create.Save(ctx)
	if err != nil {
		return dto.Customer{}, err
	}
	return toCustomerDTO(row), nil
}

func (r *CustomerRepository) Update(ctx context.Context, id uuid.UUID, input UpdateCustomerInput) (dto.Customer, error) {
	update := r.db.Customer.UpdateOneID(id)

	if input.Name != nil {
		update = update.SetName(*input.Name)
	}
	if input.UserType != nil {
		update = update.SetUserType(entCustomer.UserType(*input.UserType))
	}
	if input.Phone != nil {
		update = update.SetPhone(*input.Phone)
	}
	if input.Email != nil {
		if strings.TrimSpace(*input.Email) == "" {
			update = update.ClearEmail()
		} else {
			update = update.SetEmail(*input.Email)
		}
	}
	if input.Status != nil {
		update = update.SetStatus(entCustomer.Status(*input.Status))
	}

	row, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Customer{}, &apperror.NotFoundError{Resource: "customer", Cause: err}
		}
		return dto.Customer{}, err
	}
	return toCustomerDTO(row), nil
}

func (r *CustomerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.Customer.DeleteOneID(id).Exec(ctx)
	if err != nil && ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "customer", Cause: err}
	}
	return err
}

func (r *CustomerRepository) ListOrdersByCustomerID(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error) {
	q := r.db.Order.Query().
		Where(entOrder.CustomerIDEQ(customerID)).
		WithItems(func(oiq *ent.OrderItemQuery) {
			oiq.WithProduct()
		}).
		Order(ent.Desc(entOrder.FieldCreatedAt))
	if limit > 0 {
		q = q.Limit(limit)
	}

	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}

	return toOrderDTOs(rows), nil
}

func toCustomerDTOs(rows []*ent.Customer) []dto.Customer {
	result := make([]dto.Customer, len(rows))
	for i, row := range rows {
		result[i] = toCustomerDTO(row)
	}
	return result
}

func toCustomerDTO(c *ent.Customer) dto.Customer {
	if c == nil {
		return dto.Customer{}
	}
	return dto.Customer{
		ID:        c.ID,
		Name:      c.Name,
		UserType:  string(c.UserType),
		Phone:     c.Phone,
		Email:     c.Email,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func toOrderDTOs(rows []*ent.Order) []dto.Order {
	result := make([]dto.Order, len(rows))
	for i, row := range rows {
		result[i] = toOrderDTO(row)
	}
	return result
}

func toOrderDTO(o *ent.Order) dto.Order {
	if o == nil {
		return dto.Order{}
	}
	order := dto.Order{
		ID:            o.ID,
		OrderNumber:   o.OrderNumber,
		CustomerName:  o.CustomerName,
		CustomerPhone: o.CustomerPhone,
		CustomerEmail: o.CustomerEmail,
		CustomerIP:    o.CustomerIP,
		TotalAmount:   o.TotalAmount,
		Status:        string(o.Status),
		PaymentMethod: string(o.PaymentMethod),
		PaymentStatus: string(o.PaymentStatus),
		OrderType:     string(o.OrderType),
		TableNumber:   o.TableNumber,
		Notes:         o.Notes,
		RejectionReason: o.RejectionReason,
		ReviewedByUserID: o.ReviewedByUserID,
		ReviewedAt:    o.ReviewedAt,
		CreatedByUserID: o.CreatedByUserID,
		IsArchived:    o.IsArchived,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}
	if o.Edges.Items != nil {
		order.Items = make([]dto.OrderItem, len(o.Edges.Items))
		for i, item := range o.Edges.Items {
			order.Items[i] = toOrderItemDTO(item)
		}
	}
	return order
}

func toOrderItemDTO(item *ent.OrderItem) dto.OrderItem {
	if item == nil {
		return dto.OrderItem{}
	}
	var orderID, productID uuid.UUID
	if item.Edges.Order != nil {
		orderID = item.Edges.Order.ID
	}
	if item.Edges.Product != nil {
		productID = item.Edges.Product.ID
	}
	return dto.OrderItem{
		ID:             item.ID,
		OrderID:        orderID,
		ProductID:      productID,
		ProductName:    item.ProductName,
		Quantity:       item.Quantity,
		UnitPrice:      item.UnitPrice,
		Subtotal:       item.Subtotal,
		Customizations: item.Customizations,
	}
}

type CreateCustomerInput struct {
	Name     string
	UserType string
	Phone    string
	Email    *string
	Status   string
}

type UpdateCustomerInput struct {
	Name     *string
	UserType *string
	Phone    *string
	Email    *string
	Status   *string
}
