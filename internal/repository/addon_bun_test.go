// addon_bun_test.go — Repository integration tests for the Bun-backed addon adapter.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddonBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAddonBunRepository(db)

	created, err := repo.Create(ctx, CreateAddonInput{
		Name:         "Sprinkles",
		Price:        0.5,
		GroupName:    "toppings",
		DisplayOrder: 1,
		IsActive:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Sprinkles", created.Name)
	assert.Equal(t, 0.5, created.Price)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	all, err := repo.FindAll(ctx, false)
	require.NoError(t, err)
	require.Len(t, all, 1)
}

func TestAddonBunRepository_Update(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAddonBunRepository(db)

	created, err := repo.Create(ctx, CreateAddonInput{Name: "Nuts", Price: 0.75, GroupName: "toppings", IsActive: true})
	require.NoError(t, err)

	price := 0.99
	updated, err := repo.Update(ctx, created.ID, UpdateAddonInput{Price: &price})
	require.NoError(t, err)
	assert.Equal(t, 0.99, updated.Price)
	assert.Equal(t, "Nuts", updated.Name)
}

func TestAddonBunRepository_Delete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAddonBunRepository(db)

	created, err := repo.Create(ctx, CreateAddonInput{Name: "Cherry", Price: 0.25, GroupName: "toppings", IsActive: true})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, created.ID))

	_, err = repo.FindByID(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
