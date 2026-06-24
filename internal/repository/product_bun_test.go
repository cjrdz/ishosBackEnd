// product_bun_test.go — Repository integration tests for the Bun-backed product adapter.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func TestProductBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewProductBunRepository(db)

	category := createTestCategory(t, ctx, db, "Cones", "cones")

	created, err := repo.Create(ctx, CreateProductInput{
		Name:        "Vanilla Cone",
		Description: "Classic vanilla",
		Price:       2.5,
		CategoryID:  category.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Vanilla Cone", created.Name)
	assert.Equal(t, category.ID, created.CategoryID)
	assert.NotNil(t, created.Category)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	withCategory, err := repo.FindByIDWithCategory(ctx, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, withCategory.Category)
	assert.Equal(t, "Cones", withCategory.Category.Name)
}

func TestProductBunRepository_FindAll(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewProductBunRepository(db)

	category := createTestCategory(t, ctx, db, "Shakes", "shakes")
	_, err := repo.Create(ctx, CreateProductInput{Name: "Chocolate Shake", Description: "Yum", Price: 4.0, CategoryID: category.ID})
	require.NoError(t, err)

	all, err := repo.FindAll(ctx, "", false)
	require.NoError(t, err)
	require.Len(t, all, 1)

	bySlug, err := repo.FindAll(ctx, "shakes", false)
	require.NoError(t, err)
	require.Len(t, bySlug, 1)

	missing, err := repo.FindAll(ctx, "no-such-slug", false)
	require.NoError(t, err)
	require.Len(t, missing, 0)
}

func TestProductBunRepository_UpdateAndToggleAndDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewProductBunRepository(db)

	category := createTestCategory(t, ctx, db, "Cups", "cups")
	created, err := repo.Create(ctx, CreateProductInput{Name: "Small Cup", Description: "Small", Price: 3.0, CategoryID: category.ID})
	require.NoError(t, err)

	name := "Medium Cup"
	updated, err := repo.Update(ctx, created.ID, UpdateProductInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Medium Cup", updated.Name)

	toggled, err := repo.ToggleAvailability(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, toggled.IsAvailable)

	require.NoError(t, repo.Delete(ctx, created.ID))
	_, err = repo.FindByID(ctx, created.ID)
	require.Error(t, err)
}

func createTestCategory(t *testing.T, ctx context.Context, db *bun.DB, name, slug string) models.Category {
	t.Helper()
	category := models.Category{Name: name, Slug: slug, Description: strPtr("")}
	_, err := db.NewInsert().Model(&category).Exec(ctx)
	require.NoError(t, err)
	return category
}
