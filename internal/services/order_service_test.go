package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type orderRepoStub struct {
	listFn                func(ctx context.Context, status string, page, limit int) ([]dto.Order, int, error)
	findByID              func(ctx context.Context, id uuid.UUID) (dto.Order, error)
	findByIDWithItems     func(ctx context.Context, id uuid.UUID) (dto.Order, error)
	findByOrderNumber     func(ctx context.Context, orderNumber string) (dto.Order, error)
	findByOrderNumberAndPhone func(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error)
	findRecentByPhone     func(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error)
	createFn              func(ctx context.Context, input repository.CreateOrderInput) (dto.Order, error)
	approveFn             func(ctx context.Context, id uuid.UUID, reviewerID *string) (dto.Order, error)
	rejectFn              func(ctx context.Context, id uuid.UUID, reason string, reviewerID *string) (dto.Order, error)
	updateStatusFn        func(ctx context.Context, id uuid.UUID, status string) (dto.Order, error)
	updateNotesFn         func(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error)
	deleteFn              func(ctx context.Context, id uuid.UUID) error
}

func (s *orderRepoStub) ListPaginated(ctx context.Context, status string, page, limit int) ([]dto.Order, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, status, page, limit)
	}
	return nil, 0, nil
}

func (s *orderRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	if s.findByID != nil {
		return s.findByID(ctx, id)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) FindByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	if s.findByIDWithItems != nil {
		return s.findByIDWithItems(ctx, id)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) FindByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error) {
	if s.findByOrderNumber != nil {
		return s.findByOrderNumber(ctx, orderNumber)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) FindByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error) {
	if s.findByOrderNumberAndPhone != nil {
		return s.findByOrderNumberAndPhone(ctx, orderNumber, customerPhone)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) FindRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error) {
	if s.findRecentByPhone != nil {
		return s.findRecentByPhone(ctx, customerPhone, limit)
	}
	return nil, nil
}

func (s *orderRepoStub) Create(ctx context.Context, input repository.CreateOrderInput) (dto.Order, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) Approve(ctx context.Context, id uuid.UUID, reviewerID *string) (dto.Order, error) {
	if s.approveFn != nil {
		return s.approveFn(ctx, id, reviewerID)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string) (dto.Order, error) {
	if s.rejectFn != nil {
		return s.rejectFn(ctx, id, reason, reviewerID)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (dto.Order, error) {
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, id, status)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	if s.updateNotesFn != nil {
		return s.updateNotesFn(ctx, id, notes)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *orderRepoStub) FindByDateRange(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	return nil, nil
}

func (s *orderRepoStub) FindByDateRangeWithItems(ctx context.Context, start, end time.Time, statuses []string) ([]dto.Order, error) {
	return nil, nil
}

func (s *orderRepoStub) FindOrderItemsByOrderDateRange(ctx context.Context, start, end time.Time, excludeStatuses []string) ([]dto.OrderItem, error) {
	return nil, nil
}

func (s *orderRepoStub) Purge(ctx context.Context, start, end time.Time, statuses []string) (int, int, error) {
	return 0, 0, nil
}

type orderStoreStub struct {
	createFn func(ctx context.Context, input repository.StoreOrderInput) (dto.Order, error)
	updateFn func(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput) (dto.Order, error)
}

func (s *orderStoreStub) CreateOrder(ctx context.Context, input repository.StoreOrderInput) (dto.Order, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Order{}, nil
}

func (s *orderStoreStub) UpdateOrder(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput) (dto.Order, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Order{}, nil
}

func TestOrderQueryService_GetByOrderNumber_NotFound(t *testing.T) {
	repo := &orderRepoStub{findByOrderNumber: func(_ context.Context, _ string) (dto.Order, error) {
		return dto.Order{}, &apperror.NotFoundError{Resource: "order"}
	}}
	svc := NewOrderQueryService(repo)

	_, err := svc.GetByOrderNumber(context.Background(), "ORD-000000-0000")
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOrderQueryService_ListPaginated(t *testing.T) {
	repo := &orderRepoStub{listFn: func(_ context.Context, _ string, _, _ int) ([]dto.Order, int, error) {
		return []dto.Order{{OrderNumber: "ORD-1"}}, 1, nil
	}}
	svc := NewOrderQueryService(repo)

	orders, total, err := svc.ListPaginated(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(orders) != 1 || orders[0].OrderNumber != "ORD-1" {
		t.Fatalf("unexpected orders: %+v", orders)
	}
}

func TestOrderCommandService_Approve_InvalidState(t *testing.T) {
	repo := &orderRepoStub{findByID: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
		return dto.Order{Status: "cancelada"}, nil
	}}
	store := &orderStoreStub{}
	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, nil, nil, nil, nil)

	_, err := svc.Approve(context.Background(), uuid.New(), "staff-1", false)
	if !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("expected ErrInvalidOrderState, got %v", err)
	}
}

func TestOrderCommandService_Reject_InvalidState(t *testing.T) {
	repo := &orderRepoStub{findByID: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
		return dto.Order{Status: "entregada"}, nil
	}}
	store := &orderStoreStub{}
	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, nil, nil, nil, nil)

	_, err := svc.Reject(context.Background(), uuid.New(), "out of stock", "staff-1")
	if !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("expected ErrInvalidOrderState, got %v", err)
	}
}

