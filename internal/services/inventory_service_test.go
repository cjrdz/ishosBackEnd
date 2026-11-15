package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

type inventoryRepoStub struct {
	listFn            func(ctx context.Context) ([]dto.InventoryItem, error)
	listByTypeFn      func(ctx context.Context, itemType string) ([]dto.InventoryItem, error)
	listLinkedFn      func(ctx context.Context) ([]dto.InventoryItem, error)
	findByIDFn        func(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error)
	findByFlavorIDFn  func(ctx context.Context, flavorID uuid.UUID) (dto.InventoryItem, error)
	findByProductIDFn func(ctx context.Context, productID uuid.UUID) (dto.InventoryItem, error)
	// findByProductIDLocFn, when set, answers per location (takes precedence).
	findByProductIDLocFn func(locationID uuid.UUID) (dto.InventoryItem, error)
	createFn             func(ctx context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error)
	recordAdjustmentFn   func(ctx context.Context, input repository.RecordAdjustmentInput) (dto.StockMovement, error)
	recordEntryFn        func(ctx context.Context, input repository.RecordEntryInput) (dto.StockMovement, error)
	recordUnitEntryFn    func(ctx context.Context, input repository.RecordUnitEntryInput) (dto.StockMovement, error)
	restoreForOrderFn    func(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error)
	deductForOrderFn     func(ctx context.Context, orderID uuid.UUID, deductions []repository.StockDeductionInput, createdByID string) ([]dto.StockMovement, error)
}

func (s *inventoryRepoStub) List(ctx context.Context, _ scope.Location) ([]dto.InventoryItem, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}

func (s *inventoryRepoStub) ListByType(ctx context.Context, _ scope.Location, itemType string) ([]dto.InventoryItem, error) {
	if s.listByTypeFn != nil {
		return s.listByTypeFn(ctx, itemType)
	}
	return nil, nil
}

func (s *inventoryRepoStub) ListLinked(ctx context.Context, _ scope.Location) ([]dto.InventoryItem, error) {
	if s.listLinkedFn != nil {
		return s.listLinkedFn(ctx)
	}
	return nil, nil
}

func (s *inventoryRepoStub) UpdateLowStockThreshold(_ context.Context, id uuid.UUID, threshold int) (dto.InventoryItem, error) {
	return dto.InventoryItem{ID: id, LowStockThreshold: threshold}, nil
}

func (s *inventoryRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) FindByFlavorID(ctx context.Context, _ uuid.UUID, flavorID uuid.UUID) (dto.InventoryItem, error) {
	if s.findByFlavorIDFn != nil {
		return s.findByFlavorIDFn(ctx, flavorID)
	}
	return dto.InventoryItem{}, nil
}

