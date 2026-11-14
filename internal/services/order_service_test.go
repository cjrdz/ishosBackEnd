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
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

type orderRepoStub struct {
	listFn                       func(ctx context.Context, status string, archived bool, page, limit int) ([]dto.Order, int, error)
	findByID                     func(ctx context.Context, id uuid.UUID) (dto.Order, error)
	findByIDWithItems            func(ctx context.Context, id uuid.UUID) (dto.Order, error)
	findByIDWithItemsAndProducts func(ctx context.Context, id uuid.UUID) (dto.Order, error)
	findByOrderNumber            func(ctx context.Context, orderNumber string) (dto.Order, error)
	findByOrderNumberAndPhone    func(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error)
	findRecentByPhone            func(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error)
	createFn                     func(ctx context.Context, input repository.CreateOrderInput) (dto.Order, error)
	approveFn                    func(ctx context.Context, id uuid.UUID, reviewerID *string) (dto.Order, error)
	rejectFn                     func(ctx context.Context, id uuid.UUID, reason string, reviewerID *string) (dto.Order, error)
	updateStatusFn               func(ctx context.Context, id uuid.UUID, status string) (dto.Order, error)
	updateNotesFn                func(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error)
	setArchivedFn                func(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error)
	deleteFn                     func(ctx context.Context, id uuid.UUID) error
}

func (s *orderRepoStub) ListPaginated(ctx context.Context, _ scope.Location, status string, archived bool, page, limit int) ([]dto.Order, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, status, archived, page, limit)
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

