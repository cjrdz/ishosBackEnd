// product_repo.go — Ent-backed repository for product CRUD and flavor/addon many-to-many relations.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/addon"
	"github.com/cjrdz/ishosBackEnd/ent/category"
	"github.com/cjrdz/ishosBackEnd/ent/flavor"
	"github.com/cjrdz/ishosBackEnd/ent/product"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type ProductRepository struct {
	db *ent.Client
}

func NewProductRepository(db *ent.Client) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]dto.Product, error) {
	q := r.db.Product.Query().
		WithCategory().
		Order(ent.Asc(product.FieldCreatedAt))

	if categorySlug != "" {
		q = q.Where(product.HasCategoryWith(category.SlugEQ(categorySlug)))
	}

	if onlyAvailable {
		q = q.Where(product.IsAvailableEQ(true))
	}

	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.Product, len(rows))
	for i, row := range rows {
		result[i] = toProductDTO(row)
	}

	// Public/admin product lists need flavors/addons so the UI can render
	// customization choices and the checkout can send topping/jalea selections.
	if err := r.attachEffectiveGlobalOptions(ctx, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *ProductRepository) FindByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	row, err := r.db.Product.Query().
		Where(product.IDEQ(id)).
		WithCategory().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Product{}, &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return dto.Product{}, err
	}

	result := toProductDTO(row)
	results := []dto.Product{result}
	if err := r.attachEffectiveGlobalOptions(ctx, results); err != nil {
		return dto.Product{}, err
	}

	return results[0], nil
}

func (r *ProductRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	row, err := r.db.Product.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Product{}, &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return dto.Product{}, err
	}

	return toProductDTO(row), nil
}

// FindByIDWithCategoryAndOptions returns a product with global flavors and addons attached.
// This is intended for order enrichment, where the full set of valid customizations is required.
func (r *ProductRepository) FindByIDWithCategoryAndOptions(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	row, err := r.db.Product.Query().
		Where(product.IDEQ(id)).
		WithCategory().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Product{}, &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return dto.Product{}, err
	}

	result := toProductDTO(row)
	results := []dto.Product{result}
	if err := r.attachEffectiveGlobalOptions(ctx, results); err != nil {
		return dto.Product{}, err
	}

	return results[0], nil
}

func (r *ProductRepository) Create(ctx context.Context, input CreateProductInput) (dto.Product, error) {
	now := time.Now()

	b := r.db.Product.Create().
		SetName(input.Name).
		SetDescription(input.Description).
		SetPrice(input.Price).
		SetCategoryID(input.CategoryID).
		SetExcludeGlobalFlavors(input.ExcludeGlobalFlavors).
		SetExcludeGlobalAddons(input.ExcludeGlobalAddons).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	if input.ImagePath != nil {
		b = b.SetImagePath(*input.ImagePath)
	}
	if input.BallBased != nil {
		b = b.SetBallBased(*input.BallBased)
	}
	if input.BallQuantity != nil {
		b = b.SetBallQuantity(*input.BallQuantity)
	}
	if input.AllowsMixedFlavors != nil {
		b = b.SetAllowsMixedFlavors(*input.AllowsMixedFlavors)
	}

	created, err := b.Save(ctx)
	if err != nil {
		return dto.Product{}, err
	}

	return r.FindByIDWithCategory(ctx, created.ID)
}

func (r *ProductRepository) Update(ctx context.Context, id uuid.UUID, input UpdateProductInput) (dto.Product, error) {
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
	if input.ExcludeGlobalFlavors != nil {
		upd = upd.SetExcludeGlobalFlavors(*input.ExcludeGlobalFlavors)
	}
	if input.ExcludeGlobalAddons != nil {
		upd = upd.SetExcludeGlobalAddons(*input.ExcludeGlobalAddons)
	}
	if input.BallBased != nil {
		upd = upd.SetBallBased(*input.BallBased)
	}
	if input.BallQuantity != nil {
		upd = upd.SetBallQuantity(*input.BallQuantity)
	}
	if input.AllowsMixedFlavors != nil {
		upd = upd.SetAllowsMixedFlavors(*input.AllowsMixedFlavors)
	}

	updated, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Product{}, &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return dto.Product{}, err
	}

	return r.FindByIDWithCategory(ctx, updated.ID)
}

