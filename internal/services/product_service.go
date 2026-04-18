// product_service.go — CRUD service for products with a short-lived in-process cache.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type ProductService struct {
	repo  productRepository
	cache *cache.TTLCache
}

type productRepository interface {
	FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]*ent.Product, error)
	FindByIDWithCategory(ctx context.Context, id uuid.UUID) (*ent.Product, error)
	Create(ctx context.Context, input repository.CreateProductInput) (*ent.Product, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (*ent.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ToggleAvailability(ctx context.Context, id uuid.UUID) (*ent.Product, error)
	LinkFlavorToProduct(ctx context.Context, productID, flavorID uuid.UUID) error
	UnlinkFlavorFromProduct(ctx context.Context, productID, flavorID uuid.UUID) error
	LinkAddonToProduct(ctx context.Context, productID, addonID uuid.UUID) error
	UnlinkAddonFromProduct(ctx context.Context, productID, addonID uuid.UUID) error
}

func NewProductService(repo productRepository) *ProductService {
	return &ProductService{repo: repo, cache: cache.New(45 * time.Second)}
}

func (s *ProductService) List(ctx context.Context, categorySlug string, includeAll bool) ([]*ent.Product, error) {
	key := fmt.Sprintf("list:%s:%t", categorySlug, includeAll)
	if v, ok := s.cache.Get(key); ok {
		return v.([]*ent.Product), nil
	}
	result, err := s.repo.FindAll(ctx, categorySlug, !includeAll)
	if err == nil {
		s.cache.Set(key, result)
	}
	return result, err
}

func (s *ProductService) GetByIDWithCategory(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	return s.repo.FindByIDWithCategory(ctx, id)
}

func (s *ProductService) Create(ctx context.Context, input repository.CreateProductInput) (*ent.Product, error) {
	p, err := s.repo.Create(ctx, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return p, err
}

func (s *ProductService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (*ent.Product, error) {
	p, err := s.repo.Update(ctx, id, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return p, err
}

func (s *ProductService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func (s *ProductService) ToggleAvailability(ctx context.Context, id uuid.UUID) (*ent.Product, error) {
	p, err := s.repo.ToggleAvailability(ctx, id)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return p, err
}

func (s *ProductService) LinkFlavor(ctx context.Context, productID, flavorID uuid.UUID) error {
	err := s.repo.LinkFlavorToProduct(ctx, productID, flavorID)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func (s *ProductService) UnlinkFlavor(ctx context.Context, productID, flavorID uuid.UUID) error {
	err := s.repo.UnlinkFlavorFromProduct(ctx, productID, flavorID)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func (s *ProductService) LinkAddon(ctx context.Context, productID, addonID uuid.UUID) error {
	err := s.repo.LinkAddonToProduct(ctx, productID, addonID)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func (s *ProductService) UnlinkAddon(ctx context.Context, productID, addonID uuid.UUID) error {
	err := s.repo.UnlinkAddonFromProduct(ctx, productID, addonID)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}
