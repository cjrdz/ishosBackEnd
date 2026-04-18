// addon_repo.go — Ent-backed repository for addon CRUD operations.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/addon"
)

type AddonRepository struct {
	db *ent.Client
}

func NewAddonRepository(db *ent.Client) *AddonRepository {
	return &AddonRepository{db: db}
}

func (r *AddonRepository) FindAll(ctx context.Context, onlyActive bool) ([]*ent.Addon, error) {
	q := r.db.Addon.Query()

	if onlyActive {
		q = q.Where(addon.IsActiveEQ(true))
	}

	return q.Order(ent.Asc(addon.FieldDisplayOrder)).All(ctx)
}

func (r *AddonRepository) FindByID(ctx context.Context, id uuid.UUID) (*ent.Addon, error) {
	return r.db.Addon.Get(ctx, id)
}

func (r *AddonRepository) Create(ctx context.Context, input CreateAddonInput) (*ent.Addon, error) {
	now := time.Now()
	return r.db.Addon.Create().
		SetName(input.Name).
		SetPrice(input.Price).
		SetGroupName(input.GroupName).
		SetDisplayOrder(input.DisplayOrder).
		SetIsActive(input.IsActive).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
}

func (r *AddonRepository) Update(ctx context.Context, id uuid.UUID, input UpdateAddonInput) (*ent.Addon, error) {
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

	return upd.Save(ctx)
}

func (r *AddonRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.Addon.DeleteOneID(id).Exec(ctx)
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
