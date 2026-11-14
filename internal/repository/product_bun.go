// product_bun.go — Bun-backed repository for product CRUD and global option attachment.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// ProductBunRepository implements services.ProductRepository using Bun.
type ProductBunRepository struct {
	db    bun.IDB
	cache *cache.TTLCache
}

// NewProductBunRepository creates a new Bun-backed product repository.
// defaultFreeToppings matches the products.free_toppings column default.
const defaultFreeToppings = 1

func NewProductBunRepository(db bun.IDB) *ProductBunRepository {
	return &ProductBunRepository{db: db, cache: cache.New(30 * time.Second)}
}

// FindAll returns products, optionally filtered by category slug and availability.
func (r *ProductBunRepository) FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]dto.Product, error) {
	return r.findAll(ctx, categorySlug, onlyAvailable, 0, 0)
}

// FindAllPaginated returns a page of products plus the total count.
func (r *ProductBunRepository) FindAllPaginated(ctx context.Context, categorySlug string, onlyAvailable bool, page, perPage int) ([]dto.Product, int, error) {
	products, err := r.findAll(ctx, categorySlug, onlyAvailable, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	total, err := r.countAll(ctx, categorySlug, onlyAvailable)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (r *ProductBunRepository) buildProductQuery(ctx context.Context, categorySlug string, onlyAvailable bool) (*bun.SelectQuery, error) {
	q := r.db.NewSelect().Model((*models.Product)(nil)).Order("created_at ASC")

	if categorySlug != "" {
		var category models.Category
		if err := r.db.NewSelect().Model(&category).Where("slug = ?", categorySlug).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, sql.ErrNoRows
			}
			return nil, err
		}
		q = q.Where("category_products = ?", category.ID)
	}

	if onlyAvailable {
		q = q.Where("is_available = ?", true)
	}

	return q, nil
}

func (r *ProductBunRepository) countAll(ctx context.Context, categorySlug string, onlyAvailable bool) (int, error) {
	q, err := r.buildProductQuery(ctx, categorySlug, onlyAvailable)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return q.Count(ctx)
}

func (r *ProductBunRepository) findAll(ctx context.Context, categorySlug string, onlyAvailable bool, page, perPage int) ([]dto.Product, error) {
	q, err := r.buildProductQuery(ctx, categorySlug, onlyAvailable)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []dto.Product{}, nil
		}
		return nil, err
	}

	if page > 0 && perPage > 0 {
		offset := (page - 1) * perPage
		q = q.Limit(perPage).Offset(offset)
	}

	var rows []models.Product
	if err := q.Scan(ctx, &rows); err != nil {
		return nil, err
	}

	categories, err := r.loadCategoriesForProducts(ctx, rows)
	if err != nil {
		return nil, err
	}

	result := make([]dto.Product, len(rows))
	for i, row := range rows {
		result[i] = toProductDTOFromModel(row, categories[row.ID])
	}

	if err := r.attachEffectiveGlobalOptions(ctx, result); err != nil {
		return nil, err
	}

	return result, nil
}

// FindByID returns a product by ID without loading its category.
func (r *ProductBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	var row models.Product
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Product{}, &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return dto.Product{}, err
	}
	return toProductDTOFromModel(row, nil), nil
}

// FindByIDWithCategory returns a product with its category loaded.
func (r *ProductBunRepository) FindByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	return r.findByIDWithCategory(ctx, id)
}

// FindByIDWithCategoryAndOptions returns a product with category and global options attached.
func (r *ProductBunRepository) FindByIDWithCategoryAndOptions(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	product, err := r.findByIDWithCategory(ctx, id)
	if err != nil {
		return dto.Product{}, err
	}
	results := []dto.Product{product}
	if err := r.attachEffectiveGlobalOptions(ctx, results); err != nil {
		return dto.Product{}, err
	}
	return results[0], nil
}

func (r *ProductBunRepository) findByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	product, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Product{}, err
	}

	var category models.Category
	if err := r.db.NewSelect().Model(&category).Where("id = ?", product.CategoryID).Scan(ctx); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return dto.Product{}, err
		}
		return product, nil
	}

	cat := toCategoryDTOFromModel(category)
	product.Category = &cat
	return product, nil
}

