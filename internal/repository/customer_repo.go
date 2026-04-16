package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entCustomer "github.com/cjrdz/ishosBackEnd/ent/customer"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
)

type CustomerRepository struct {
	db *ent.Client
}

func NewCustomerRepository(db *ent.Client) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) List(ctx context.Context, status, search string) ([]*ent.Customer, error) {
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

	return q.Order(ent.Desc(entCustomer.FieldCreatedAt)).All(ctx)
}

func (r *CustomerRepository) ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]*ent.Customer, int, error) {
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

	customers, err := q.Order(ent.Desc(entCustomer.FieldCreatedAt)).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

func (r *CustomerRepository) FindByID(ctx context.Context, id uuid.UUID) (*ent.Customer, error) {
	return r.db.Customer.Get(ctx, id)
}

func (r *CustomerRepository) Create(ctx context.Context, input CreateCustomerInput) (*ent.Customer, error) {
	create := r.db.Customer.Create().
		SetName(input.Name).
		SetUserType(entCustomer.UserType(input.UserType)).
		SetPhone(input.Phone).
		SetStatus(entCustomer.Status(input.Status))

	if input.Email != nil {
		create = create.SetEmail(*input.Email)
	}

	return create.Save(ctx)
}

func (r *CustomerRepository) Update(ctx context.Context, id uuid.UUID, input UpdateCustomerInput) (*ent.Customer, error) {
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

	return update.Save(ctx)
}

func (r *CustomerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.Customer.DeleteOneID(id).Exec(ctx)
}

func (r *CustomerRepository) ListOrdersByPhone(ctx context.Context, phone string, limit int) ([]*ent.Order, error) {
	q := r.db.Order.Query().
		Where(entOrder.CustomerPhoneEQ(phone)).
		WithItems().
		Order(ent.Desc(entOrder.FieldCreatedAt))
	if limit > 0 {
		q = q.Limit(limit)
	}

	return q.All(ctx)
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
