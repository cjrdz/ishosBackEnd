// addon_service.go — CRUD service for addons with a short-lived in-process cache.
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

type AddonService struct {
	repo  addonRepository
	cache *cache.TTLCache
}

type addonRepository interface {
	FindAll(ctx context.Context, onlyActive bool) ([]*ent.Addon, error)
	FindByID(ctx context.Context, id uuid.UUID) (*ent.Addon, error)
	Create(ctx context.Context, input repository.CreateAddonInput) (*ent.Addon, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (*ent.Addon, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

func NewAddonService(repo addonRepository) *AddonService {
	return &AddonService{repo: repo, cache: cache.New(45 * time.Second)}
}

func (s *AddonService) List(ctx context.Context, onlyActive bool) ([]*ent.Addon, error) {
	key := fmt.Sprintf("list:%t", onlyActive)
	if v, ok := s.cache.Get(key); ok {
		return v.([]*ent.Addon), nil
	}

	result, err := s.repo.FindAll(ctx, onlyActive)
	if err == nil {
		s.cache.Set(key, result)
	}

	return result, err
}

func (s *AddonService) GetByID(ctx context.Context, id uuid.UUID) (*ent.Addon, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *AddonService) Create(ctx context.Context, input repository.CreateAddonInput) (*ent.Addon, error) {
	a, err := s.repo.Create(ctx, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return a, err
}

func (s *AddonService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (*ent.Addon, error) {
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
