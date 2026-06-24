// container_type_bun_test.go — Repository integration tests for the Bun-backed container type adapter.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerTypeBunRepository_CreateAndList(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewContainerTypeBunRepository(db)

	created, err := repo.Create(ctx, "Single", 1, false)
	require.NoError(t, err)
	assert.Equal(t, "Single", created.Name)
	assert.Equal(t, 1, created.BallsPerContainer)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, created.ID, list[0].ID)

	found, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestContainerTypeBunRepository_Update(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewContainerTypeBunRepository(db)

	created, err := repo.Create(ctx, "Double", 2, false)
	require.NoError(t, err)

	name := "Trio"
	balls := 3
	updated, err := repo.Update(ctx, created.ID, &name, &balls, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Trio", updated.Name)
	assert.Equal(t, 3, updated.BallsPerContainer)
	assert.False(t, updated.IsCustom)
}

func TestContainerTypeBunRepository_Delete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewContainerTypeBunRepository(db)

	created, err := repo.Create(ctx, "Custom", 5, true)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, created.ID))

	_, err = repo.GetByID(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
