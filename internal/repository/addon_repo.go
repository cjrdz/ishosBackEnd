// addon_repo.go — Ent-backed repository for addon CRUD operations.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/addon"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type AddonRepository struct {
	db *ent.Client
}

func NewAddonRepository(db *ent.Client) *AddonRepository {
	return &AddonRepository{db: db}
}

func (r *AddonRepository) FindAll(ctx context.Context, onlyActive bool) ([]dto.Addon, error) {
	q := r.db.Addon.Query()

	if onlyActive {
		q = q.Where(addon.IsActiveEQ(true))
	}

	rows, err := q.Order(ent.Asc(addon.FieldDisplayOrder)).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.Addon, len(rows))
	for i, row := range rows {
		result[i] = toAddonDTO(row)
	}
	return result, nil
}

func (r *AddonRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Addon, error) {
	row, err := r.db.Addon.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Addon{}, &apperror.NotFoundError{Resource: "addon", Cause: err}
		}
		return dto.Addon{}, err
	}
	return toAddonDTO(row), nil
}

func (r *AddonRepository) Create(ctx context.Context, input CreateAddonInput) (dto.Addon, error) {
	now := time.Now()
	row, err := r.db.Addon.Create().
		SetName(input.Name).
		SetPrice(input.Price).
		SetGroupName(input.GroupName).
		SetDisplayOrder(input.DisplayOrder).
		SetIsActive(input.IsActive).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return dto.Addon{}, err
	}
	return toAddonDTO(row), nil
}

func (r *AddonRepository) Update(ctx context.Context, id uuid.UUID, input UpdateAddonInput) (dto.Addon, error) {
	upd := r.db.Addon.UpdateOneID(id).SetUpdatedAt(time.Now())

	if input.Name != nil {
		upd = upd.SetName(*input.Name)
	}
	if input.Price != nil {
		upd = upd.SetPrice(*input.Price)
	}
	if input.GroupName != nil {
		upd = upd.SetGroupName(*input.GroupName)
	}
	if input.DisplayOrder != nil {
		upd = upd.SetDisplayOrder(*input.DisplayOrder)
	}
	if input.IsActive != nil {
		upd = upd.SetIsActive(*input.IsActive)
	}

	row, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Addon{}, &apperror.NotFoundError{Resource: "addon", Cause: err}
		}
		return dto.Addon{}, err
	}
	return toAddonDTO(row), nil
}

func (r *AddonRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.Addon.DeleteOneID(id).Exec(ctx)
	if err != nil && ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "addon", Cause: err}
	}
	return err
}

func toAddonDTO(e *ent.Addon) dto.Addon {
	return dto.Addon{
		ID:           e.ID,
		Name:         e.Name,
		Price:        e.Price,
		GroupName:    e.GroupName,
		DisplayOrder: e.DisplayOrder,
		IsActive:     e.IsActive,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

type CreateAddonInput struct {
	Name         string
	Price        float64
	GroupName    string
	DisplayOrder int
	IsActive     bool
}

type UpdateAddonInput struct {
	Name         *string
	Price        *float64
	GroupName    *string
	DisplayOrder *int
	IsActive     *bool
}
