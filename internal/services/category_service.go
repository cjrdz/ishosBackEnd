// category_service.go — CRUD service for categories with a short-lived in-process cache.
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

type CategoryService struct {
	repo  CategoryRepository
	cache *cache.TTLCache
}

func NewCategoryService(repo CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo, cache: cache.New(45 * time.Second)}
}

func (s *CategoryService) List(ctx context.Context, showAll bool) ([]dto.Category, error) {
	key := fmt.Sprintf("list:%t", showAll)
	if v, ok := s.cache.Get(key); ok {
		return v.([]dto.Category), nil
	}

	var result []dto.Category
	var err error
	if showAll {
		result, err = s.repo.FindAll(ctx)
	} else {
		result, err = s.repo.FindActive(ctx)
	}

	if err == nil {
		s.cache.Set(key, result)
	}
	return result, err
}

func (s *CategoryService) GetBySlug(ctx context.Context, slug string) (dto.Category, error) {
	return s.repo.FindBySlugWithProducts(ctx, slug)
}

func (s *CategoryService) Create(ctx context.Context, input repository.CreateCategoryInput) (dto.Category, error) {
	c, err := s.repo.Create(ctx, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return c, err
}

func (s *CategoryService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateCategoryInput) (dto.Category, error) {
	c, err := s.repo.Update(ctx, id, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return c, err
}

func (s *CategoryService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}