func (s *inventoryRepoStub) FindByProductID(ctx context.Context, locationID, productID uuid.UUID) (dto.InventoryItem, error) {
	if s.findByProductIDLocFn != nil {
		return s.findByProductIDLocFn(locationID)
	}
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

	_, err := svc.GetByID(context.Background(), scope.All(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestInventoryService_CheckStock_NotTracked(t *testing.T) {
	repo := &inventoryRepoStub{findByFlavorIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
		return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
	}}
	svc := NewInventoryService(repo, &containerTypeRepoStub{})

	ok, err := svc.CheckStock(context.Background(), uuid.New(), uuid.New(), 100)
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

	ok, err := svc.CheckStock(context.Background(), uuid.New(), uuid.New(), 10)
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

	items, err := svc.GetLowStockItems(context.Background(), scope.All())
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

	stats, err := svc.GetInventoryStats(context.Background(), scope.All())
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

	_, err := svc.RecordAdjustment(context.Background(), scope.All(), uuid.New(), -10, "spoilage", "staff-1")
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

	_, err := svc.RecordEntry(context.Background(), uuid.New(), uuid.New(), uuid.New(), 1, 0, "staff-1")
	if !errors.Is(err, apperror.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestInventoryService_RecordEntry_AutoCreatesFlavorInventoryItem(t *testing.T) {
	flavorID := uuid.New()
	containerTypeID := uuid.New()
	inventoryItemID := uuid.New()

	repo := &inventoryRepoStub{
		findByFlavorIDFn: func(_ context.Context, id uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
		},
		createFn: func(_ context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error) {
			if input.Name != "Vainilla" {
				t.Fatalf("unexpected name %q", input.Name)
			}
			if input.Type != "ball_based" {
				t.Fatalf("unexpected type %q", input.Type)
			}
			if input.CurrentStock != 0 {
				t.Fatalf("unexpected current_stock %d", input.CurrentStock)
			}
			if input.LowStockThreshold != defaultFlavorLowStockThreshold {
				t.Fatalf("unexpected threshold %d", input.LowStockThreshold)
			}
			if input.FlavorID == nil || *input.FlavorID != flavorID {
				t.Fatalf("unexpected flavor_id")
			}
			return dto.InventoryItem{
				ID:                inventoryItemID,
				Name:              input.Name,
				Type:              input.Type,
				CurrentStock:      input.CurrentStock,
				LowStockThreshold: input.LowStockThreshold,
				FlavorID:          input.FlavorID,
			}, nil
		},
		recordEntryFn: func(_ context.Context, input repository.RecordEntryInput) (dto.StockMovement, error) {
			if input.InventoryItemID != inventoryItemID {
				t.Fatalf("expected inventory item %s, got %s", inventoryItemID, input.InventoryItemID)
			}
			if input.TotalBalls != 100 {
				t.Fatalf("expected total balls 100, got %d", input.TotalBalls)
			}
			return dto.StockMovement{ID: uuid.New(), InventoryItemID: inventoryItemID, Quantity: 100}, nil
		},
	}
	ctRepo := &containerTypeRepoStub{getByID: func(_ context.Context, _ uuid.UUID) (dto.ContainerType, error) {
		return dto.ContainerType{ID: containerTypeID, Name: "Big", BallsPerContainer: 100, IsCustom: false}, nil
	}}
	flavorRepo := &flavorRepoStub{findByID: func(_ context.Context, id uuid.UUID) (dto.Flavor, error) {
		if id != flavorID {
			t.Fatalf("unexpected flavor id %s", id)
		}
		return dto.Flavor{ID: flavorID, Name: "Vainilla"}, nil
	}}
	svc := NewInventoryService(repo, ctRepo, WithFlavorRepo(flavorRepo))

	movement, err := svc.RecordEntry(context.Background(), uuid.New(), flavorID, containerTypeID, 1, 0, "staff-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if movement.Quantity != 100 {
		t.Fatalf("expected quantity 100, got %d", movement.Quantity)
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
	_, err := svc.DeductForOrder(context.Background(), uuid.New(), uuid.New(), []dto.OrderItem{
		{ProductID: productID, Quantity: 2, Product: &dto.Product{ID: productID, Name: "Cup", BallBased: false}},
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInventoryService_CreateUnitInventoryItem(t *testing.T) {
	s1, s2 := uuid.New(), uuid.New()
	trackedAt := map[uuid.UUID]bool{s2: true} // Sucursal 2 already tracks it
	var createdAt []uuid.UUID
	repo := &inventoryRepoStub{
		createFn: func(_ context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error) {
			if input.Type != "unit_based" {
				t.Fatalf("expected unit_based, got %s", input.Type)
			}
			createdAt = append(createdAt, input.LocationID)
			return dto.InventoryItem{Name: input.Name, LocationID: input.LocationID}, nil
		},
		findByProductIDLocFn: func(locationID uuid.UUID) (dto.InventoryItem, error) {
			if trackedAt[locationID] {
				return dto.InventoryItem{LocationID: locationID}, nil
			}
			return dto.InventoryItem{}, &apperror.NotFoundError{Resource: "inventory_item"}
		},
	}
	svc := NewInventoryService(repo, &containerTypeRepoStub{},
		WithLocationIDs(func(context.Context) ([]uuid.UUID, error) { return []uuid.UUID{s1, s2}, nil }))

	items, err := svc.CreateUnitInventoryItem(context.Background(), "Cones", uuid.New(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(createdAt) != 1 || createdAt[0] != s1 {
		t.Fatalf("expected one row at the untracked location, got %v", createdAt)
	}
	if len(items) != 1 || items[0].Name != "Cones" || items[0].LocationID != s1 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestEnsureAvailable_SumsFlavourBallsAcrossItems(t *testing.T) {
	flavorID := uuid.New()
	itemID := uuid.New()
	repo := &inventoryRepoStub{
		findByFlavorIDFn: func(_ context.Context, id uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{ID: itemID, FlavorID: &id, Type: "ball_based", Name: "Mango", CurrentStock: 3}, nil
		},
		findByIDFn: func(_ context.Context, id uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{ID: id, Type: "ball_based", Name: "Mango", CurrentStock: 3}, nil
		},
	}
	svc := NewInventoryService(repo, nil)
	cone := &dto.Product{ID: uuid.New(), BallBased: true}
	items := []dto.OrderItem{
		{Product: cone, Quantity: 1, Customizations: map[string]any{"flavor_ids": []any{flavorID.String(), flavorID.String()}}},
		{Product: cone, Quantity: 2, Customizations: map[string]any{"flavor_ids": []any{flavorID.String()}}},
	}
	err := svc.EnsureAvailable(context.Background(), uuid.New(), items, nil)
	var stockErr *apperror.InsufficientStockError
	if !errors.As(err, &stockErr) {
		t.Fatalf("expected InsufficientStockError, got %v", err)
	}
	if stockErr.Needed != 4 || stockErr.Available != 3 || stockErr.Unit != "bola" {
		t.Fatalf("unexpected stock error: %+v", stockErr)
	}
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatal("stock errors must map to 409")
	}

	if err := svc.EnsureAvailable(context.Background(), uuid.New(), items[:1], nil); err != nil {
		t.Fatalf("2 of 3 balls should be available, got %v", err)
	}
}

func TestEnsureAvailable_UntrackedIsAlwaysAvailable(t *testing.T) {
	repo := &inventoryRepoStub{
		findByProductIDFn: func(_ context.Context, _ uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{}, apperror.ErrNotFound
		},
	}
	svc := NewInventoryService(repo, nil)
	items := []dto.OrderItem{{Product: &dto.Product{ID: uuid.New()}, Quantity: 50}}
	if err := svc.EnsureAvailable(context.Background(), uuid.New(), items, nil); err != nil {
		t.Fatalf("untracked products must not be refused, got %v", err)
	}
}

func TestStockLevel(t *testing.T) {
	cases := []struct {
		stock, min int
		want       string
	}{
		{0, 20, "out"}, {-1, 0, "out"}, {19, 20, "low"}, {20, 20, "ok"}, {100, 100, "ok"}, {5, 0, "ok"},
	}
	for _, tc := range cases {
		if got := StockLevel(tc.stock, tc.min); got != tc.want {
			t.Errorf("StockLevel(%d, %d) = %q, want %q", tc.stock, tc.min, got, tc.want)
		}
	}
}

func TestUpdateLowStockThreshold_Bounds(t *testing.T) {
	repo := &inventoryRepoStub{}
	svc := NewInventoryService(repo, nil)
	for _, bad := range []int{-1, MaxLowStockThreshold + 1} {
		if _, err := svc.UpdateLowStockThreshold(context.Background(), scope.All(), uuid.New(), bad); !errors.Is(err, apperror.ErrValidation) {
			t.Errorf("threshold %d: expected validation error, got %v", bad, err)
		}
	}
}

func TestEnsureAvailable_EditOnlyChecksIncreases(t *testing.T) {
	flavorID := uuid.New()
	repo := &inventoryRepoStub{
		findByFlavorIDFn: func(_ context.Context, id uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{ID: flavorID, FlavorID: &id, Type: "ball_based", Name: "Mango"}, nil
		},
		findByIDFn: func(_ context.Context, id uuid.UUID) (dto.InventoryItem, error) {
			return dto.InventoryItem{ID: id, Type: "ball_based", Name: "Mango", CurrentStock: 2}, nil
		},
	}
	svc := NewInventoryService(repo, nil)
	cone := &dto.Product{ID: uuid.New(), BallBased: true}
	three := []dto.OrderItem{{Product: cone, Quantity: 3, Customizations: map[string]any{"flavor_ids": []any{flavorID.String()}}}}
	four := []dto.OrderItem{{Product: cone, Quantity: 4, Customizations: map[string]any{"flavor_ids": []any{flavorID.String()}}}}
	if err := svc.EnsureAvailable(context.Background(), uuid.New(), three, three); err != nil {
		t.Fatalf("unchanged items must not be refused when stock ran low: %v", err)
	}
	var stockErr *apperror.InsufficientStockError
	if err := svc.EnsureAvailable(context.Background(), uuid.New(), four, three); !errors.As(err, &stockErr) {
		t.Fatalf("a larger need must still be checked, got %v", err)
	}
}
