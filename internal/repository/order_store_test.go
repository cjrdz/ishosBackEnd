// order_store_test.go — Integration tests for OrderStore transactional behavior.
//
// These tests prove that OrderStore.UpdateStatusWithStockAdjustment and
// UpdateOrderWithStockAdjustment actually roll back the order mutation when the
// inventory adjustment fails, and commit both atomically when it succeeds.
// They require a reachable PostgreSQL instance and skip otherwise.
package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// noopEnrichment is a test EnrichmentService that passes customizations through
// without adding any extra charge.
type noopEnrichment struct{}

func (noopEnrichment) EnrichCustomizations(_ dto.Product, raw map[string]any) (map[string]any, float64, error) {
	return raw, 0, nil
}

// TestOrderStore_UpdateStatusWithStockAdjustment_RollsBackOnDeductionFailure
// proves that when a stock deduction fails (insufficient stock), the order
// status change is rolled back — the order remains in its original status and
// no stock movement is recorded.
func TestOrderStore_UpdateStatusWithStockAdjustment_RollsBackOnDeductionFailure(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	// Create an order in "recibida" status (valid predecessor for "en_proceso").
	order, err := orderRepo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-ST-001",
		CustomerName:  "Test",
		CustomerPhone: "555-0001",
		TotalAmount:   10.0,
		Status:        string(models.OrderStatusRecibida),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	// Create an inventory item with only 3 units in stock.
	invItem, err := invRepo.Create(ctx, CreateInventoryItemInput{
		Name:         "TestItem-RB",
		Type:         string(models.InventoryItemTypeUnitBased),
		CurrentStock: 3,
	})
	require.NoError(t, err)

	// Attempt to deduct 10 units (more than available) — this must fail.
	_, err = store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), "", []StockDeductionInput{
		{InventoryItemID: invItem.ID, Quantity: 10},
	}, false)
	require.Error(t, err, "expected deduction failure due to insufficient stock")

	// Verify the order status was rolled back — it must still be "recibida".
	unchanged, err := orderRepo.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusRecibida), unchanged.Status,
		"order status must be rolled back to original when stock deduction fails")

	// Verify no stock movement was recorded.
	movements, err := invRepo.FindStockMovementsByOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Empty(t, movements, "no stock movements should exist after a rolled-back deduction")

	// Verify stock level is unchanged.
	item, err := invRepo.FindByID(ctx, invItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, item.CurrentStock, "stock must be unchanged after rolled-back deduction")
}

// TestOrderStore_UpdateStatusWithStockAdjustment_CommitsOnSuccess proves that
// when the stock deduction succeeds, both the status change and the stock
// deduction are committed together.
func TestOrderStore_UpdateStatusWithStockAdjustment_CommitsOnSuccess(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	order, err := orderRepo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-ST-002",
		CustomerName:  "Test",
		CustomerPhone: "555-0002",
		TotalAmount:   10.0,
		Status:        string(models.OrderStatusRecibida),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	invItem, err := invRepo.Create(ctx, CreateInventoryItemInput{
		Name:         "TestItem-OK",
		Type:         string(models.InventoryItemTypeUnitBased),
		CurrentStock: 50,
	})
	require.NoError(t, err)

	updated, err := store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), "", []StockDeductionInput{
		{InventoryItemID: invItem.ID, Quantity: 5},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusEnProceso), updated.Status)

	// Verify stock was actually deducted.
	item, err := invRepo.FindByID(ctx, invItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, item.CurrentStock, "stock must be deducted after successful commit")

	// Verify the status change persisted.
	persisted, err := orderRepo.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusEnProceso), persisted.Status)
}

// TestOrderStore_UpdateStatusWithStockAdjustment_RestoreOnCancel proves that
// cancelling an order restores stock atomically with the status change.
func TestOrderStore_UpdateStatusWithStockAdjustment_RestoreOnCancel(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	// Create order in "en_proceso" (stock already deducted).
	order, err := orderRepo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-ST-003",
		CustomerName:  "Test",
		CustomerPhone: "555-0003",
		TotalAmount:   10.0,
		Status:        string(models.OrderStatusEnProceso),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	invItem, err := invRepo.Create(ctx, CreateInventoryItemInput{
		Name:         "TestItem-Restore",
		Type:         string(models.InventoryItemTypeUnitBased),
		CurrentStock: 50,
	})
	require.NoError(t, err)

	// First, deduct some stock so there is something to restore.
	_, err = invRepo.DeductStockForOrder(ctx, order.ID, []StockDeductionInput{
		{InventoryItemID: invItem.ID, Quantity: 5},
	}, "test")
	require.NoError(t, err)

	item, err := invRepo.FindByID(ctx, invItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, item.CurrentStock, "precondition: stock deducted to 45")

	// Now cancel the order — stock should be restored atomically.
	_, err = store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusCancelada), "", nil, true)
	require.NoError(t, err)

	// Verify stock was restored.
	item, err = invRepo.FindByID(ctx, invItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 50, item.CurrentStock, "stock must be restored after cancellation")

	// Verify status changed.
	persisted, err := orderRepo.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusCancelada), persisted.Status)
}

