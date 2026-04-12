package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type FlavorService struct {
	repo *repository.FlavorRepository
}

func NewFlavorService(repo *repository.FlavorRepository) *FlavorService {
	return &FlavorService{repo: repo}
}

func (s *FlavorService) List(ctx context.Context, onlyActive bool) ([]*ent.Flavor, error) {
	return s.repo.FindAll(ctx, onlyActive)
}

func (s *FlavorService) GetByID(ctx context.Context, id uuid.UUID) (*ent.Flavor, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *FlavorService) Create(ctx context.Context, input repository.CreateFlavorInput) (*ent.Flavor, error) {
	return s.repo.Create(ctx, input)
}

func (s *FlavorService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (*ent.Flavor, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *FlavorService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *FlavorService) LinkToProduct(ctx context.Context, flavorID, productID uuid.UUID) error {
	return s.repo.LinkToProduct(ctx, flavorID, productID)
}

func (s *FlavorService) UnlinkFromProduct(ctx context.Context, flavorID, productID uuid.UUID) error {
	return s.repo.UnlinkFromProduct(ctx, flavorID, productID)
}