func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.Product.DeleteOneID(id).Exec(ctx)
	if err != nil && ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "product", Cause: err}
	}
	return err
}

func (r *ProductRepository) ToggleAvailability(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	current, err := r.db.Product.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Product{}, &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return dto.Product{}, err
	}

	if _, err := r.db.Product.UpdateOneID(id).
		SetIsAvailable(!current.IsAvailable).
		SetUpdatedAt(time.Now()).
		Save(ctx); err != nil {
		return dto.Product{}, err
	}

	return r.FindByIDWithCategory(ctx, id)
}

func (r *ProductRepository) attachEffectiveGlobalOptions(ctx context.Context, products []dto.Product) error {
	if len(products) == 0 {
		return nil
	}

	globalFlavors, err := r.db.Flavor.Query().
		Where(flavor.IsActiveEQ(true)).
		Order(ent.Asc(flavor.FieldDisplayOrder)).
		All(ctx)
	if err != nil {
		log.Error().Err(err).Msg("attachEffectiveGlobalOptions: failed to load global flavors")
		return err
	}

	globalAddons, err := r.db.Addon.Query().
		Where(addon.IsActiveEQ(true)).
		Order(ent.Asc(addon.FieldDisplayOrder)).
		All(ctx)
	if err != nil {
		log.Error().Err(err).Msg("attachEffectiveGlobalOptions: failed to load global addons")
		return err
	}

	flavorDTOs := make([]dto.Flavor, len(globalFlavors))
	for i, f := range globalFlavors {
		flavorDTOs[i] = toFlavorDTO(f)
	}

	addonDTOs := make([]dto.Addon, len(globalAddons))
	for i, a := range globalAddons {
		addonDTOs[i] = toAddonDTO(a)
	}

	log.Debug().
		Int("global_flavor_count", len(flavorDTOs)).
		Int("global_addon_count", len(addonDTOs)).
		Int("product_count", len(products)).
		Msg("attachEffectiveGlobalOptions: attaching global options")

	for i := range products {
		if products[i].ExcludeGlobalFlavors {
			products[i].Flavors = []dto.Flavor{}
		} else {
			products[i].Flavors = flavorDTOs
		}

		if products[i].ExcludeGlobalAddons {
			products[i].Addons = []dto.Addon{}
		} else {
			products[i].Addons = addonDTOs
		}
	}

	return nil
}

func toProductDTO(p *ent.Product) dto.Product {
	if p == nil {
		return dto.Product{}
	}
	product := dto.Product{
		ID:                   p.ID,
		Name:                 p.Name,
		Description:          p.Description,
		Price:                p.Price,
		ImagePath:            p.ImagePath,
		IsAvailable:          p.IsAvailable,
		ExcludeGlobalFlavors: p.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  p.ExcludeGlobalAddons,
		BallBased:            p.BallBased,
		BallQuantity:         p.BallQuantity,
		AllowsMixedFlavors:   p.AllowsMixedFlavors,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
	if p.Edges.Category != nil {
		cat := toCategoryDTO(p.Edges.Category)
		product.CategoryID = cat.ID
		product.Category = &cat
	}
	return product
}

type CreateProductInput struct {
	Name                 string
	Description          string
	Price                float64
	CategoryID           uuid.UUID
	ImagePath            *string
	ExcludeGlobalFlavors bool
	ExcludeGlobalAddons  bool
	BallBased            *bool
	BallQuantity         *int
	AllowsMixedFlavors   *bool
}

type UpdateProductInput struct {
	Name                 *string
	Description          *string
	Price                *float64
	CategoryID           *uuid.UUID
	ImagePath            *string
	IsAvailable          *bool
	ExcludeGlobalFlavors *bool
	ExcludeGlobalAddons  *bool
	BallBased            *bool
	BallQuantity         *int
	AllowsMixedFlavors   *bool
}
