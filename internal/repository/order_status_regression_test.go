// order_status_regression_test.go — Integration tests for order status/stock
// regressions found in the pre-launch review:
//   - status changes must not wipe amount_received
//   - rejecting an in-progress order must return its stock
//   - restoring stock is net-based, so repeated restores never over-restock
//   - replacing items on an in-progress order restores old stock before deducting new
//   - a stale expected status is rejected with a conflict (concurrent updates)
//
// They require a reachable PostgreSQL instance and skip otherwise.
package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func createRegressionOrder(t *testing.T, ctx context.Context, repo *OrderBunRepository, number, status string, amountReceived *float64) dto.Order {
	t.Helper()
	order, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:    number,
		CustomerName:   "Regression",
		CustomerPhone:  "555-" + number[len(number)-4:],
		TotalAmount:    10.0,
		Status:         status,
		PaymentMethod:  string(models.OrderPaymentMethodEfectivo),
		AmountReceived: amountReceived,
		OrderType:      string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)
	return order
}

func createRegressionInventoryItem(t *testing.T, ctx context.Context, repo *InventoryBunRepository, name string, stock int) dto.InventoryItem {
	t.Helper()
	item, err := repo.Create(ctx, CreateInventoryItemInput{
		Name:         name,
		Type:         string(models.InventoryItemTypeUnitBased),
		CurrentStock: stock,
	})
	require.NoError(t, err)
	return item
}

func currentStock(t *testing.T, ctx context.Context, repo *InventoryBunRepository, id uuid.UUID) int {
	t.Helper()
	item, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	return item.CurrentStock
}

func TestOrderStatusChanges_PreserveAmountReceived(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	received := 20.0
	order := createRegressionOrder(t, ctx, orderRepo, "ORD-RG-0001", string(models.OrderStatusPendienteRevision), &received)

	reviewer := "reviewer-1"
	_, err := orderRepo.Approve(ctx, order.ID, &reviewer, string(models.OrderStatusPendienteRevision))
	require.NoError(t, err)
	_, err = store.UpdateStatus(ctx, order.ID, string(models.OrderStatusEnProceso), string(models.OrderStatusRecibida))
	require.NoError(t, err)
	_, err = orderRepo.UpdateStatus(ctx, order.ID, string(models.OrderStatusLista), string(models.OrderStatusEnProceso))
	require.NoError(t, err)

	persisted, err := orderRepo.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusLista), persisted.Status)
	require.NotNil(t, persisted.AmountReceived, "amount_received must survive status changes")
	assert.InDelta(t, received, *persisted.AmountReceived, 0.001)
}

func TestOrderReject_InProgress_RestoresStock(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	order := createRegressionOrder(t, ctx, orderRepo, "ORD-RG-0002", string(models.OrderStatusRecibida), nil)
	item := createRegressionInventoryItem(t, ctx, invRepo, "Regression-Reject", 50)

	_, err := store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), string(models.OrderStatusRecibida),
		[]StockDeductionInput{{InventoryItemID: item.ID, Quantity: 5}}, false)
	require.NoError(t, err)
	require.Equal(t, 45, currentStock(t, ctx, invRepo, item.ID))

	reviewer := "reviewer-1"
	rejected, err := orderRepo.Reject(ctx, order.ID, "customer left", &reviewer, string(models.OrderStatusEnProceso))
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusCancelada), rejected.Status)
	assert.Equal(t, 50, currentStock(t, ctx, invRepo, item.ID), "rejecting an in-progress order must return its stock")
}

func TestRestoreStock_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)

	order := createRegressionOrder(t, ctx, orderRepo, "ORD-RG-0003", string(models.OrderStatusEnProceso), nil)
	item := createRegressionInventoryItem(t, ctx, invRepo, "Regression-Idempotent", 50)

	_, err := invRepo.DeductStockForOrder(ctx, order.ID, []StockDeductionInput{{InventoryItemID: item.ID, Quantity: 5}}, "")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err = invRepo.RestoreStockForOrder(ctx, order.ID, "")
		require.NoError(t, err)
	}
	assert.Equal(t, 50, currentStock(t, ctx, invRepo, item.ID), "repeated restores must not over-restock")

	// Deduct again (e.g. a force-approved order re-entering en_proceso), then restore once more.
	_, err = invRepo.DeductStockForOrder(ctx, order.ID, []StockDeductionInput{{InventoryItemID: item.ID, Quantity: 3}}, "")
	require.NoError(t, err)
	_, err = invRepo.RestoreStockForOrder(ctx, order.ID, "")
	require.NoError(t, err)
	assert.Equal(t, 50, currentStock(t, ctx, invRepo, item.ID))
}

