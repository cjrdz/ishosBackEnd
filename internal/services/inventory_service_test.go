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

type inventoryRepoStub struct {
	listFn             func(ctx context.Context) ([]dto.InventoryItem, error)
	listByTypeFn       func(ctx context.Context, itemType string) ([]dto.InventoryItem, error)
	listLinkedFn       func(ctx context.Context) ([]dto.InventoryItem, error)
	findByIDFn         func(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error)
	findByFlavorIDFn   func(ctx context.Context, flavorID uuid.UUID) (dto.InventoryItem, error)
	findByProductIDFn  func(ctx context.Context, productID uuid.UUID) (dto.InventoryItem, error)
	createFn           func(ctx context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error)
	recordAdjustmentFn func(ctx context.Context, input repository.RecordAdjustmentInput) (dto.StockMovement, error)
	recordEntryFn      func(ctx context.Context, input repository.RecordEntryInput) (dto.StockMovement, error)
	recordUnitEntryFn  func(ctx context.Context, input repository.RecordUnitEntryInput) (dto.StockMovement, error)
	restoreForOrderFn  func(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error)
	deductForOrderFn   func(ctx context.Context, orderID uuid.UUID, deductions []repository.StockDeductionInput, createdByID string) ([]dto.StockMovement, error)
}

func (s *inventoryRepoStub) List(ctx context.Context) ([]dto.InventoryItem, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}

func (s *inventoryRepoStub) ListByType(ctx context.Context, itemType string) ([]dto.InventoryItem, error) {
	if s.listByTypeFn != nil {
		return s.listByTypeFn(ctx, itemType)
	}
	return nil, nil
}

func (s *inventoryRepoStub) ListLinked(ctx context.Context) ([]dto.InventoryItem, error) {
	if s.listLinkedFn != nil {
		return s.listLinkedFn(ctx)
	}
	return nil, nil
}

