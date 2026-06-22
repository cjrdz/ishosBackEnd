// container_type_service.go — CRUD service for container types.
package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type ContainerTypeService struct {
	repo ContainerTypeRepository
}

func NewContainerTypeService(repo ContainerTypeRepository) *ContainerTypeService {
	return &ContainerTypeService{repo: repo}
}

func (s *ContainerTypeService) List(ctx context.Context) ([]dto.ContainerType, error) {
	return s.repo.List(ctx)
}

func (s *ContainerTypeService) GetByID(ctx context.Context, id uuid.UUID) (dto.ContainerType, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ContainerTypeService) Create(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error) {
	return s.repo.Create(ctx, name, ballsPerContainer, isCustom)
}

func (s *ContainerTypeService) Update(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error) {
	return s.repo.Update(ctx, id, name, ballsPerContainer, isCustom, isActive)
}

func (s *ContainerTypeService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