func (s *orderRepoStub) FindByIDWithItemsAndProducts(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	if s.findByIDWithItemsAndProducts != nil {
		return s.findByIDWithItemsAndProducts(ctx, id)
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

func (s *orderRepoStub) Approve(ctx context.Context, id uuid.UUID, reviewerID *string, expectedStatus string) (dto.Order, error) {
	if s.approveFn != nil {
		return s.approveFn(ctx, id, reviewerID)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string, expectedStatus string) (dto.Order, error) {
	if s.rejectFn != nil {
		return s.rejectFn(ctx, id, reason, reviewerID)
	}
	return dto.Order{}, nil
}

func (s *orderRepoStub) UpdateStatus(ctx context.Context, id uuid.UUID, status, expectedStatus string) (dto.Order, error) {
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

func (s *orderRepoStub) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error) {
	if s.setArchivedFn != nil {
		return s.setArchivedFn(ctx, id, archived)
	}
	return dto.Order{ID: id, IsArchived: archived}, nil
}

func (s *orderRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *orderRepoStub) FindByDateRange(ctx context.Context, _ scope.Location, start, end time.Time, statuses []string) ([]dto.Order, error) {
	return nil, nil
}

func (s *orderRepoStub) FindByDateRangeWithItems(ctx context.Context, _ scope.Location, start, end time.Time, statuses []string) ([]dto.Order, error) {
	return nil, nil
}

func (s *orderRepoStub) FindOrderItemsByOrderDateRange(ctx context.Context, _ scope.Location, start, end time.Time, excludeStatuses []string) ([]dto.OrderItem, error) {
	return nil, nil
}

func (s *orderRepoStub) Purge(ctx context.Context, _ scope.Location, start, end time.Time, statuses []string) (int, int, error) {
	return 0, 0, nil
}

func (s *orderRepoStub) FindEligibleForArchive(ctx context.Context, terminalStatuses []string, cutoff time.Time, limit int) ([]dto.Order, error) {
	return nil, nil
}

func (s *orderRepoStub) ArchiveOrders(ctx context.Context, ids []uuid.UUID) (int, error) {
	return 0, nil
}

func (s *orderRepoStub) AnalyticsOverview(ctx context.Context, _ scope.Location, start, end time.Time) (dto.AnalyticsOverview, error) {
	return dto.AnalyticsOverview{StatusBreakdown: map[string]int{}}, nil
}

func (s *orderRepoStub) AnalyticsOrdersOverTime(ctx context.Context, _ scope.Location, start, end time.Time, groupBy string) ([]dto.AnalyticsTimelinePoint, error) {
	return nil, nil
}

func (s *orderRepoStub) AnalyticsTopProducts(ctx context.Context, _ scope.Location, start, end time.Time, limit int) ([]dto.AnalyticsTopProduct, error) {
	return nil, nil
}

func (s *orderRepoStub) StreamOrderExportRows(ctx context.Context, _ scope.Location, start, end time.Time, statuses []string, batchSize int, cb func(dto.ExportOrderRow) error) error {
	return nil
}

type orderStoreStub struct {
	createFn                     func(ctx context.Context, input repository.StoreOrderInput) (dto.Order, error)
	findByIdempotencyKeyFn       func(ctx context.Context, key uuid.UUID) (dto.Order, error)
	updateFn                     func(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput) (dto.Order, error)
	updateWithAdjustmentFn       func(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error)
	updateStatusFn               func(ctx context.Context, id uuid.UUID, status string) (dto.Order, error)
	updateStatusWithAdjustmentFn func(ctx context.Context, id uuid.UUID, status string, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error)
}

func (s *orderStoreStub) CreateOrder(ctx context.Context, input repository.StoreOrderInput) (dto.Order, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Order{}, nil
}

func (s *orderStoreStub) FindOrderByIdempotencyKey(ctx context.Context, key uuid.UUID) (dto.Order, error) {
	if s.findByIdempotencyKeyFn != nil {
		return s.findByIdempotencyKeyFn(ctx, key)
	}
	return dto.Order{}, &apperror.NotFoundError{Resource: "idempotency_key"}
}

func (s *orderStoreStub) UpdateOrder(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput) (dto.Order, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Order{}, nil
}

func (s *orderStoreStub) UpdateOrderWithStockAdjustment(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error) {
	if s.updateWithAdjustmentFn != nil {
		return s.updateWithAdjustmentFn(ctx, id, input, deductions, restore)
	}
	return dto.Order{ID: id, Status: input.PaymentMethod}, nil
}

func (s *orderStoreStub) UpdateStatus(ctx context.Context, id uuid.UUID, status, expectedStatus string) (dto.Order, error) {
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, id, status)
	}
	return dto.Order{ID: id, Status: status}, nil
}

func (s *orderStoreStub) UpdateStatusWithStockAdjustment(ctx context.Context, id uuid.UUID, status, expectedStatus string, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error) {
	if s.updateStatusWithAdjustmentFn != nil {
		return s.updateStatusWithAdjustmentFn(ctx, id, status, deductions, restore)
	}
	return dto.Order{ID: id, Status: status}, nil
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
	repo := &orderRepoStub{listFn: func(_ context.Context, _ string, _ bool, _, _ int) ([]dto.Order, int, error) {
		return []dto.Order{{OrderNumber: "ORD-1"}}, 1, nil
	}}
	svc := NewOrderQueryService(repo)

	orders, total, err := svc.ListPaginated(context.Background(), scope.All(), "", false, 1, 10)
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
	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)

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
	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)

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
	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)

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
	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)

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
	svc := NewOrderCommandService(repo, &orderStoreStub{}, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)

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

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, &productRepoStub{}, nil, nil, nil)
	_, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:    testLocation,
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

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, &productRepoStub{}, nil, nil, nil)
	_, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:    testLocation,
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

func TestOrderCommandService_Create_WithIdempotencyKey_ExistingOrder_ReturnsOriginalWithoutCreating(t *testing.T) {
	key := uuid.New()
	existing := dto.Order{ID: uuid.New(), OrderNumber: "ORD-IK-001", LocationID: testLocation}

	store := &orderStoreStub{
		findByIdempotencyKeyFn: func(_ context.Context, got uuid.UUID) (dto.Order, error) {
			if got != key {
				t.Fatalf("expected key %q, got %q", key, got)
			}
			return existing, nil
		},
		createFn: func(_ context.Context, _ repository.StoreOrderInput) (dto.Order, error) {
			t.Fatal("CreateOrder should not be called when an idempotency key already has an order")
			return dto.Order{}, nil
		},
	}

	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)
	got, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:     testLocation,
		IdempotencyKey: &key,
		CustomerName:   "Jane",
		CustomerPhone:  "5550000",
		PaymentMethod:  "efectivo",
		OrderType:      "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OrderNumber != existing.OrderNumber {
		t.Fatalf("expected order %q, got %+v", existing.OrderNumber, got)
	}
}