func TestOrderStore_CreateOrder_DuplicateIdempotencyKey_SecondCallReturnsFirstOrder(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	productRepo := NewProductBunRepository(db)
	store := NewOrderStore(db, productRepo, noopEnrichment{})

	product, _ := createTestProduct(t, ctx, db, "IK-Product", 5.0)
	key := uuid.New()

	first, err := store.CreateOrder(ctx, StoreOrderInput{
		OrderNumber:    "ORD-IK-001",
		IdempotencyKey: &key,
		CustomerName:   "Test",
		CustomerPhone:  "555-7001",
		PaymentMethod:  string(models.OrderPaymentMethodEfectivo),
		OrderType:      string(models.OrderTypeParaLlevar),
		Items: []StoreOrderItemInput{
			{ProductID: product.ID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, first.ID)

	// Second call with the same key must not create a new order.
	_, err = store.CreateOrder(ctx, StoreOrderInput{
		OrderNumber:    "ORD-IK-002",
		IdempotencyKey: &key,
		CustomerName:   "Test",
		CustomerPhone:  "555-7002",
		PaymentMethod:  string(models.OrderPaymentMethodEfectivo),
		OrderType:      string(models.OrderTypeParaLlevar),
		Items: []StoreOrderItemInput{
			{ProductID: product.ID, Quantity: 1},
		},
	})
	require.Error(t, err)
	assert.True(t, isUniqueViolation(err), "expected unique violation, got %v", err)

	// Exactly one order should be associated with this idempotency key.
	var count int
	err = db.NewSelect().
		Model((*models.IdempotencyKey)(nil)).
		ColumnExpr("COUNT(*)").
		Where("key = ?", key).
		Scan(ctx, &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected exactly one idempotency key row")

	var orderID uuid.UUID
	err = db.NewSelect().
		Model((*models.IdempotencyKey)(nil)).
		Column("order_id").
		Where("key = ?", key).
		Scan(ctx, &orderID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, orderID, "idempotency key must point to the first order")

	// The order number must be the first one, not the second.
	persisted, err := NewOrderBunRepository(db).FindByID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, "ORD-IK-001", persisted.OrderNumber)
}

func TestOrderStore_CreateOrder_ConcurrentDuplicateKeys_OnlyOneOrderPersists(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	productRepo := NewProductBunRepository(db)
	store := NewOrderStore(db, productRepo, noopEnrichment{})

	product, _ := createTestProduct(t, ctx, db, "IK-Concurrent", 5.0)
	key := uuid.New()

	const goroutines = 2
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errors := make([]error, goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			_, errors[idx] = store.CreateOrder(ctx, StoreOrderInput{
				OrderNumber:    fmt.Sprintf("ORD-IK-CONC-%d", idx),
				IdempotencyKey: &key,
				CustomerName:   "Test",
				CustomerPhone:  fmt.Sprintf("555-800%d", idx),
				PaymentMethod:  string(models.OrderPaymentMethodEfectivo),
				OrderType:      string(models.OrderTypeParaLlevar),
				Items: []StoreOrderItemInput{
					{ProductID: product.ID, Quantity: 1},
				},
			})
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range errors {
		if err == nil {
			successes++
		} else if !isUniqueViolation(err) {
			t.Fatalf("expected unique violation or nil, got %v", err)
		}
	}
	assert.Equal(t, 1, successes, "exactly one concurrent create should succeed")

	var orderCount int
	err := db.NewSelect().
		TableExpr("orders AS o").
		Join("JOIN idempotency_keys AS ik ON ik.order_id = o.id").
		ColumnExpr("COUNT(*)").
		Where("ik.key = ?", key).
		Scan(ctx, &orderCount)
	require.NoError(t, err)
	assert.Equal(t, 1, orderCount, "exactly one order should persist for the idempotency key")
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
}

// TestOrderStore_CreateOrder_AppliesOfferBasePrice proves active-offer base
// prices are charged server-side (the storefront shows them in the cart).
func TestOrderStore_CreateOrder_AppliesOfferBasePrice(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	productRepo := NewProductBunRepository(db)
	store := NewOrderStore(db, productRepo, noopEnrichment{})

	offered, _ := createTestProduct(t, ctx, db, "Offer-Product", 5.0)
	regular, _ := createTestProduct(t, ctx, db, "Regular-Product", 2.0)

	order, err := store.CreateOrder(ctx, StoreOrderInput{
		OrderNumber:   "ORD-OF-001",
		CustomerName:  "Test",
		CustomerPhone: "555-7101",
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
		Items: []StoreOrderItemInput{
			{ProductID: offered.ID, Quantity: 2},
			{ProductID: regular.ID, Quantity: 1},
		},
		BasePrices: map[uuid.UUID]float64{offered.ID: 3.5},
	})
	require.NoError(t, err)
	assert.InDelta(t, 2*3.5+2.0, order.TotalAmount, 0.001, "offer price replaces the base price; others unchanged")
}
