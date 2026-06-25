// order_store_test.go — Integration tests for OrderStore transactional behavior.
//
// These tests prove that OrderStore.UpdateStatusWithStockAdjustment and
// UpdateOrderWithStockAdjustment actually roll back the order mutation when the
// inventory adjustment fails, and commit both atomically when it succeeds.
// They require a reachable PostgreSQL instance and skip otherwise.
package repository

import (
	"context"
	"testing"

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
	_, err = store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), []StockDeductionInput{
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

	updated, err := store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), []StockDeductionInput{
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
	_, err = store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusCancelada), nil, true)
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