// Create creates a new product.
func (r *ProductBunRepository) Create(ctx context.Context, input CreateProductInput) (dto.Product, error) {
	now := time.Now()
	row := models.Product{
		ID:                   uuid.New(),
		Name:                 input.Name,
		Description:          input.Description,
		Price:                input.Price,
		CategoryID:           input.CategoryID,
		ImagePath:            input.ImagePath,
		IsAvailable:          true,
		ExcludeGlobalFlavors: input.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  input.ExcludeGlobalAddons,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if input.BallBased != nil {
		row.BallBased = *input.BallBased
	}
	if input.BallQuantity != nil {
		row.BallQuantity = *input.BallQuantity
	}
	if input.AllowsMixedFlavors != nil {
		row.AllowsMixedFlavors = *input.AllowsMixedFlavors
	}
	// One free topping unless the caller says otherwise.
	row.FreeToppings = defaultFreeToppings
	if input.FreeToppings != nil {
		row.FreeToppings = *input.FreeToppings
	}

	// Written explicitly: with the `default` tag Bun replaces a zero value
	// with the column default, so a deliberate "0 free toppings" became 1.
	if _, err := r.db.NewInsert().Model(&row).Value("free_toppings", "?", row.FreeToppings).Exec(ctx); err != nil {
		return dto.Product{}, err
	}
	return r.findByIDWithCategory(ctx, row.ID)
}

// Update updates an existing product.
func (r *ProductBunRepository) Update(ctx context.Context, id uuid.UUID, input UpdateProductInput) (dto.Product, error) {
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Product{}, err
	}

	row := models.Product{
		ID:                   existing.ID,
		Name:                 existing.Name,
		Description:          existing.Description,
		Price:                existing.Price,
		CategoryID:           existing.CategoryID,
		ImagePath:            existing.ImagePath,
		IsAvailable:          existing.IsAvailable,
		ExcludeGlobalFlavors: existing.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  existing.ExcludeGlobalAddons,
		BallBased:            existing.BallBased,
		BallQuantity:         existing.BallQuantity,
		AllowsMixedFlavors:   existing.AllowsMixedFlavors,
		// Was missing: every product edit reset the free toppings to 0, so the
		// first topping started being charged.
		FreeToppings: existing.FreeToppings,
		CreatedAt:    existing.CreatedAt,
		UpdatedAt:    time.Now(),
	}

	if input.Name != nil {
		row.Name = *input.Name
	}
	if input.Description != nil {
		row.Description = *input.Description
	}
	if input.Price != nil {
		row.Price = *input.Price
	}
	if input.CategoryID != nil {
		row.CategoryID = *input.CategoryID
	}
	if input.ImagePath != nil {
		row.ImagePath = input.ImagePath
	}
	if input.IsAvailable != nil {
		row.IsAvailable = *input.IsAvailable
	}
	if input.ExcludeGlobalFlavors != nil {
		row.ExcludeGlobalFlavors = *input.ExcludeGlobalFlavors
	}
	if input.ExcludeGlobalAddons != nil {
		row.ExcludeGlobalAddons = *input.ExcludeGlobalAddons
	}
	if input.BallBased != nil {
		row.BallBased = *input.BallBased
	}
	if input.BallQuantity != nil {
		row.BallQuantity = *input.BallQuantity
	}
	if input.AllowsMixedFlavors != nil {
		row.AllowsMixedFlavors = *input.AllowsMixedFlavors
	}
	if input.FreeToppings != nil {
		row.FreeToppings = *input.FreeToppings
	}

	res, err := r.db.NewUpdate().Model(&row).WherePK().Exec(ctx)
	if err != nil {
		return dto.Product{}, err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Product{}, &apperror.NotFoundError{Resource: "product"}
	}

	return r.findByIDWithCategory(ctx, row.ID)
}

// Delete deletes a product by ID.
func (r *ProductBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	row := models.Product{ID: id}
	res, err := r.db.NewDelete().Model(&row).WherePK().Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &apperror.NotFoundError{Resource: "product", Cause: err}
		}
		return err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return &apperror.NotFoundError{Resource: "product"}
	}
	return nil
}

// ToggleAvailability flips the is_available flag for a product.
func (r *ProductBunRepository) ToggleAvailability(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Product{}, err
	}

	row := models.Product{ID: existing.ID, IsAvailable: !existing.IsAvailable, UpdatedAt: time.Now()}
	if _, err := r.db.NewUpdate().Model(&row).Set("is_available = ?, updated_at = ?", row.IsAvailable, row.UpdatedAt).Where("id = ?", row.ID).Exec(ctx); err != nil {
		return dto.Product{}, err
	}

	return r.findByIDWithCategory(ctx, id)
}