func TestOrderCommandService_Create_WithIdempotencyKey_NoExisting_CreatesNormally(t *testing.T) {
	key := uuid.New()
	customerID := uuid.New()
	customerRepo := &customerRepoStub{
		findByPhone: func(_ context.Context, _ string) (dto.Customer, error) {
			return dto.Customer{ID: customerID}, nil
		},
	}

	called := false
	store := &orderStoreStub{
		findByIdempotencyKeyFn: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
			return dto.Order{}, &apperror.NotFoundError{Resource: "idempotency_key"}
		},
		createFn: func(_ context.Context, input repository.StoreOrderInput) (dto.Order, error) {
			called = true
			if input.IdempotencyKey == nil || *input.IdempotencyKey != key {
				t.Fatalf("expected idempotency key %q in store input, got %v", key, input.IdempotencyKey)
			}
			return dto.Order{OrderNumber: "ORD-IK-002"}, nil
		},
	}

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, &productRepoStub{}, nil, nil, nil)
	_, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:     testLocation,
		IdempotencyKey: &key,
		CustomerName:   "Jane",
		CustomerPhone:  "5550000",
		PaymentMethod:  "efectivo",
		OrderType:      "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected CreateOrder to be called")
	}
}

func TestOrderCommandService_Create_WithoutIdempotencyKey_Unaffected(t *testing.T) {
	customerRepo := &customerRepoStub{
		findByPhone: func(_ context.Context, _ string) (dto.Customer, error) {
			return dto.Customer{ID: uuid.New()}, nil
		},
	}

	findCalled := false
	store := &orderStoreStub{
		findByIdempotencyKeyFn: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
			findCalled = true
			return dto.Order{}, nil
		},
		createFn: func(_ context.Context, _ repository.StoreOrderInput) (dto.Order, error) {
			return dto.Order{OrderNumber: "ORD-NO-IK"}, nil
		},
	}

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, &productRepoStub{}, nil, nil, nil)
	_, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:    testLocation,
		CustomerName:  "Jane",
		CustomerPhone: "5550000",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findCalled {
		t.Fatal("FindOrderByIdempotencyKey should not be called when no key is supplied")
	}
}

func TestOrderCommandService_Create_IdempotencyKeyRaceLoser_FetchesWinner(t *testing.T) {
	key := uuid.New()
	winner := dto.Order{ID: uuid.New(), OrderNumber: "ORD-IK-WINNER", LocationID: testLocation}
	customerRepo := &customerRepoStub{
		findByPhone: func(_ context.Context, _ string) (dto.Customer, error) {
			return dto.Customer{ID: uuid.New()}, nil
		},
	}

	store := &orderStoreStub{
		findByIdempotencyKeyFn: func(_ context.Context, got uuid.UUID) (dto.Order, error) {
			if got != key {
				t.Fatalf("expected key %q, got %q", key, got)
			}
			return winner, nil
		},
		createFn: func(_ context.Context, _ repository.StoreOrderInput) (dto.Order, error) {
			return dto.Order{}, errors.New("duplicate key value violates unique constraint \"idempotency_keys_pkey\"")
		},
	}

	svc := NewOrderCommandService(&orderRepoStub{}, store, customerRepo, &productRepoStub{}, nil, nil, nil)
	got, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:     testLocation,
		IdempotencyKey: &key,
		CustomerName:   "Jane",
		CustomerPhone:  "5550000",
		PaymentMethod:  "efectivo",
		OrderType:      "para_llevar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OrderNumber != winner.OrderNumber {
		t.Fatalf("expected winner order %+v, got %+v", winner, got)
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

	svc := NewOrderCommandService(repo, store, customerRepo, &productRepoStub{}, nil, nil, nil)
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

// --- Atomic status + stock adjustment tests -------------------------------
//
// These tests prove that UpdateStatus routes stock-affecting transitions
// through OrderStore.UpdateStatusWithStockAdjustment (the atomic path) and
// propagates stock errors instead of swallowing them. The previous
// implementation discarded inventory errors with `_, _ =`.

func TestOrderCommandService_UpdateStatus_EnProceso_RoutesToAtomicStore(t *testing.T) {
	orderID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "recibida"}, nil
		},
		findByIDWithItems: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Items: []dto.OrderItem{}}, nil
		},
	}

	atomicCalled := false
	plainCalled := false
	store := &orderStoreStub{
		updateStatusFn: func(_ context.Context, _ uuid.UUID, _ string) (dto.Order, error) {
			plainCalled = true
			return dto.Order{}, nil
		},
		updateStatusWithAdjustmentFn: func(_ context.Context, id uuid.UUID, status string, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error) {
			atomicCalled = true
			if restore {
				t.Fatal("expected restore=false for en_proceso deduction")
			}
			return dto.Order{ID: id, Status: status}, nil
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, inventory, nil, nil)
	_, err := svc.UpdateStatus(context.Background(), orderID, "en_proceso")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !atomicCalled {
		t.Fatal("expected UpdateStatusWithStockAdjustment to be called for en_proceso transition")
	}
	if plainCalled {
		t.Fatal("plain UpdateStatus should not be called for en_proceso transition")
	}
}

