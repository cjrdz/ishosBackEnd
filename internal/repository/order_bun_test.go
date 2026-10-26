// order_bun_test.go — Repository integration tests for the Bun-backed order adapter.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func createTestProduct(t *testing.T, ctx context.Context, db *bun.DB, name string, price float64) (models.Product, models.Category) {
	t.Helper()
	category := models.Category{
		ID:          uuid.New(),
		Name:        name + " Category",
		Slug:        name + "-category",
		Description: strPtr(""),
	}
	_, err := db.NewInsert().Model(&category).Exec(ctx)
	require.NoError(t, err)

	product := models.Product{
		ID:          uuid.New(),
		Name:        name,
		Description: "test",
		Price:       price,
		CategoryID:  category.ID,
		IsAvailable: true,
	}
	_, err = db.NewInsert().Model(&product).Exec(ctx)
	require.NoError(t, err)
	return product, category
}

func TestOrderBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db)

	created, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-001",
		CustomerName:  "Alice",
		CustomerPhone: "555-0001",
		TotalAmount:   10.0,
		Status:        string(models.OrderStatusPendienteRevision),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)
	assert.Equal(t, "ORD-001", created.OrderNumber)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	withItems, err := repo.FindByIDWithItems(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, withItems.ID)

	byNumber, err := repo.FindByOrderNumber(ctx, "ORD-001")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byNumber.ID)
}

func TestOrderBunRepository_CreateItem(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db)

	order, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-002",
		CustomerName:  "Bob",
		CustomerPhone: "555-0002",
		TotalAmount:   5.0,
		Status:        string(models.OrderStatusPendienteRevision),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	product, _ := createTestProduct(t, ctx, db, "Cone", 2.5)
	item, err := repo.CreateItem(ctx, CreateOrderItemInput{
		OrderID:     order.ID,
		ProductID:   product.ID,
		ProductName: product.Name,
		Quantity:    2,
		UnitPrice:   2.5,
		Subtotal:    5.0,
	})
	require.NoError(t, err)
	assert.Equal(t, product.Name, item.ProductName)

	withItems, err := repo.FindByIDWithItems(ctx, order.ID)
	require.NoError(t, err)
	require.Len(t, withItems.Items, 1)
	assert.Equal(t, product.Name, withItems.Items[0].ProductName)
}

func TestOrderBunRepository_ApproveAndReject(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db)

	created, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-003",
		CustomerName:  "Carol",
		CustomerPhone: "555-0003",
		TotalAmount:   8.0,
		Status:        string(models.OrderStatusPendienteRevision),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	reviewer := "admin"
	approved, err := repo.Approve(ctx, created.ID, &reviewer, "")
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusRecibida), approved.Status)
	assert.Equal(t, string(models.OrderPaymentStatusConfirmado), approved.PaymentStatus)
	assert.Equal(t, "admin", *approved.ReviewedByUserID)

	rejected, err := repo.Reject(ctx, approved.ID, "out of stock", &reviewer, "")
	require.NoError(t, err)
	assert.Equal(t, string(models.OrderStatusCancelada), rejected.Status)
	assert.Equal(t, "out of stock", *rejected.RejectionReason)
}

func TestOrderBunRepository_ListPaginated(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db)

	_, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-004",
		CustomerName:  "Dave",
		CustomerPhone: "555-0004",
		TotalAmount:   1.0,
		Status:        string(models.OrderStatusPendienteRevision),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	orders, total, err := repo.ListPaginated(ctx, "", false, 1, 10)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, 1, total)
}

func TestOrderBunRepository_Delete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db)

	created, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-005",
		CustomerName:  "Eve",
		CustomerPhone: "555-0005",
		TotalAmount:   3.0,
		Status:        string(models.OrderStatusPendienteRevision),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, created.ID))
	_, err = repo.FindByID(ctx, created.ID)
	require.Error(t, err)
}

func TestOrderBunRepository_FindByDateRange(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db)

	created, err := repo.Create(ctx, CreateOrderInput{
		OrderNumber:   "ORD-006",
		CustomerName:  "Frank",
		CustomerPhone: "555-0006",
		TotalAmount:   4.0,
		Status:        string(models.OrderStatusPendienteRevision),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)

	orders, err := repo.FindByDateRange(ctx, created.CreatedAt.Add(-time.Hour), created.CreatedAt.Add(time.Hour), nil)
	require.NoError(t, err)
	require.Len(t, orders, 1)
}
