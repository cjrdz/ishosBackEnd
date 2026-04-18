// category_repo.go — Ent-backed repository for category CRUD operations.
package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/category"
)

type CategoryRepository struct {
	db *ent.Client
}

func NewCategoryRepository(db *ent.Client) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) FindActive(ctx context.Context) ([]*ent.Category, error) {
	return r.db.Category.Query().
		Where(category.IsActiveEQ(true)).
		Order(ent.Asc(category.FieldDisplayOrder)).
		All(ctx)
}

func (r *CategoryRepository) FindAll(ctx context.Context) ([]*ent.Category, error) {
	return r.db.Category.Query().
		Order(ent.Asc(category.FieldDisplayOrder)).
		All(ctx)
}

func (r *CategoryRepository) FindBySlugWithProducts(ctx context.Context, slug string) (*ent.Category, error) {
	return r.db.Category.Query().
		Where(category.SlugEQ(slug)).
		WithProducts().
		Only(ctx)
}

func (r *CategoryRepository) Create(ctx context.Context, input CreateCategoryInput) (*ent.Category, error) {
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

	return create.Save(ctx)
}

func (r *CategoryRepository) Update(ctx context.Context, id uuid.UUID, input UpdateCategoryInput) (*ent.Category, error) {
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

	return upd.Save(ctx)
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.Category.DeleteOneID(id).Exec(ctx)
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