func TestOrderCommandService_UpdateStatus_EnProceso_PropagatesStockError(t *testing.T) {
	orderID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "recibida"}, nil
		},
		findByIDWithItems: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Items: []dto.OrderItem{}}, nil
		},
	}

	stockErr := apperror.ErrConflict
	store := &orderStoreStub{
		updateStatusWithAdjustmentFn: func(_ context.Context, _ uuid.UUID, _ string, _ []repository.StockDeductionInput, _ bool) (dto.Order, error) {
			return dto.Order{}, stockErr
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, inventory, nil, nil)
	_, err := svc.UpdateStatus(context.Background(), orderID, "en_proceso")
	if !errors.Is(err, stockErr) {
		t.Fatalf("expected stock error to propagate, got %v", err)
	}
}

func TestOrderCommandService_UpdateStatus_Cancelada_RoutesToAtomicStore(t *testing.T) {
	orderID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "en_proceso"}, nil
		},
	}

	atomicCalled := false
	plainCalled := false
	store := &orderStoreStub{
		updateStatusFn: func(_ context.Context, _ uuid.UUID, _ string) (dto.Order, error) {
			plainCalled = true
			return dto.Order{}, nil
		},
		updateStatusWithAdjustmentFn: func(_ context.Context, id uuid.UUID, status string, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error) {
			atomicCalled = true
			if !restore {
				t.Fatal("expected restore=true for cancelada transition")
			}
			if len(deductions) != 0 {
				t.Fatalf("expected no deductions for cancelada, got %d", len(deductions))
			}
			return dto.Order{ID: id, Status: status}, nil
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, inventory, nil, nil)
	_, err := svc.UpdateStatus(context.Background(), orderID, "cancelada")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !atomicCalled {
		t.Fatal("expected UpdateStatusWithStockAdjustment to be called for cancelada transition")
	}
	if plainCalled {
		t.Fatal("plain UpdateStatus should not be called for cancelada transition")
	}
}

func TestOrderCommandService_UpdateStatus_Cancelada_PropagatesStockError(t *testing.T) {
	orderID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "en_proceso"}, nil
		},
	}

	stockErr := apperror.ErrConflict
	store := &orderStoreStub{
		updateStatusWithAdjustmentFn: func(_ context.Context, _ uuid.UUID, _ string, _ []repository.StockDeductionInput, _ bool) (dto.Order, error) {
			return dto.Order{}, stockErr
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, inventory, nil, nil)
	_, err := svc.UpdateStatus(context.Background(), orderID, "cancelada")
	if !errors.Is(err, stockErr) {
		t.Fatalf("expected stock error to propagate, got %v", err)
	}
}

