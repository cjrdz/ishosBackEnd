// category_bun.go — Bun-backed repository for category CRUD.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// CategoryBunRepository implements services.CategoryRepository using Bun.
type CategoryBunRepository struct {
	db bun.IDB
}

// NewCategoryBunRepository creates a new Bun-backed category repository.
func NewCategoryBunRepository(db bun.IDB) *CategoryBunRepository {
	return &CategoryBunRepository{db: db}
}

// FindActive returns all active categories ordered by display_order.
func (r *CategoryBunRepository) FindActive(ctx context.Context) ([]dto.Category, error) {
	var rows []models.Category
	if err := r.db.NewSelect().Model(&rows).Where("is_active = ?", true).Order("display_order ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return toCategoryDTOsFromModels(rows), nil
}

// FindAll returns all categories ordered by display_order.
func (r *CategoryBunRepository) FindAll(ctx context.Context) ([]dto.Category, error) {
	var rows []models.Category
	if err := r.db.NewSelect().Model(&rows).Order("display_order ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return toCategoryDTOsFromModels(rows), nil
}

// FindByID returns a category by ID.
func (r *CategoryBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Category, error) {
	var row models.Category
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Category{}, &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return dto.Category{}, err
	}
	return toCategoryDTOFromModel(row), nil
}

// FindBySlug returns a category by slug.
func (r *CategoryBunRepository) FindBySlug(ctx context.Context, slug string) (dto.Category, error) {
	var row models.Category
	if err := r.db.NewSelect().Model(&row).Where("slug = ?", slug).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Category{}, &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return dto.Category{}, err
	}
	return toCategoryDTOFromModel(row), nil
}

// FindBySlugWithProducts returns a category by slug including its products.
func (r *CategoryBunRepository) FindBySlugWithProducts(ctx context.Context, slug string) (dto.Category, error) {
	category, err := r.FindBySlug(ctx, slug)
	if err != nil {
		return dto.Category{}, err
	}

	var products []models.Product
	if err := r.db.NewSelect().Model(&products).Where("category_products = ?", category.ID).Scan(ctx); err != nil {
		return dto.Category{}, err
	}

	category.Products = make([]dto.Product, len(products))
	for i, p := range products {
		category.Products[i] = toProductSummaryFromModel(p, category.ID)
	}
	return category, nil
}

// Create creates a new category.
func (r *CategoryBunRepository) Create(ctx context.Context, input CreateCategoryInput) (dto.Category, error) {
	now := time.Now()
	normalizedImagePath, _ := normalizeCategoryImagePath(input.ImagePath)

	row := models.Category{
		ID:           uuid.New(),
		Name:         input.Name,
		Slug:         input.Slug,
		Description:  &input.Description,
		ImagePath:    normalizedImagePath,
		DisplayOrder: input.DisplayOrder,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := r.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return dto.Category{}, err
	}
	return toCategoryDTOFromModel(row), nil
}

// Update updates an existing category.
func (r *CategoryBunRepository) Update(ctx context.Context, id uuid.UUID, input UpdateCategoryInput) (dto.Category, error) {
	row, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Category{}, err
	}

	model := models.Category{
		ID:           row.ID,
		Name:         row.Name,
		Slug:         row.Slug,
		Description:  &row.Description,
		ImagePath:    row.ImagePath,
		DisplayOrder: row.DisplayOrder,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    time.Now(),
	}

	if input.Name != nil {
		model.Name = *input.Name
	}
	if input.Slug != nil {
		model.Slug = *input.Slug
	}
	if input.Description != nil {
		model.Description = input.Description
	}
	if input.ImagePath != nil {
		normalizedImagePath, clearImagePath := normalizeCategoryImagePath(input.ImagePath)
		if clearImagePath {
			model.ImagePath = nil
		} else {
			model.ImagePath = normalizedImagePath
		}
	}
	if input.DisplayOrder != nil {
		model.DisplayOrder = *input.DisplayOrder
	}
	if input.IsActive != nil {
		model.IsActive = *input.IsActive
	}

	res, err := r.db.NewUpdate().Model(&model).WherePK().Exec(ctx)
	if err != nil {
		return dto.Category{}, err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Category{}, &apperror.NotFoundError{Resource: "category"}
	}

	return toCategoryDTOFromModel(model), nil
}

// Delete deletes a category by ID.
func (r *CategoryBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	model := models.Category{ID: id}
	res, err := r.db.NewDelete().Model(&model).WherePK().Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return &apperror.NotFoundError{Resource: "category"}
	}
	return nil
}

func toProductSummaryFromModel(p models.Product, categoryID uuid.UUID) dto.Product {
	return dto.Product{
		ID:                   p.ID,
		Name:                 p.Name,
		Description:          p.Description,
		Price:                p.Price,
		CategoryID:           categoryID,
		ImagePath:            p.ImagePath,
		IsAvailable:          p.IsAvailable,
		BallBased:            p.BallBased,
		BallQuantity:         p.BallQuantity,
		AllowsMixedFlavors:   p.AllowsMixedFlavors,
		ExcludeGlobalFlavors: p.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  p.ExcludeGlobalAddons,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
}

func toCategoryDTOsFromModels(rows []models.Category) []dto.Category {
	result := make([]dto.Category, len(rows))
	for i, row := range rows {
		result[i] = toCategoryDTOFromModel(row)
	}
	return result
}

func toCategoryDTOFromModel(c models.Category) dto.Category {
	desc := ""
	if c.Description != nil {
		desc = *c.Description
	}
	return dto.Category{
		ID:           c.ID,
		Name:         c.Name,
		Slug:         c.Slug,
		Description:  desc,
		ImagePath:    c.ImagePath,
		DisplayOrder: c.DisplayOrder,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

func normalizeCategoryImagePath(imagePath *string) (*string, bool) {
	if imagePath == nil {
		return nil, false
	}

	normalized := strings.TrimSpace(*imagePath)
	if normalized == "" || strings.EqualFold(normalized, "null") {
		return nil, true
	}

	return &normalized, false
}