func (s *inventoryRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) FindByFlavorID(ctx context.Context, flavorID uuid.UUID) (dto.InventoryItem, error) {
	if s.findByFlavorIDFn != nil {
		return s.findByFlavorIDFn(ctx, flavorID)
	}
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) FindByProductID(ctx context.Context, productID uuid.UUID) (dto.InventoryItem, error) {
	if s.findByProductIDFn != nil {
		return s.findByProductIDFn(ctx, productID)
	}
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) Create(ctx context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) Update(ctx context.Context, id uuid.UUID, input repository.UpdateInventoryItemInput) (dto.InventoryItem, error) {
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (s *inventoryRepoStub) LinkToProduct(ctx context.Context, inventoryItemID, productID uuid.UUID) (dto.InventoryItem, error) {
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) UnlinkFromProduct(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) LinkToFlavor(ctx context.Context, inventoryItemID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) UnlinkFromFlavor(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error) {
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) FindStockMovementsByOrder(ctx context.Context, orderID uuid.UUID) ([]dto.StockMovement, error) {
	return nil, nil
}

func (s *inventoryRepoStub) FindStockMovementsByInventoryItem(ctx context.Context, inventoryItemID uuid.UUID, limit, offset int) ([]dto.StockMovement, int, error) {
	return nil, 0, nil
}

func (s *inventoryRepoStub) DeductStockForOrder(ctx context.Context, orderID uuid.UUID, deductions []repository.StockDeductionInput, createdByID string) ([]dto.StockMovement, error) {
	if s.deductForOrderFn != nil {
		return s.deductForOrderFn(ctx, orderID, deductions, createdByID)
	}
	return nil, nil
}

func (s *inventoryRepoStub) RestoreStockForOrder(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error) {
	if s.restoreForOrderFn != nil {
		return s.restoreForOrderFn(ctx, orderID, createdByID)
	}
	return nil, nil
}

func (s *inventoryRepoStub) RecordEntry(ctx context.Context, input repository.RecordEntryInput) (dto.StockMovement, error) {
	if s.recordEntryFn != nil {
		return s.recordEntryFn(ctx, input)
	}
	return dto.StockMovement{}, nil
}

func (s *inventoryRepoStub) RecordAdjustment(ctx context.Context, input repository.RecordAdjustmentInput) (dto.StockMovement, error) {
	if s.recordAdjustmentFn != nil {
		return s.recordAdjustmentFn(ctx, input)
	}
	return dto.StockMovement{}, nil
}

func (s *inventoryRepoStub) RecordUnitEntry(ctx context.Context, input repository.RecordUnitEntryInput) (dto.StockMovement, error) {
	if s.recordUnitEntryFn != nil {
		return s.recordUnitEntryFn(ctx, input)
	}
	return dto.StockMovement{}, nil
}

func TestInventoryService_GetByID_NotFound(t *testing.T) {
	repo := &inventoryRepoStub{findByIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestInventoryService_CheckStock_NotTracked(t *testing.T) {
	repo := &inventoryRepoStub{findByFlavorIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	ok, err := svc.CheckStock(context.Background(), uuid.New(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok for untracked flavor")
	}
}

func TestInventoryService_CheckStock_Insufficient(t *testing.T) {
	repo := &inventoryRepoStub{findByFlavorIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
		return dto.InventoryItem{CurrentStock: 5}, nil
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	ok, err := svc.CheckStock(context.Background(), uuid.New(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not ok for insufficient stock")
	}
}

func TestInventoryService_GetLowStockItems(t *testing.T) {
	repo := &inventoryRepoStub{listLinkedFn: func(_ context.Context) ([]dto.InventoryItem, error) {
		return []dto.InventoryItem{
			{ID: uuid.New(), CurrentStock: 10, LowStockThreshold: 20},
			{ID: uuid.New(), CurrentStock: 30, LowStockThreshold: 20},
		}, nil
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	items, err := svc.GetLowStockItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 low stock item, got %d", len(items))
	}
}

func TestInventoryService_GetInventoryStats(t *testing.T) {
	repo := &inventoryRepoStub{listLinkedFn: func(_ context.Context) ([]dto.InventoryItem, error) {
		return []dto.InventoryItem{
			{Type: "ball_based", CurrentStock: 10, LowStockThreshold: 5},
			{Type: "unit_based", CurrentStock: 0, LowStockThreshold: 5},
		}, nil
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	stats, err := svc.GetInventoryStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalItems != 2 {
		t.Fatalf("total = %d, want 2", stats.TotalItems)
	}
	if stats.BallBasedItems != 1 || stats.UnitBasedItems != 1 {
		t.Fatalf("unexpected type counts: %+v", stats)
	}
	if stats.OutOfStockItems != 1 || stats.HealthyItems != 1 {
		t.Fatalf("unexpected health counts: %+v", stats)
	}
}

func TestInventoryService_RecordAdjustment_NegativeStock(t *testing.T) {
	repo := &inventoryRepoStub{findByIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
		return dto.InventoryItem{CurrentStock: 5}, nil
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	_, err := svc.RecordAdjustment(context.Background(), uuid.New(), -10, "spoilage", "staff-1")
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestInventoryService_RecordEntry_ContainerTypeNotFound(t *testing.T) {
	repo := &inventoryRepoStub{findByFlavorIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
		return dto.InventoryItem{ID: uuid.New()}, nil
	}}
	ctRepo := &containerTypeRepoStub{getByID: func(_ context.Context, _ uuid.UUID) (dto.ContainerType, error) {
		return dto.ContainerType{}, &apperror.NotFoundError{Resource: "container_type"}
	}}
	svc := NewInventoryService(repo, ctRepo)

	_, err := svc.RecordEntry(context.Background(), uuid.New(), uuid.New(), 1, 0, "staff-1")
	if !errors.Is(err, apperror.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestInventoryService_DeductForOrder_SkipsUntracked(t *testing.T) {
	repo := &inventoryRepoStub{
		findByProductIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
		},
		deductForOrderFn: func(_ context.Context, _ uuid.UUID, deductions []repository.StockDeductionInput, _ string) ([]dto.StockMovement, error) {
			if len(deductions) != 0 {
				t.Fatalf("expected no deductions for untracked product, got %d", len(deductions))
			}
			return nil, nil
		},
	}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	productID := uuid.New()
	_, err := svc.DeductForOrder(context.Background(), uuid.New(), []dto.OrderItem{
		{ProductID: productID, Quantity: 2, Product: &dto.Product{ID: productID, Name: "Cup", BallBased: false}},
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInventoryService_CreateUnitInventoryItem(t *testing.T) {
	called := false
	repo := &inventoryRepoStub{createFn: func(_ context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error) {
		called = true
		if input.Type != "unit_based" {
			t.Fatalf("expected unit_based, got %s", input.Type)
		}
		return dto.InventoryItem{Name: input.Name}, nil
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	item, err := svc.CreateUnitInventoryItem(context.Background(), "Cones", uuid.New(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected repo.Create to be called")
	}
	if item.Name != "Cones" {
		t.Fatalf("unexpected item: %+v", item)
	}
}