func TestOrderCommandService_UpdateStatus_NonStockTransition_UsesPlainUpdate(t *testing.T) {
	orderID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "en_proceso"}, nil
		},
	}

	atomicCalled := false
	plainCalled := false
	store := &orderStoreStub{
		updateStatusFn: func(_ context.Context, id uuid.UUID, status string) (dto.Order, error) {
			plainCalled = true
			return dto.Order{ID: id, Status: status}, nil
		},
		updateStatusWithAdjustmentFn: func(_ context.Context, _ uuid.UUID, _ string, _ []repository.StockDeductionInput, _ bool) (dto.Order, error) {
			atomicCalled = true
			return dto.Order{}, nil
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, &productRepoStub{}, inventory, nil, nil)
	// en_proceso → lista is a valid transition that does not affect stock.
	_, err := svc.UpdateStatus(context.Background(), orderID, "lista")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plainCalled {
		t.Fatal("expected plain UpdateStatus for non-stock transition (lista)")
	}
	if atomicCalled {
		t.Fatal("atomic UpdateStatusWithStockAdjustment should not be called for lista transition")
	}
}

// --- Atomic order item update + stock adjustment tests --------------------

func TestOrderCommandService_Update_EnProcesoWithItems_RoutesToAtomicStore(t *testing.T) {
	orderID := uuid.New()
	productID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "en_proceso", CustomerPhone: "5550000"}, nil
		},
	}

	atomicCalled := false
	plainCalled := false
	store := &orderStoreStub{
		updateFn: func(_ context.Context, _ uuid.UUID, _ repository.StoreUpdateOrderInput) (dto.Order, error) {
			plainCalled = true
			return dto.Order{}, nil
		},
		updateWithAdjustmentFn: func(_ context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error) {
			atomicCalled = true
			if !restore {
				t.Fatal("expected restore=true when replacing items on en_proceso order")
			}
			return dto.Order{ID: id, Status: "en_proceso"}, nil
		},
	}
	productRepo := &productRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Product, error) {
			return dto.Product{ID: id, Name: "Test", BallBased: false}, nil
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{
		findByProductIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
		},
	}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, productRepo, inventory, nil, nil)
	_, err := svc.Update(context.Background(), orderID, UpdateOrderInput{
		CustomerName:  "Jane",
		CustomerPhone: "5550000",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
		Items: []OrderItemInput{
			{ProductID: productID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !atomicCalled {
		t.Fatal("expected UpdateOrderWithStockAdjustment for en_proceso item replacement")
	}
	if plainCalled {
		t.Fatal("plain UpdateOrder should not be called when stock adjustment is needed")
	}
}

func TestOrderCommandService_Update_EnProcesoWithItems_PropagatesStockError(t *testing.T) {
	orderID := uuid.New()
	productID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "en_proceso", CustomerPhone: "5550000"}, nil
		},
	}

	stockErr := apperror.ErrConflict
	store := &orderStoreStub{
		updateWithAdjustmentFn: func(_ context.Context, _ uuid.UUID, _ repository.StoreUpdateOrderInput, _ []repository.StockDeductionInput, _ bool) (dto.Order, error) {
			return dto.Order{}, stockErr
		},
	}
	productRepo := &productRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Product, error) {
			return dto.Product{ID: id, Name: "Test"}, nil
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, productRepo, inventory, nil, nil)
	_, err := svc.Update(context.Background(), orderID, UpdateOrderInput{
		CustomerName:  "Jane",
		CustomerPhone: "5550000",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
		Items: []OrderItemInput{
			{ProductID: productID, Quantity: 2},
		},
	})
	if !errors.Is(err, stockErr) {
		t.Fatalf("expected stock error to propagate, got %v", err)
	}
}

