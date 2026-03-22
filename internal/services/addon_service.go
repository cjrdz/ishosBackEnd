package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type AddonService struct {
	repo *repository.AddonRepository
}

func NewAddonService(repo *repository.AddonRepository) *AddonService {
	return &AddonService{repo: repo}
}

func (s *AddonService) List(ctx context.Context, onlyActive bool) ([]*ent.Addon, error) {
	return s.repo.FindAll(ctx, onlyActive)
}

func (s *AddonService) GetByID(ctx context.Context, id uuid.UUID) (*ent.Addon, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *AddonService) Create(ctx context.Context, input repository.CreateAddonInput) (*ent.Addon, error) {
	return s.repo.Create(ctx, input)
}

func (s *AddonService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (*ent.Addon, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *AddonService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
