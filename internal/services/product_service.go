// product_service.go — CRUD service for products with a short-lived in-process cache.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type ProductService struct {
	repo  ProductRepository
	cache *cache.TTLCache
}

func NewProductService(repo ProductRepository) *ProductService {
	return &ProductService{repo: repo, cache: cache.New(5 * time.Minute)}
}

func (s *ProductService) List(ctx context.Context, categorySlug string, includeAll bool) ([]dto.Product, error) {
	key := fmt.Sprintf("list:%s:%t", categorySlug, includeAll)
	if v, ok := s.cache.Get(key); ok {
		return v.([]dto.Product), nil
	}
	result, err := s.repo.FindAll(ctx, categorySlug, !includeAll)
	if err == nil {
		s.cache.Set(key, result)
	}
	return result, err
}

func (s *ProductService) GetByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	return s.repo.FindByIDWithCategory(ctx, id)
}

func (s *ProductService) Create(ctx context.Context, input repository.CreateProductInput) (dto.Product, error) {
	p, err := s.repo.Create(ctx, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return p, err
}

func (s *ProductService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (dto.Product, error) {
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

func (s *ProductService) ToggleAvailability(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	p, err := s.repo.ToggleAvailability(ctx, id)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return p, err
}
