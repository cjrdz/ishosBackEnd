package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosFactory-backend/ent"
	"github.com/cjrdz/ishosFactory-backend/internal/repository"
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
