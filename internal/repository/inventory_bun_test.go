// inventory_bun_test.go — Repository integration tests for the Bun-backed inventory adapter.
package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/models"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

func TestInventoryBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	created, err := repo.Create(ctx, CreateInventoryItemInput{
		LocationID:        testLocationID(t, db),
		Name:              "Vanilla Base",
		Type:              string(models.InventoryItemTypeBallBased),
		CurrentStock:      100,
		LowStockThreshold: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, "Vanilla Base", created.Name)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	list, err := repo.List(ctx, scope.All())
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestInventoryBunRepository_LinkAndUnlink(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	flavor := models.Flavor{Name: "Strawberry", IsActive: true}
	_, err := db.NewInsert().Model(&flavor).Exec(ctx)
	require.NoError(t, err)

	created, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: testLocationID(t, db), Name: "Strawberry", Type: string(models.InventoryItemTypeBallBased)})
	require.NoError(t, err)

	linked, err := repo.LinkToFlavor(ctx, created.ID, flavor.ID)
	require.NoError(t, err)
	require.NotNil(t, linked.FlavorID)
	assert.Equal(t, flavor.ID, *linked.FlavorID)

	unlinked, err := repo.UnlinkFromFlavor(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, unlinked.FlavorID)
}

func TestInventoryBunRepository_DeductAndRestoreStock(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	created, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: testLocationID(t, db), Name: "Chocolate", Type: string(models.InventoryItemTypeBallBased), CurrentStock: 50})
	require.NoError(t, err)

	orderID := createTestOrderAt(t, db, testLocationID(t, db))
	movements, err := repo.DeductStockForOrder(ctx, orderID, []StockDeductionInput{
		{InventoryItemID: created.ID, Quantity: 5},
	}, "user-1")
	require.NoError(t, err)
	require.Len(t, movements, 1)
	assert.Equal(t, -5, movements[0].Quantity)

	item, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, item.CurrentStock)

	restored, err := repo.RestoreStockForOrder(ctx, orderID, "user-1")
	require.NoError(t, err)
	require.Len(t, restored, 1)

	item, err = repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 50, item.CurrentStock)
}

func TestInventoryBunRepository_RecordEntry(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	created, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: testLocationID(t, db), Name: "Mango", Type: string(models.InventoryItemTypeBallBased)})
	require.NoError(t, err)

	containerType := models.ContainerType{Name: "Tub", BallsPerContainer: 10}
	_, err = db.NewInsert().Model(&containerType).Exec(ctx)
	require.NoError(t, err)

	movement, err := repo.RecordEntry(ctx, RecordEntryInput{
		InventoryItemID:    created.ID,
		ContainerTypeID:    containerType.ID,
		QuantityContainers: 2,
		TotalBalls:         20,
		CreatedByID:        "user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, 20, movement.Quantity)

	item, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 20, item.CurrentStock)
}

func TestInventoryBunRepository_RecordAdjustment(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	created, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: testLocationID(t, db), Name: "Spillage", Type: string(models.InventoryItemTypeBallBased), CurrentStock: 30})
	require.NoError(t, err)

	movement, err := repo.RecordAdjustment(ctx, RecordAdjustmentInput{
		InventoryItemID: created.ID,
		Quantity:        -5,
		Reason:          "spilled",
		CreatedByID:     "user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, -5, movement.Quantity)

	item, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 25, item.CurrentStock)
}

func TestInventoryBunRepository_FindStockMovements(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	created, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: testLocationID(t, db), Name: "Tracked", Type: string(models.InventoryItemTypeBallBased)})
	require.NoError(t, err)

	containerType := models.ContainerType{Name: "Bucket", BallsPerContainer: 10}
	_, err = db.NewInsert().Model(&containerType).Exec(ctx)
	require.NoError(t, err)

	_, err = repo.RecordEntry(ctx, RecordEntryInput{
		InventoryItemID:    created.ID,
		ContainerTypeID:    containerType.ID,
		QuantityContainers: 1,
		TotalBalls:         10,
		CreatedByID:        "user-1",
	})
	require.NoError(t, err)

	movements, total, err := repo.FindStockMovementsByInventoryItem(ctx, created.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, movements, 1)
	assert.Equal(t, 1, total)
}

// createTestOrderAt inserts a bare order at a location and returns its ID.
func createTestOrderAt(t *testing.T, db bun.IDB, locationID uuid.UUID) uuid.UUID {
	t.Helper()
	order, err := NewOrderBunRepository(db).Create(context.Background(), CreateOrderInput{
		LocationID:    locationID,
		OrderNumber:   "ORD-" + uuid.NewString()[:8],
		CustomerName:  "Prueba",
		CustomerPhone: "70000000",
		TotalAmount:   1,
		Status:        string(models.OrderStatusEnProceso),
		PaymentMethod: string(models.OrderPaymentMethodEfectivo),
		OrderType:     string(models.OrderTypeParaLlevar),
	})
	require.NoError(t, err)
	return order.ID
}

func TestInventoryBunRepository_StockIsPerLocation(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)
	s1, s2 := testLocationBySlug(t, db, "sucursal-1"), testLocationBySlug(t, db, "sucursal-2")

	flavorID := uuid.New()
	_, err := db.NewInsert().Model(&models.Flavor{ID: flavorID, Name: "Fresa"}).Exec(ctx)
	require.NoError(t, err)
	itemS1, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: s1, Name: "Fresa", Type: string(models.InventoryItemTypeBallBased), FlavorID: &flavorID, CurrentStock: 10})
	require.NoError(t, err)
	itemS2, err := repo.Create(ctx, CreateInventoryItemInput{LocationID: s2, Name: "Fresa", Type: string(models.InventoryItemTypeBallBased), FlavorID: &flavorID, CurrentStock: 3})
	require.NoError(t, err, "the same flavour is tracked separately at each location")

	found, err := repo.FindByFlavorID(ctx, s2, flavorID)
	require.NoError(t, err)
	assert.Equal(t, itemS2.ID, found.ID)

	// An order at Sucursal 2 must never consume Sucursal 1's stock.
	orderS2 := createTestOrderAt(t, db, s2)
	_, err = repo.DeductStockForOrder(ctx, orderS2, []StockDeductionInput{{InventoryItemID: itemS1.ID, Quantity: 1}}, "")
	require.ErrorIs(t, err, apperror.ErrConflict)

	_, err = repo.DeductStockForOrder(ctx, orderS2, []StockDeductionInput{{InventoryItemID: itemS2.ID, FlavorID: &flavorID, Quantity: 2}}, "")
	require.NoError(t, err)
	assert.Equal(t, 10, currentStock(t, ctx, repo, itemS1.ID), "Sucursal 1 untouched")
	assert.Equal(t, 1, currentStock(t, ctx, repo, itemS2.ID))

	s1Items, err := repo.List(ctx, scope.Only(s1))
	require.NoError(t, err)
	require.Len(t, s1Items, 1)
	assert.Equal(t, s1, s1Items[0].LocationID)
	all, err := repo.List(ctx, scope.All())
	require.NoError(t, err)
	assert.Len(t, all, 2)
}
