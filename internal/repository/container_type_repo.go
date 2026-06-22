// container_type_repo.go — Ent-backed repository for container type CRUD operations.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/containertype"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type ContainerTypeRepository struct {
	db *ent.Client
}

func NewContainerTypeRepository(db *ent.Client) *ContainerTypeRepository {
	return &ContainerTypeRepository{db: db}
}

func (r *ContainerTypeRepository) List(ctx context.Context) ([]dto.ContainerType, error) {
	rows, err := r.db.ContainerType.Query().
		Order(ent.Asc(containertype.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ContainerType, len(rows))
	for i, row := range rows {
		result[i] = toContainerTypeDTO(row)
	}
	return result, nil
}

func (r *ContainerTypeRepository) GetByID(ctx context.Context, id uuid.UUID) (dto.ContainerType, error) {
	row, err := r.db.ContainerType.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.ContainerType{}, &apperror.NotFoundError{Resource: "container_type", Cause: err}
		}
		return dto.ContainerType{}, err
	}
	return toContainerTypeDTO(row), nil
}

func (r *ContainerTypeRepository) Create(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error) {
	row, err := r.db.ContainerType.Create().
		SetName(name).
		SetBallsPerContainer(ballsPerContainer).
		SetIsCustom(isCustom).
		Save(ctx)
	if err != nil {
		return dto.ContainerType{}, err
	}
	return toContainerTypeDTO(row), nil
}

func (r *ContainerTypeRepository) Update(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error) {
	upd := r.db.ContainerType.UpdateOneID(id)
	if name != nil {
		upd.SetName(*name)
	}
	if ballsPerContainer != nil {
		upd.SetBallsPerContainer(*ballsPerContainer)
	}
	if isCustom != nil {
		upd.SetIsCustom(*isCustom)
	}
	if isActive != nil {
		upd.SetIsActive(*isActive)
	}
	row, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.ContainerType{}, &apperror.NotFoundError{Resource: "container_type", Cause: err}
		}
		return dto.ContainerType{}, err
	}
	return toContainerTypeDTO(row), nil
}

func (r *ContainerTypeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.ContainerType.DeleteOneID(id).Exec(ctx)
	if err != nil && ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "container_type", Cause: err}
	}
	return err
}

func toContainerTypeDTO(ct *ent.ContainerType) dto.ContainerType {
	if ct == nil {
		return dto.ContainerType{}
	}
	return dto.ContainerType{
		ID:                ct.ID,
		Name:              ct.Name,
		BallsPerContainer: ct.BallsPerContainer,
		IsCustom:          ct.IsCustom,
		IsActive:          ct.IsActive,
		CreatedAt:         ct.CreatedAt,
		UpdatedAt:         ct.UpdatedAt,
	}
}
