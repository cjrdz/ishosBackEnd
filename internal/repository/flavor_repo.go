// flavor_repo.go — Ent-backed repository for flavor CRUD and product many-to-many linking.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/flavor"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type FlavorRepository struct {
	db *ent.Client
}

func NewFlavorRepository(db *ent.Client) *FlavorRepository {
	return &FlavorRepository{db: db}
}

func (r *FlavorRepository) FindAll(ctx context.Context, onlyActive bool) ([]dto.Flavor, error) {
	q := r.db.Flavor.Query()

	if onlyActive {
		q = q.Where(flavor.IsActiveEQ(true))
	}

	rows, err := q.Order(ent.Asc(flavor.FieldDisplayOrder)).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.Flavor, len(rows))
	for i, row := range rows {
		result[i] = toFlavorDTO(row)
	}
	return result, nil
}

func (r *FlavorRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Flavor, error) {
	row, err := r.db.Flavor.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Flavor{}, &apperror.NotFoundError{Resource: "flavor", Cause: err}
		}
		return dto.Flavor{}, err
	}
	return toFlavorDTO(row), nil
}

func (r *FlavorRepository) Create(ctx context.Context, input CreateFlavorInput) (dto.Flavor, error) {
	now := time.Now()
	row, err := r.db.Flavor.Create().
		SetName(input.Name).
		SetDisplayOrder(input.DisplayOrder).
		SetIsSeasonal(input.IsSeasonal).
		SetIsActive(input.IsActive).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return dto.Flavor{}, err
	}
	return toFlavorDTO(row), nil
}

func (r *FlavorRepository) Update(ctx context.Context, id uuid.UUID, input UpdateFlavorInput) (dto.Flavor, error) {
	upd := r.db.Flavor.UpdateOneID(id).SetUpdatedAt(time.Now())

	if input.Name != nil {
		upd = upd.SetName(*input.Name)
	}
	if input.DisplayOrder != nil {
		upd = upd.SetDisplayOrder(*input.DisplayOrder)
	}
	if input.IsSeasonal != nil {
		upd = upd.SetIsSeasonal(*input.IsSeasonal)
	}
	if input.IsActive != nil {
		upd = upd.SetIsActive(*input.IsActive)
	}

	row, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.Flavor{}, &apperror.NotFoundError{Resource: "flavor", Cause: err}
		}
		return dto.Flavor{}, err
	}
	return toFlavorDTO(row), nil
}

func (r *FlavorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.Flavor.DeleteOneID(id).Exec(ctx)
	if err != nil && ent.IsNotFound(err) {
		return &apperror.NotFoundError{Resource: "flavor", Cause: err}
	}
	return err
}

func toFlavorDTO(e *ent.Flavor) dto.Flavor {
	return dto.Flavor{
		ID:           e.ID,
		Name:         e.Name,
		DisplayOrder: e.DisplayOrder,
		IsSeasonal:   e.IsSeasonal,
		IsActive:     e.IsActive,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

type CreateFlavorInput struct {
	Name         string
	DisplayOrder int
	IsSeasonal   bool
	IsActive     bool
}

type UpdateFlavorInput struct {
	Name         *string
	DisplayOrder *int
	IsSeasonal   *bool
	IsActive     *bool
}