func TestUpdateOrderWithStockAdjustment_RestoresBeforeDeducting(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	productRepo := NewProductBunRepository(db)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, productRepo, noopEnrichment{})

	product, _ := createTestProduct(t, ctx, db, "Regression-Edit-Product", 5.0)
	order := createRegressionOrder(t, ctx, orderRepo, "ORD-RG-0004", string(models.OrderStatusRecibida), nil)
	item := createRegressionInventoryItem(t, ctx, invRepo, "Regression-Edit", 50)

	_, err := store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), string(models.OrderStatusRecibida),
		[]StockDeductionInput{{InventoryItemID: item.ID, Quantity: 5}}, false)
	require.NoError(t, err)
	require.Equal(t, 45, currentStock(t, ctx, invRepo, item.ID))

	// Replace the items: the new items need 2 units. Old 5 must come back first.
	_, err = store.UpdateOrderWithStockAdjustment(ctx, order.ID, StoreUpdateOrderInput{
		CustomerName:   "Regression",
		CustomerPhone:  "555-0004",
		PaymentMethod:  string(models.OrderPaymentMethodEfectivo),
		OrderType:      string(models.OrderTypeParaLlevar),
		Items:          []StoreOrderItemInput{{ProductID: product.ID, Quantity: 1}},
		ExpectedStatus: string(models.OrderStatusEnProceso),
	}, []StockDeductionInput{{InventoryItemID: item.ID, Quantity: 2}}, true)
	require.NoError(t, err)
	assert.Equal(t, 48, currentStock(t, ctx, invRepo, item.ID), "edit must net to the new items' usage")

	// Cancelling afterwards returns exactly what is still held.
	reviewer := "reviewer-1"
	_, err = orderRepo.Reject(ctx, order.ID, "cancelled", &reviewer, string(models.OrderStatusEnProceso))
	require.NoError(t, err)
	assert.Equal(t, 50, currentStock(t, ctx, invRepo, item.ID))
}

func TestStatusUpdate_StaleExpectedStatus_Conflicts(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	order := createRegressionOrder(t, ctx, orderRepo, "ORD-RG-0005", string(models.OrderStatusRecibida), nil)
	item := createRegressionInventoryItem(t, ctx, invRepo, "Regression-Race", 50)
	deduct := []StockDeductionInput{{InventoryItemID: item.ID, Quantity: 5}}

	// Two devices both saw "recibida" and both try to start the order.
	_, err := store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), string(models.OrderStatusRecibida), deduct, false)
	require.NoError(t, err)
	_, err = store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), string(models.OrderStatusRecibida), deduct, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperror.ErrConflict), "stale status must surface as a conflict, got %v", err)

	assert.Equal(t, 45, currentStock(t, ctx, invRepo, item.ID), "stock must be deducted exactly once")
}

// TestStatusUpdate_ConcurrentTransitions_DeductOnce races several "start order"
// requests (two tablets, double taps). The row lock + expected-status check
// must let exactly one through and deduct stock exactly once.
func TestStatusUpdate_ConcurrentTransitions_DeductOnce(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	orderRepo := NewOrderBunRepository(db)
	invRepo := NewInventoryBunRepository(db)
	store := NewOrderStore(db, NewProductBunRepository(db), noopEnrichment{})

	order := createRegressionOrder(t, ctx, orderRepo, "ORD-RG-0006", string(models.OrderStatusRecibida), nil)
	item := createRegressionInventoryItem(t, ctx, invRepo, "Regression-Concurrent", 50)
	deduct := []StockDeductionInput{{InventoryItemID: item.ID, Quantity: 5}}

	const racers = 8
	start := make(chan struct{})
	errs := make(chan error, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := store.UpdateStatusWithStockAdjustment(ctx, order.ID, string(models.OrderStatusEnProceso), string(models.OrderStatusRecibida), deduct, false)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	succeeded, conflicts := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, apperror.ErrConflict):
			conflicts++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, succeeded, "exactly one transition may win")
	assert.Equal(t, racers-1, conflicts, "the rest must be rejected as conflicts")
	assert.Equal(t, 45, currentStock(t, ctx, invRepo, item.ID), "stock deducted exactly once")
}
