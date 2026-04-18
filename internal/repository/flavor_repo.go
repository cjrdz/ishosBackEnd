// flavor_repo.go — Ent-backed repository for flavor CRUD and product many-to-many linking.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/flavor"
)

type FlavorRepository struct {
	db *ent.Client
}

func NewFlavorRepository(db *ent.Client) *FlavorRepository {
	return &FlavorRepository{db: db}
}

func (r *FlavorRepository) FindAll(ctx context.Context, onlyActive bool) ([]*ent.Flavor, error) {
	q := r.db.Flavor.Query()

	if onlyActive {
		q = q.Where(flavor.IsActiveEQ(true))
	}

	return q.Order(ent.Asc(flavor.FieldDisplayOrder)).All(ctx)
}

func (r *FlavorRepository) FindByID(ctx context.Context, id uuid.UUID) (*ent.Flavor, error) {
	return r.db.Flavor.Get(ctx, id)
}

func (r *FlavorRepository) Create(ctx context.Context, input CreateFlavorInput) (*ent.Flavor, error) {
	now := time.Now()
	return r.db.Flavor.Create().
		SetName(input.Name).
		SetDisplayOrder(input.DisplayOrder).
		SetIsSeasonal(input.IsSeasonal).
		SetIsActive(input.IsActive).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
}

func (r *FlavorRepository) Update(ctx context.Context, id uuid.UUID, input UpdateFlavorInput) (*ent.Flavor, error) {
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

	return upd.Save(ctx)
}

func (r *FlavorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.Flavor.DeleteOneID(id).Exec(ctx)
}

// LinkToProduct adds a flavor to a product (creates many-to-many relation)
func (r *FlavorRepository) LinkToProduct(ctx context.Context, flavorID, productID uuid.UUID) error {
	return r.db.Product.UpdateOneID(productID).
		AddFlavorIDs(flavorID).
		Exec(ctx)
}

// UnlinkFromProduct removes a flavor from a product
func (r *FlavorRepository) UnlinkFromProduct(ctx context.Context, flavorID, productID uuid.UUID) error {
	return r.db.Product.UpdateOneID(productID).
		RemoveFlavorIDs(flavorID).
		Exec(ctx)
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
