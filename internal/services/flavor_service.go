// flavor_service.go — CRUD service for flavors with a short-lived in-process cache.
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

type FlavorService struct {
	repo  flavorRepository
	cache *cache.TTLCache
}

type flavorRepository interface {
	FindAll(ctx context.Context, onlyActive bool) ([]*ent.Flavor, error)
	FindByID(ctx context.Context, id uuid.UUID) (*ent.Flavor, error)
	Create(ctx context.Context, input repository.CreateFlavorInput) (*ent.Flavor, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (*ent.Flavor, error)
	Delete(ctx context.Context, id uuid.UUID) error
	LinkToProduct(ctx context.Context, flavorID, productID uuid.UUID) error
	UnlinkFromProduct(ctx context.Context, flavorID, productID uuid.UUID) error
}

func NewFlavorService(repo flavorRepository) *FlavorService {
	return &FlavorService{repo: repo, cache: cache.New(45 * time.Second)}
}

func (s *FlavorService) List(ctx context.Context, onlyActive bool) ([]*ent.Flavor, error) {
	key := fmt.Sprintf("list:%t", onlyActive)
	if v, ok := s.cache.Get(key); ok {
		return v.([]*ent.Flavor), nil
	}
	result, err := s.repo.FindAll(ctx, onlyActive)
	if err == nil {
		s.cache.Set(key, result)
	}
	return result, err
}

func (s *FlavorService) GetByID(ctx context.Context, id uuid.UUID) (*ent.Flavor, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *FlavorService) Create(ctx context.Context, input repository.CreateFlavorInput) (*ent.Flavor, error) {
	f, err := s.repo.Create(ctx, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return f, err
}

func (s *FlavorService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (*ent.Flavor, error) {
	f, err := s.repo.Update(ctx, id, input)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return f, err
}

func (s *FlavorService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func (s *FlavorService) LinkToProduct(ctx context.Context, flavorID, productID uuid.UUID) error {
	return s.repo.LinkToProduct(ctx, flavorID, productID)
}

func (s *FlavorService) UnlinkFromProduct(ctx context.Context, flavorID, productID uuid.UUID) error {
	return s.repo.UnlinkFromProduct(ctx, flavorID, productID)
}
