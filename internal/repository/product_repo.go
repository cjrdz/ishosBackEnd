package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosFactory-backend/ent"
	"github.com/cjrdz/ishosFactory-backend/ent/category"
	"github.com/cjrdz/ishosFactory-backend/ent/product"
)

type ProductRepository struct {
	db *ent.Client
}

func NewProductRepository(db *ent.Client) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]*ent.Product, error) {
	q := r.db.Product.Query().WithCategory()

	if categorySlug != "" {
		q = q.Where(product.HasCategoryWith(category.SlugEQ(categorySlug)))
	}

	if onlyAvailable {
		q = q.Where(
			product.IsAvailableEQ(true),
			product.StockStatusEQ(product.StockStatusInStock),
		)
	}

	return q.All(ctx)
}

func (r *ProductRepository) FindByIDWithCategory(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	return r.db.Product.Query().
		Where(product.IDEQ(id)).
		WithCategory().
		Only(ctx)
}

func (r *ProductRepository) FindByID(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	return r.db.Product.Get(ctx, id)
}

func (r *ProductRepository) Create(ctx context.Context, input CreateProductInput) (*ent.Product, error) {
	now := time.Now()
	return r.db.Product.Create().
		SetName(input.Name).
		SetDescription(input.Description).
		SetPrice(input.Price).
		SetCategoryID(input.CategoryID).
		SetNillableImagePath(input.ImagePath).
		SetStockStatus(product.StockStatus(input.StockStatus)).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
}

func (r *ProductRepository) Update(ctx context.Context, id uuid.UUID, input UpdateProductInput) (*ent.Product, error) {
	upd := r.db.Product.UpdateOneID(id).SetUpdatedAt(time.Now())

	if input.Name != nil {
		upd = upd.SetName(*input.Name)
	}
	if input.Description != nil {
		upd = upd.SetDescription(*input.Description)
	}
	if input.Price != nil {
		upd = upd.SetPrice(*input.Price)
	}
	if input.ImagePath != nil {
		upd = upd.SetImagePath(*input.ImagePath)
	}
	if input.IsAvailable != nil {
		upd = upd.SetIsAvailable(*input.IsAvailable)
	}
	if input.StockStatus != nil {
		upd = upd.SetStockStatus(product.StockStatus(*input.StockStatus))
	}

	return upd.Save(ctx)
}

func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.Product.DeleteOneID(id).Exec(ctx)
}

func (r *ProductRepository) ToggleAvailability(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	current, err := r.db.Product.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return r.db.Product.UpdateOneID(id).
		SetIsAvailable(!current.IsAvailable).
		SetUpdatedAt(time.Now()).
		Save(ctx)
}

type CreateProductInput struct {
	Name        string
	Description string
	Price       float64
	CategoryID  uuid.UUID
	ImagePath   *string
	StockStatus string
}

type UpdateProductInput struct {
	Name        *string
	Description *string
	Price       *float64
	ImagePath   *string
	IsAvailable *bool
	StockStatus *string
}
