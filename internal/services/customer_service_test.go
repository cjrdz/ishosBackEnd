package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type customerRepoStub struct {
	listCalls      int
	listFn         func(ctx context.Context, status, search string) ([]dto.Customer, error)
	listPaginated  func(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error)
	findByID       func(ctx context.Context, id uuid.UUID) (dto.Customer, error)
	findByPhone    func(ctx context.Context, phone string) (dto.Customer, error)
	createFn       func(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error)
	updateFn       func(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (dto.Customer, error)
	deleteFn       func(ctx context.Context, id uuid.UUID) error
	listOrdersByID func(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error)
}

func (s *customerRepoStub) List(ctx context.Context, status, search string) ([]dto.Customer, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, status, search)
	}
	return nil, nil
}

func (s *customerRepoStub) ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error) {
	if s.listPaginated != nil {
		return s.listPaginated(ctx, status, search, offset, limit)
	}
	return nil, 0, nil
}

func (s *customerRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.Customer, error) {
	if s.findByID != nil {
		return s.findByID(ctx, id)
	}
	return dto.Customer{}, nil
}

func (s *customerRepoStub) FindByPhone(ctx context.Context, phone string) (dto.Customer, error) {
	if s.findByPhone != nil {
		return s.findByPhone(ctx, phone)
	}
	return dto.Customer{}, nil
}

func (s *customerRepoStub) Create(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Customer{}, nil
}

func (s *customerRepoStub) Update(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (dto.Customer, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Customer{}, nil
}

func (s *customerRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *customerRepoStub) ListOrdersByCustomerID(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error) {
	if s.listOrdersByID != nil {
		return s.listOrdersByID(ctx, customerID, limit)
	}
	return nil, nil
}

func TestCustomerServiceCreate_TrimsInput(t *testing.T) {
	repo := &customerRepoStub{createFn: func(_ context.Context, input repository.CreateCustomerInput) (dto.Customer, error) {
		if input.Name != "Alice" {
			t.Fatalf("expected name trimmed to %q, got %q", "Alice", input.Name)
		}
		if input.Phone != "555-1234" {
			t.Fatalf("expected phone trimmed to %q, got %q", "555-1234", input.Phone)
		}
		return dto.Customer{Name: input.Name, Phone: input.Phone}, nil
	}}
	svc := NewCustomerService(repo)

	_, err := svc.Create(context.Background(), repository.CreateCustomerInput{
		Name:  "  Alice  ",
		Phone: "  555-1234  ",
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
}

func TestCustomerServiceGetByID_NotFound(t *testing.T) {
	repo := &customerRepoStub{findByID: func(_ context.Context, _ uuid.UUID) (dto.Customer, error) {
		return dto.Customer{}, &apperror.NotFoundError{Resource: "customer"}
	}}
	svc := NewCustomerService(repo)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCustomerServiceOrdersByCustomer(t *testing.T) {
	customerID := uuid.New()
	repo := &customerRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Customer, error) {
			if id != customerID {
				t.Fatalf("expected customer ID %q, got %q", customerID, id)
			}
			return dto.Customer{ID: customerID, Phone: "555-1234"}, nil
		},
		listOrdersByID: func(_ context.Context, id uuid.UUID, limit int) ([]dto.Order, error) {
			if id != customerID {
				t.Fatalf("expected customer ID %q, got %q", customerID, id)
			}
			return []dto.Order{{OrderNumber: "ORD-001"}}, nil
		},
	}
	svc := NewCustomerService(repo)

	orders, err := svc.OrdersByCustomer(context.Background(), customerID, 10)
	if err != nil {
		t.Fatalf("OrdersByCustomer error = %v", err)
	}
	if len(orders) != 1 || orders[0].OrderNumber != "ORD-001" {
		t.Fatalf("unexpected orders: %+v", orders)
	}
}

func TestCustomerServiceUpdate_TrimsInput(t *testing.T) {
	repo := &customerRepoStub{updateFn: func(_ context.Context, _ uuid.UUID, input repository.UpdateCustomerInput) (dto.Customer, error) {
		if *input.Name != "Bob" {
			t.Fatalf("expected name trimmed to %q, got %q", "Bob", *input.Name)
		}
		return dto.Customer{Name: *input.Name}, nil
	}}
	svc := NewCustomerService(repo)

	name := "  Bob  "
	_, err := svc.Update(context.Background(), uuid.New(), repository.UpdateCustomerInput{Name: &name})
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}
}