func TestOrderCommandService_Update_PendienteStatusWithItems_UsesPlainUpdate(t *testing.T) {
	orderID := uuid.New()
	productID := uuid.New()
	repo := &orderRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: id, Status: "pendiente_revision", CustomerPhone: "5550000"}, nil
		},
	}

	atomicCalled := false
	plainCalled := false
	store := &orderStoreStub{
		updateFn: func(_ context.Context, id uuid.UUID, _ repository.StoreUpdateOrderInput) (dto.Order, error) {
			plainCalled = true
			return dto.Order{ID: id, Status: "pendiente_revision"}, nil
		},
		updateWithAdjustmentFn: func(_ context.Context, _ uuid.UUID, _ repository.StoreUpdateOrderInput, _ []repository.StockDeductionInput, _ bool) (dto.Order, error) {
			atomicCalled = true
			return dto.Order{}, nil
		},
	}
	productRepo := &productRepoStub{
		findByID: func(_ context.Context, id uuid.UUID) (dto.Product, error) {
			return dto.Product{ID: id, Name: "Test"}, nil
		},
	}
	inventory := NewInventoryService(&inventoryRepoStub{}, nil)

	svc := NewOrderCommandService(repo, store, &customerRepoStub{}, productRepo, inventory, nil, nil)
	_, err := svc.Update(context.Background(), orderID, UpdateOrderInput{
		CustomerName:  "Jane",
		CustomerPhone: "5550000",
		PaymentMethod: "efectivo",
		OrderType:     "para_llevar",
		Items: []OrderItemInput{
			{ProductID: productID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plainCalled {
		t.Fatal("expected plain UpdateOrder for pendiente_revision order (no stock deducted yet)")
	}
	if atomicCalled {
		t.Fatal("atomic UpdateOrderWithStockAdjustment should not be called when order has no deducted stock")
	}
}

// testLocation is the location service-level order tests create orders at.
var testLocation = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// An idempotency key reused at another location must not hand back (or leak)
// that location's order.
func TestOrderCommandService_Create_IdempotencyKeyFromOtherLocation_Conflicts(t *testing.T) {
	key := uuid.New()
	otherLocation := uuid.New()
	store := &orderStoreStub{
		findByIdempotencyKeyFn: func(_ context.Context, _ uuid.UUID) (dto.Order, error) {
			return dto.Order{ID: uuid.New(), OrderNumber: "ORD-OTHER", LocationID: otherLocation}, nil
		},
		createFn: func(_ context.Context, _ repository.StoreOrderInput) (dto.Order, error) {
			t.Fatal("CreateOrder should not be called")
			return dto.Order{}, nil
		},
	}
	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)

	got, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:     testLocation,
		IdempotencyKey: &key,
		CustomerName:   "Ana",
		CustomerPhone:  "70000000",
		PaymentMethod:  "efectivo",
		OrderType:      "para_llevar",
		Items:          []OrderItemInput{{ProductID: uuid.New(), Quantity: 1}},
	})
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("expected conflict, got order %+v err %v", got, err)
	}
	if got.OrderNumber != "" {
		t.Fatal("must not return the other location's order")
	}
}

func TestOrderCommandService_Create_AcceptsOnlyStaffOrders(t *testing.T) {
	var got []bool
	store := &orderStoreStub{
		createFn: func(_ context.Context, input repository.StoreOrderInput) (dto.Order, error) {
			got = append(got, input.Accepted)
			return dto.Order{ID: uuid.New()}, nil
		},
	}
	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)
	staff := "staff-1"
	for _, createdBy := range []*string{&staff, nil} {
		if _, err := svc.Create(context.Background(), CreateOrderInput{
			LocationID:    testLocation,
			CustomerName:  "Ana",
			CustomerPhone: "70000000",
			PaymentMethod: "efectivo",
			OrderType:     "para_llevar",
			Items:         []OrderItemInput{{ProductID: uuid.New(), Quantity: 1}},
			CreatedByID:   createdBy,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("staff order must be accepted and online order not; got %v", got)
	}
}

func TestOrderService_IsOrderEditable_CounterOrderWhileAccepted(t *testing.T) {
	svc := &OrderService{}
	staff := "staff-1"
	if !svc.IsOrderEditable(dto.Order{Status: "recibida", CreatedByUserID: &staff}) {
		t.Fatal("a counter order must stay editable while accepted")
	}
	if svc.IsOrderEditable(dto.Order{Status: "recibida"}) {
		t.Fatal("an accepted online order stays locked")
	}
	if svc.IsOrderEditable(dto.Order{Status: "entregada", CreatedByUserID: &staff}) {
		t.Fatal("delivered orders stay locked")
	}
}
