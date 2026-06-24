// customer_bun_test.go — Repository integration tests for the Bun-backed customer adapter.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func TestCustomerBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCustomerBunRepository(db)

	created, err := repo.Create(ctx, CreateCustomerInput{
		Name:     "Alice",
		UserType: "user",
		Phone:    "555-0001",
		Email:    strPtr("alice@example.com"),
		Status:   "active",
	})
	require.NoError(t, err)
	assert.Equal(t, "Alice", created.Name)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	byPhone, err := repo.FindByPhone(ctx, "555-0001")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byPhone.ID)
}

func TestCustomerBunRepository_ListAndSearch(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCustomerBunRepository(db)

	_, err := repo.Create(ctx, CreateCustomerInput{Name: "Bob", UserType: "user", Phone: "555-0002", Status: "active"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, CreateCustomerInput{Name: "Charlie", UserType: "user", Phone: "555-0003", Status: "inactive"})
	require.NoError(t, err)

	all, err := repo.List(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, all, 2)

	active, err := repo.List(ctx, "active", "")
	require.NoError(t, err)
	require.Len(t, active, 1)

	search, err := repo.List(ctx, "", "bob")
	require.NoError(t, err)
	require.Len(t, search, 1)

	paginated, total, err := repo.ListPaginated(ctx, "", "", 0, 10)
	require.NoError(t, err)
	require.Len(t, paginated, 2)
	assert.Equal(t, 2, total)
}

func TestCustomerBunRepository_Update(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCustomerBunRepository(db)

	created, err := repo.Create(ctx, CreateCustomerInput{Name: "Dave", UserType: "user", Phone: "555-0004", Status: "active"})
	require.NoError(t, err)

	name := "David"
	updated, err := repo.Update(ctx, created.ID, UpdateCustomerInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "David", updated.Name)
}

func TestCustomerBunRepository_Delete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCustomerBunRepository(db)

	created, err := repo.Create(ctx, CreateCustomerInput{Name: "Eve", UserType: "user", Phone: "555-0005", Status: "active"})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, created.ID))
	_, err = repo.FindByID(ctx, created.ID)
	require.Error(t, err)
}

func TestCustomerBunRepository_ListOrdersByCustomerID(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCustomerBunRepository(db)

	customer, err := repo.Create(ctx, CreateCustomerInput{Name: "Frank", UserType: "user", Phone: "555-0006", Status: "active"})
	require.NoError(t, err)

	category := models.Category{Name: "Test Category", Slug: "test-category"}
	_, err = db.NewInsert().Model(&category).Exec(ctx)
	require.NoError(t, err)

	product := models.Product{
		Name:        "Product A",
		Description: "Test product",
		Price:       10.0,
		CategoryID:  category.ID,
	}
	_, err = db.NewInsert().Model(&product).Exec(ctx)
	require.NoError(t, err)

	order := models.Order{
		OrderNumber:   "ORD-001",
		CustomerID:    &customer.ID,
		CustomerName:  customer.Name,
		CustomerPhone: customer.Phone,
		TotalAmount:   10.0,
		Status:        models.OrderStatusPendienteRevision,
		PaymentMethod: models.OrderPaymentMethodEfectivo,
		PaymentStatus: models.OrderPaymentStatusPendiente,
		OrderType:     models.OrderTypeParaLlevar,
	}
	_, err = db.NewInsert().Model(&order).Exec(ctx)
	require.NoError(t, err)

	item := models.OrderItem{
		ProductName: "Product A",
		Quantity:    1,
		UnitPrice:   10.0,
		Subtotal:    10.0,
		OrderID:     order.ID,
		ProductID:   product.ID,
	}
	_, err = db.NewInsert().Model(&item).Exec(ctx)
	require.NoError(t, err)

	orders, err := repo.ListOrdersByCustomerID(ctx, customer.ID, 10)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, "ORD-001", orders[0].OrderNumber)
	require.Len(t, orders[0].Items, 1)
	assert.Equal(t, "Product A", orders[0].Items[0].ProductName)
}
