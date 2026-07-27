// container_type_bun.go — Bun-backed repository for container type CRUD.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// ContainerTypeBunRepository implements services.ContainerTypeRepository using Bun.
type ContainerTypeBunRepository struct {
	db bun.IDB
}

// NewContainerTypeBunRepository creates a new Bun-backed container type repository.
func NewContainerTypeBunRepository(db bun.IDB) *ContainerTypeBunRepository {
	return &ContainerTypeBunRepository{db: db}
}

// List returns all container types ordered by name.
func (r *ContainerTypeBunRepository) List(ctx context.Context) ([]dto.ContainerType, error) {
	var rows []models.ContainerType
	if err := r.db.NewSelect().Model(&rows).Order("name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]dto.ContainerType, len(rows))
	for i, row := range rows {
		result[i] = toContainerTypeDTOFromModel(row)
	}
	return result, nil
}

// GetByID returns a container type by ID.
func (r *ContainerTypeBunRepository) GetByID(ctx context.Context, id uuid.UUID) (dto.ContainerType, error) {
	var row models.ContainerType
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.ContainerType{}, &apperror.NotFoundError{Resource: "container_type", Cause: err}
		}
		return dto.ContainerType{}, err
	}
	return toContainerTypeDTOFromModel(row), nil
}

// Create creates a new container type.
func (r *ContainerTypeBunRepository) Create(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error) {
	now := time.Now()
	row := models.ContainerType{
		ID:                uuid.New(),
		Name:              name,
		BallsPerContainer: ballsPerContainer,
		IsCustom:          isCustom,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if _, err := r.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return dto.ContainerType{}, err
	}
	return toContainerTypeDTOFromModel(row), nil
}

// Update updates an existing container type.
func (r *ContainerTypeBunRepository) Update(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return dto.ContainerType{}, err
	}

	row := models.ContainerType{
		ID:                existing.ID,
		Name:              existing.Name,
		BallsPerContainer: existing.BallsPerContainer,
		IsCustom:          existing.IsCustom,
		IsActive:          existing.IsActive,
		CreatedAt:         existing.CreatedAt,
		UpdatedAt:         time.Now(),
	}

	if name != nil {
		row.Name = *name
	}
	if ballsPerContainer != nil {
		row.BallsPerContainer = *ballsPerContainer
	}
	if isCustom != nil {
		row.IsCustom = *isCustom
	}
	if isActive != nil {
		row.IsActive = *isActive
	}

	res, err := r.db.NewUpdate().Model(&row).WherePK().Exec(ctx)
	if err != nil {
		return dto.ContainerType{}, err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.ContainerType{}, &apperror.NotFoundError{Resource: "container_type"}
	}

	return toContainerTypeDTOFromModel(row), nil
}

// Delete deletes a container type by ID.
func (r *ContainerTypeBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	row := models.ContainerType{ID: id}
	res, err := r.db.NewDelete().Model(&row).WherePK().Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &apperror.NotFoundError{Resource: "container_type", Cause: err}
		}
		return err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return &apperror.NotFoundError{Resource: "container_type"}
	}
	return nil
}

func toContainerTypeDTOFromModel(ct models.ContainerType) dto.ContainerType {
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
