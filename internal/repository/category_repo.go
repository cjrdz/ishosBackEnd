// category_repo.go — Ent-backed repository for category CRUD operations.
package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/category"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type CategoryRepository struct {
	db *ent.Client
}

func NewCategoryRepository(db *ent.Client) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) FindActive(ctx context.Context) ([]dto.Category, error) {
	rows, err := r.db.Category.Query().
		Where(category.IsActiveEQ(true)).
		Order(ent.Asc(category.FieldDisplayOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return toCategoryDTOs(rows), nil
}

func (r *CategoryRepository) FindAll(ctx context.Context) ([]dto.Category, error) {
	rows, err := r.db.Category.Query().
		Order(ent.Asc(category.FieldDisplayOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return toCategoryDTOs(rows), nil
}

func (r *CategoryRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Category, error) {
	row, err := r.db.Category.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Category{}, &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return dto.Category{}, err
	}
	return toCategoryDTO(row), nil
}

func (r *CategoryRepository) FindBySlug(ctx context.Context, slug string) (dto.Category, error) {
	row, err := r.db.Category.Query().
		Where(category.SlugEQ(slug)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Category{}, &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return dto.Category{}, err
	}
	return toCategoryDTO(row), nil
}

func (r *CategoryRepository) FindBySlugWithProducts(ctx context.Context, slug string) (dto.Category, error) {
	row, err := r.db.Category.Query().
		Where(category.SlugEQ(slug)).
		WithProducts().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Category{}, &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return dto.Category{}, err
	}
	return toCategoryDTO(row), nil
}

func (r *CategoryRepository) Create(ctx context.Context, input CreateCategoryInput) (dto.Category, error) {
	now := time.Now()
	normalizedImagePath, _ := normalizeCategoryImagePath(input.ImagePath)

	create := r.db.Category.Create().
		SetName(input.Name).
		SetSlug(input.Slug).
		SetDescription(input.Description).
		SetDisplayOrder(input.DisplayOrder).
		SetIsActive(true).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	create = create.SetNillableImagePath(normalizedImagePath)

	row, err := create.Save(ctx)
	if err != nil {
		return dto.Category{}, err
	}
	return toCategoryDTO(row), nil
}

func (r *CategoryRepository) Update(ctx context.Context, id uuid.UUID, input UpdateCategoryInput) (dto.Category, error) {
	upd := r.db.Category.UpdateOneID(id).SetUpdatedAt(time.Now())

	if input.Name != nil {
		upd = upd.SetName(*input.Name)
	}
	if input.Slug != nil {
		upd = upd.SetSlug(*input.Slug)
	}
	if input.Description != nil {
		upd = upd.SetDescription(*input.Description)
	}
	if input.ImagePath != nil {
		normalizedImagePath, clearImagePath := normalizeCategoryImagePath(input.ImagePath)
		if clearImagePath {
			upd = upd.ClearImagePath()
		} else {
			upd = upd.SetImagePath(*normalizedImagePath)
		}
	}
	if input.DisplayOrder != nil {
		upd = upd.SetDisplayOrder(*input.DisplayOrder)
	}
	if input.IsActive != nil {
		upd = upd.SetIsActive(*input.IsActive)
	}

	row, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Category{}, &apperror.NotFoundError{Resource: "category", Cause: err}
		}
		return dto.Category{}, err
	}
	return toCategoryDTO(row), nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.Category.DeleteOneID(id).Exec(ctx)
	if err != nil && ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "category", Cause: err}
	}
	return err
}

func toCategoryDTOs(rows []*ent.Category) []dto.Category {
	result := make([]dto.Category, len(rows))
	for i, row := range rows {
		result[i] = toCategoryDTO(row)
	}
	return result
}

func toCategoryDTO(c *ent.Category) dto.Category {
	if c == nil {
		return dto.Category{}
	}
	cat := dto.Category{
		ID:           c.ID,
		Name:         c.Name,
		Slug:         c.Slug,
		Description:  c.Description,
		ImagePath:    c.ImagePath,
		DisplayOrder: c.DisplayOrder,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
	if c.Edges.Products != nil {
		cat.Products = make([]dto.Product, len(c.Edges.Products))
		for i, p := range c.Edges.Products {
			cat.Products[i] = toProductSummaryDTO(p)
		}
	}
	return cat
}

func toProductSummaryDTO(p *ent.Product) dto.Product {
	var catID uuid.UUID
	if p.Edges.Category != nil {
		catID = p.Edges.Category.ID
	}
	return dto.Product{
		ID:                   p.ID,
		Name:                 p.Name,
		Description:          p.Description,
		Price:                p.Price,
		CategoryID:           catID,
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

type CreateCategoryInput struct {
	Name         string
	Slug         string
	Description  string
	ImagePath    *string
	DisplayOrder int
}

type UpdateCategoryInput struct {
	Name         *string
	Slug         *string
	Description  *string
	ImagePath    *string
	DisplayOrder *int
	IsActive     *bool
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
