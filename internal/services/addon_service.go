// addon_service.go — CRUD service for addons with a short-lived in-process cache.
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

type AddonService struct {
	repo  AddonRepository
	cache *cache.TTLCache
}

func NewAddonService(repo AddonRepository) *AddonService {
	return &AddonService{repo: repo, cache: cache.New(45 * time.Second)}
}

func (s *AddonService) List(ctx context.Context, onlyActive bool) ([]dto.Addon, error) {
	key := fmt.Sprintf("list:%t", onlyActive)
	if v, ok := s.cache.Get(key); ok {
		return v.([]dto.Addon), nil
	}

	result, err := s.repo.FindAll(ctx, onlyActive)
	if err == nil {
		s.cache.Set(key, result)
	}

	return result, err
}

func (s *AddonService) GetByID(ctx context.Context, id uuid.UUID) (dto.Addon, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *AddonService) Create(ctx context.Context, input repository.CreateAddonInput) (dto.Addon, error) {
	a, err := s.repo.Create(ctx, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return a, err
}

func (s *AddonService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (dto.Addon, error) {
	a, err := s.repo.Update(ctx, id, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return a, err
}

func (s *AddonService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}
