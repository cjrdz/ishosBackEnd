// category_bun_test.go — Repository integration tests for the Bun-backed category adapter.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func TestCategoryBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCategoryBunRepository(db)

	created, err := repo.Create(ctx, CreateCategoryInput{
		Name:         "Ice Cream",
		Slug:         "ice-cream",
		Description:  "Frozen treats",
		DisplayOrder: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ice Cream", created.Name)
	assert.True(t, created.IsActive)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	bySlug, err := repo.FindBySlug(ctx, "ice-cream")
	require.NoError(t, err)
	assert.Equal(t, created.ID, bySlug.ID)

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
}

func TestCategoryBunRepository_FindActive(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCategoryBunRepository(db)

	activeCat, err := repo.Create(ctx, CreateCategoryInput{Name: "Active", Slug: "active", DisplayOrder: 1})
	require.NoError(t, err)
	inactiveCat, err := repo.Create(ctx, CreateCategoryInput{Name: "Inactive", Slug: "inactive", DisplayOrder: 2})
	require.NoError(t, err)
	isActive := false
	_, err = repo.Update(ctx, inactiveCat.ID, UpdateCategoryInput{IsActive: &isActive})
	require.NoError(t, err)

	active, err := repo.FindActive(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, activeCat.ID, active[0].ID)
}

func TestCategoryBunRepository_FindBySlugWithProducts(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCategoryBunRepository(db)

	cat, err := repo.Create(ctx, CreateCategoryInput{Name: "Shakes", Slug: "shakes", DisplayOrder: 1})
	require.NoError(t, err)

	product := models.Product{
		Name:        "Vanilla Shake",
		CategoryID:  cat.ID,
		Price:       3.5,
		IsAvailable: true,
	}
	_, err = db.NewInsert().Model(&product).Exec(ctx)
	require.NoError(t, err)

	found, err := repo.FindBySlugWithProducts(ctx, "shakes")
	require.NoError(t, err)
	require.Len(t, found.Products, 1)
	assert.Equal(t, "Vanilla Shake", found.Products[0].Name)
}

func TestCategoryBunRepository_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewCategoryBunRepository(db)

	created, err := repo.Create(ctx, CreateCategoryInput{Name: "Old", Slug: "old", DisplayOrder: 1})
	require.NoError(t, err)

	name := "New"
	updated, err := repo.Update(ctx, created.ID, UpdateCategoryInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)

	require.NoError(t, repo.Delete(ctx, created.ID))
	_, err = repo.FindByID(ctx, created.ID)
	require.Error(t, err)
}