func (r *ProductBunRepository) loadCategoriesForProducts(ctx context.Context, products []models.Product) (map[uuid.UUID]*models.Category, error) {
	result := make(map[uuid.UUID]*models.Category)
	if len(products) == 0 {
		return result, nil
	}

	categoryIDs := make([]uuid.UUID, 0, len(products))
	seen := make(map[uuid.UUID]struct{})
	for _, p := range products {
		if _, ok := seen[p.CategoryID]; !ok {
			seen[p.CategoryID] = struct{}{}
			categoryIDs = append(categoryIDs, p.CategoryID)
		}
	}

	var categories []models.Category
	if err := r.db.NewSelect().Model(&categories).Where("id IN (?)", bun.List(categoryIDs)).Scan(ctx); err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]models.Category, len(categories))
	for _, c := range categories {
		byID[c.ID] = c
	}

	for _, p := range products {
		if c, ok := byID[p.CategoryID]; ok {
			cp := c
			result[p.ID] = &cp
		}
	}
	return result, nil
}

//nolint:gosec // G602 false positive: range loop guarantees valid slice indices.
func (r *ProductBunRepository) attachEffectiveGlobalOptions(ctx context.Context, products []dto.Product) error {
	if len(products) == 0 {
		return nil
	}

	flavorDTOs, err := r.loadCachedFlavors(ctx)
	if err != nil {
		return err
	}
	addonDTOs, err := r.loadCachedAddons(ctx)
	if err != nil {
		return err
	}

	log.Debug().
		Int("global_flavor_count", len(flavorDTOs)).
		Int("global_addon_count", len(addonDTOs)).
		Int("product_count", len(products)).
		Msg("attachEffectiveGlobalOptions: attaching global options")

	// #nosec G602 -- loop uses range, so the index is always within bounds.
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

func (r *ProductBunRepository) loadCachedFlavors(ctx context.Context) ([]dto.Flavor, error) {
	if v, ok := r.cache.Get("global_flavors"); ok {
		return v.([]dto.Flavor), nil
	}

	var rows []models.Flavor
	if err := r.db.NewSelect().Model(&rows).Where("is_active = ?", true).Order("display_order ASC").Scan(ctx); err != nil {
		log.Error().Err(err).Msg("attachEffectiveGlobalOptions: failed to load global flavors")
		return nil, err
	}

	flavorDTOs := make([]dto.Flavor, len(rows))
	for i, f := range rows {
		flavorDTOs[i] = toFlavorDTOFromModel(f)
	}
	r.cache.Set("global_flavors", flavorDTOs)
	return flavorDTOs, nil
}

func (r *ProductBunRepository) loadCachedAddons(ctx context.Context) ([]dto.Addon, error) {
	if v, ok := r.cache.Get("global_addons"); ok {
		return v.([]dto.Addon), nil
	}

	var rows []models.Addon
	if err := r.db.NewSelect().Model(&rows).Where("is_active = ?", true).Order("display_order ASC").Scan(ctx); err != nil {
		log.Error().Err(err).Msg("attachEffectiveGlobalOptions: failed to load global addons")
		return nil, err
	}

	addonDTOs := make([]dto.Addon, len(rows))
	for i, a := range rows {
		addonDTOs[i] = toAddonDTOFromModel(a)
	}
	r.cache.Set("global_addons", addonDTOs)
	return addonDTOs, nil
}

func toProductDTOFromModel(p models.Product, category *models.Category) dto.Product {
	product := dto.Product{
		ID:                   p.ID,
		Name:                 p.Name,
		Description:          p.Description,
		Price:                p.Price,
		CategoryID:           p.CategoryID,
		ImagePath:            p.ImagePath,
		IsAvailable:          p.IsAvailable,
		ExcludeGlobalFlavors: p.ExcludeGlobalFlavors,
		ExcludeGlobalAddons:  p.ExcludeGlobalAddons,
		BallBased:            p.BallBased,
		BallQuantity:         p.BallQuantity,
		AllowsMixedFlavors:   p.AllowsMixedFlavors,
		FreeToppings:         p.FreeToppings,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
	if category != nil {
		cat := toCategoryDTOFromModel(*category)
		product.Category = &cat
	}
	return product
}
