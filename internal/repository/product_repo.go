// product_repo.go — Ent-backed repository for product CRUD and flavor/addon many-to-many relations.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entaddon "github.com/cjrdz/ishosBackEnd/ent/addon"
	"github.com/cjrdz/ishosBackEnd/ent/category"
	entflavor "github.com/cjrdz/ishosBackEnd/ent/flavor"
	"github.com/cjrdz/ishosBackEnd/ent/product"
)

type ProductRepository struct {
	db *ent.Client
}

func NewProductRepository(db *ent.Client) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]*ent.Product, error) {
	q := r.db.Product.Query().
		WithCategory().
		WithFlavors(func(flavorQuery *ent.FlavorQuery) {
			flavorQuery.Order(ent.Asc(entflavor.FieldDisplayOrder))
		}).
		WithAddons(func(addonQuery *ent.AddonQuery) {
			addonQuery.Order(ent.Asc(entaddon.FieldDisplayOrder))
		})

	if categorySlug != "" {
		q = q.Where(product.HasCategoryWith(category.SlugEQ(categorySlug)))
	}

	if onlyAvailable {
		q = q.Where(product.IsAvailableEQ(true))
	}

	q = q.Order(ent.Asc(product.FieldCreatedAt))

	return q.All(ctx)
}

func (r *ProductRepository) FindByIDWithCategory(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	return r.db.Product.Query().
		Where(product.IDEQ(id)).
		WithCategory().
		WithFlavors().
		WithAddons().
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
	if input.CategoryID != nil {
		upd = upd.SetCategoryID(*input.CategoryID)
	}
	if input.ImagePath != nil {
		upd = upd.SetImagePath(*input.ImagePath)
	}
	if input.IsAvailable != nil {
		upd = upd.SetIsAvailable(*input.IsAvailable)
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

// GetProductFlavors returns all flavors for a product
func (r *ProductRepository) GetProductFlavors(ctx context.Context, productID uuid.UUID) ([]*ent.Flavor, error) {
	return r.db.Flavor.Query().
		Where(entflavor.HasProductsWith(product.IDEQ(productID))).
		Order(ent.Asc(entflavor.FieldDisplayOrder)).
		All(ctx)
}

// GetProductAddons returns all addons for a product
func (r *ProductRepository) GetProductAddons(ctx context.Context, productID uuid.UUID) ([]*ent.Addon, error) {
	return r.db.Addon.Query().
		Where(entaddon.HasProductsWith(product.IDEQ(productID))).
		Order(ent.Asc(entaddon.FieldDisplayOrder)).
		All(ctx)
}

// LinkFlavorToProduct adds a flavor to a product
func (r *ProductRepository) LinkFlavorToProduct(ctx context.Context, productID, flavorID uuid.UUID) error {
	return r.db.Product.UpdateOneID(productID).
		AddFlavorIDs(flavorID).
		Exec(ctx)
}

// UnlinkFlavorFromProduct removes a flavor from a product
func (r *ProductRepository) UnlinkFlavorFromProduct(ctx context.Context, productID, flavorID uuid.UUID) error {
	return r.db.Product.UpdateOneID(productID).
		RemoveFlavorIDs(flavorID).
		Exec(ctx)
}

// LinkAddonToProduct adds an addon to a product
func (r *ProductRepository) LinkAddonToProduct(ctx context.Context, productID, addonID uuid.UUID) error {
	return r.db.Product.UpdateOneID(productID).
		AddAddonIDs(addonID).
		Exec(ctx)
}

// UnlinkAddonFromProduct removes an addon from a product
func (r *ProductRepository) UnlinkAddonFromProduct(ctx context.Context, productID, addonID uuid.UUID) error {
	return r.db.Product.UpdateOneID(productID).
		RemoveAddonIDs(addonID).
		Exec(ctx)
}

type CreateProductInput struct {
	Name        string
	Description string
	Price       float64
	CategoryID  uuid.UUID
	ImagePath   *string
}

type UpdateProductInput struct {
	Name        *string
	Description *string
	Price       *float64
	CategoryID  *uuid.UUID
	ImagePath   *string
	IsAvailable *bool
}
