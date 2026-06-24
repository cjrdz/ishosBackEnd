// inventory_bun_test.go — Repository integration tests for the Bun-backed inventory adapter.
package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func TestInventoryBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewInventoryBunRepository(db)

	created, err := repo.Create(ctx, CreateInventoryItemInput{
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

	list, err := repo.List(ctx)
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

	created, err := repo.Create(ctx, CreateInventoryItemInput{Name: "Strawberry", Type: string(models.InventoryItemTypeBallBased)})
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

	created, err := repo.Create(ctx, CreateInventoryItemInput{Name: "Chocolate", Type: string(models.InventoryItemTypeBallBased), CurrentStock: 50})
	require.NoError(t, err)

	orderID := uuid.New()
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

	created, err := repo.Create(ctx, CreateInventoryItemInput{Name: "Mango", Type: string(models.InventoryItemTypeBallBased)})
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

	created, err := repo.Create(ctx, CreateInventoryItemInput{Name: "Spillage", Type: string(models.InventoryItemTypeBallBased), CurrentStock: 30})
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

	created, err := repo.Create(ctx, CreateInventoryItemInput{Name: "Tracked", Type: string(models.InventoryItemTypeBallBased)})
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
