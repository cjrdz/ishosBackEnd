// product_bun_test.go — Repository integration tests for the Bun-backed product adapter.
package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
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

func TestProductBunRepository_CreateDefaultsToOneFreeTopping(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewProductBunRepository(db)
	category := createTestCategory(t, ctx, db, "Bowls", "bowls")

	// Not specified (e.g. the seed): the first topping is included.
	created, err := repo.Create(ctx, CreateProductInput{Name: "Bowl", Price: 4.5, CategoryID: category.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, created.FreeToppings)

	// An explicit choice, including zero, is kept.
	zero := 0
	paid, err := repo.Create(ctx, CreateProductInput{Name: "Bowl sin topping", Price: 4.5, CategoryID: category.ID, FreeToppings: &zero})
	require.NoError(t, err)
	assert.Equal(t, 0, paid.FreeToppings)
}

func TestProductBunRepository_UpdateKeepsFreeToppings(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewProductBunRepository(db)
	category := createTestCategory(t, ctx, db, "Conos", "conos")

	two := 2
	created, err := repo.Create(ctx, CreateProductInput{Name: "Cono", Price: 0.9, CategoryID: category.ID, FreeToppings: &two})
	require.NoError(t, err)

	newPrice := 1.0
	updated, err := repo.Update(ctx, created.ID, UpdateProductInput{Price: &newPrice})
	require.NoError(t, err)
	assert.Equal(t, 2, updated.FreeToppings, "editing another field must not reset free toppings")

	zero := 0
	updated, err = repo.Update(ctx, created.ID, UpdateProductInput{FreeToppings: &zero})
	require.NoError(t, err)
	assert.Equal(t, 0, updated.FreeToppings)
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
	category := models.Category{ID: uuid.New(), Name: name, Slug: slug, Description: strPtr("")}
	_, err := db.NewInsert().Model(&category).Exec(ctx)
	require.NoError(t, err)
	return category
}
