package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) List(ctx context.Context, categorySlug string, includeAll bool) ([]*ent.Product, error) {
	return s.repo.FindAll(ctx, categorySlug, !includeAll)
}

func (s *ProductService) GetByIDWithCategory(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	return s.repo.FindByIDWithCategory(ctx, id)
}

func (s *ProductService) Create(ctx context.Context, input repository.CreateProductInput) (*ent.Product, error) {
	return s.repo.Create(ctx, input)
}

func (s *ProductService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (*ent.Product, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *ProductService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *ProductService) ToggleAvailability(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	return s.repo.ToggleAvailability(ctx, id)
}

func (s *ProductService) LinkFlavor(ctx context.Context, productID, flavorID uuid.UUID) error {
	return s.repo.LinkFlavorToProduct(ctx, productID, flavorID)
}

func (s *ProductService) UnlinkFlavor(ctx context.Context, productID, flavorID uuid.UUID) error {
	return s.repo.UnlinkFlavorFromProduct(ctx, productID, flavorID)
}

func (s *ProductService) LinkAddon(ctx context.Context, productID, addonID uuid.UUID) error {
	return s.repo.LinkAddonToProduct(ctx, productID, addonID)
}

func (s *ProductService) UnlinkAddon(ctx context.Context, productID, addonID uuid.UUID) error {
	return s.repo.UnlinkAddonFromProduct(ctx, productID, addonID)
}