func TestOrderCommandService_UpdateStatus_InvalidStatus(t *testing.T) {
	repo := &orderRepoStub{findByID: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
		return dto.Order{Status: "pendiente_revision"}, nil
	}}
	store := &orderStoreStub{}
	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, nil, nil, nil, nil)

	_, err := svc.UpdateStatus(context.Background(), uuid.New(), "not_a_status")
	if !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("expected ErrInvalidOrderState, got %v", err)
	}
}

func TestOrderCommandService_Update_DelegatesToStore(t *testing.T) {
	called := false
	store := &orderStoreStub{updateFn: func(_ context.Context, _ uuid.UUID, _ repository.StoreUpdateOrderInput) (dto.Order, error) {
		called = true
		return dto.Order{OrderNumber: "ORD-UPDATED"}, nil
	}}
	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, nil, nil, nil, nil)

	order, err := svc.Update(context.Background(), uuid.New(), UpdateOrderInput{
		CustomerName:  "Jane",
		CustomerPhone: "5550000",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected store.UpdateOrder to be called")
	}
	if order.OrderNumber != "ORD-UPDATED" {
		t.Fatalf("unexpected order: %+v", order)
	}
}

func TestOrderCommandService_Delete(t *testing.T) {
	repo := &orderRepoStub{
		findByID: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
			return dto.Order{OrderNumber: "ORD-DELETE"}, nil
		},
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc := NewOrderCommandService(repo, &orderStoreStub{}, &customerRepoStub{}, nil, nil, nil, nil)

	if err := svc.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrderCommandService_Create_LinksExistingCustomer(t *testing.T) {
	customerID := uuid.New()
	customerRepo := &customerRepoStub{
		findByPhone: func(_ context.Context, phone string) (dto.Customer, error) {
			if phone != "5550000" {
				t.Fatalf("expected phone %q, got %q", "5550000", phone)
			}
			return dto.Customer{ID: customerID, Phone: phone}, nil
		},
	}

	var capturedCustomerID *uuid.UUID
	store := &orderStoreStub{createFn: func(_ context.Context, input repository.StoreOrderInput) (dto.Order, error) {
		capturedCustomerID = input.CustomerID
		return dto.Order{OrderNumber: "ORD-001"}, nil
	}}

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, nil, nil, nil, nil)
	_, err := svc.Create(context.Background(), CreateOrderInput{
		CustomerName:  "Jane",
		CustomerPhone: "5550000",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCustomerID == nil || *capturedCustomerID != customerID {
		t.Fatalf("expected customer ID %q, got %v", customerID, capturedCustomerID)
	}
}

func TestOrderCommandService_Create_CreatesCustomerWhenMissing(t *testing.T) {
	customerRepo := &customerRepoStub{
		findByPhone: func(_ context.Context, _ string) (dto.Customer, error) {
			return dto.Customer{}, &apperror.NotFoundError{Resource: "customer"}
		},
		createFn: func(_ context.Context, input repository.CreateCustomerInput) (dto.Customer, error) {
			return dto.Customer{ID: uuid.New(), Phone: input.Phone, Name: input.Name}, nil
		},
	}

	var capturedCustomerID *uuid.UUID
	store := &orderStoreStub{createFn: func(_ context.Context, input repository.StoreOrderInput) (dto.Order, error) {
		capturedCustomerID = input.CustomerID
		return dto.Order{OrderNumber: "ORD-002"}, nil
	}}

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, nil, nil, nil, nil)
	_, err := svc.Create(context.Background(), CreateOrderInput{
		CustomerName:  "John",
		CustomerPhone: "5551111",
		CustomerEmail: strPtr("john@example.com"),
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCustomerID == nil {
		t.Fatal("expected a customer ID to be linked")
	}
}

func TestOrderCommandService_Update_ReLinksWhenPhoneChanges(t *testing.T) {
	orderID := uuid.New()
	newCustomerID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			if id != orderID {
				t.Fatalf("expected order ID %q, got %q", orderID, id)
			}
			return dto.Order{ID: orderID, CustomerPhone: "5550000"}, nil
		},
	}
	customerRepo := &customerRepoStub{
		findByPhone: func(_ context.Context, phone string) (dto.Customer, error) {
			if phone != "5552222" {
				t.Fatalf("expected phone %q, got %q", "5552222", phone)
			}
			return dto.Customer{ID: newCustomerID, Phone: phone}, nil
		},
	}

	var capturedCustomerID *uuid.UUID
	store := &orderStoreStub{updateFn: func(_ context.Context, _ uuid.UUID, input repository.StoreUpdateOrderInput) (dto.Order, error) {
		capturedCustomerID = input.CustomerID
		return dto.Order{OrderNumber: "ORD-UPDATED"}, nil
	}}

	svc := NewOrderCommandService(repo, store, customerRepo, nil, nil, nil, nil)
	_, err := svc.Update(context.Background(), orderID, UpdateOrderInput{
		CustomerName:  "Jane",
		CustomerPhone: "5552222",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCustomerID == nil || *capturedCustomerID != newCustomerID {
		t.Fatalf("expected customer ID %q, got %v", newCustomerID, capturedCustomerID)
	}
}

func strPtr(s string) *string {
	return &s
}
