// flavor_bun.go — Bun-backed repository for flavor CRUD.
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

// FlavorBunRepository implements services.FlavorRepository using Bun.
type FlavorBunRepository struct {
	db bun.IDB
}

// NewFlavorBunRepository creates a new Bun-backed flavor repository.
func NewFlavorBunRepository(db bun.IDB) *FlavorBunRepository {
	return &FlavorBunRepository{db: db}
}

// FindAll returns all flavors, optionally filtered to active ones.
func (r *FlavorBunRepository) FindAll(ctx context.Context, onlyActive bool) ([]dto.Flavor, error) {
	var rows []models.Flavor
	q := r.db.NewSelect().Model(&rows).Order("display_order ASC")
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	result := make([]dto.Flavor, len(rows))
	for i, row := range rows {
		result[i] = toFlavorDTOFromModel(row)
	}
	return result, nil
}

// FindByID returns a flavor by ID.
func (r *FlavorBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Flavor, error) {
	var row models.Flavor
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Flavor{}, &apperror.NotFoundError{Resource: "flavor", Cause: err}
		}
		return dto.Flavor{}, err
	}
	return toFlavorDTOFromModel(row), nil
}

// nextFlavorDisplayOrder returns the next available display_order for flavors.
func (r *FlavorBunRepository) nextDisplayOrder(ctx context.Context, db bun.IDB) (int, error) {
	var max sql.NullInt64
	if err := db.NewSelect().
		Table("flavors").
		ColumnExpr("COALESCE(MAX(display_order), -1)").
		Scan(ctx, &max); err != nil {
		return 0, err
	}
	return int(max.Int64) + 1, nil
}

// Create creates a new flavor. If input.DisplayOrder is nil, it is auto-assigned.
func (r *FlavorBunRepository) Create(ctx context.Context, input CreateFlavorInput) (dto.Flavor, error) {
	displayOrder := 0
	if input.DisplayOrder != nil {
		displayOrder = *input.DisplayOrder
	} else {
		next, err := r.nextDisplayOrder(ctx, r.db)
		if err != nil {
			return dto.Flavor{}, err
		}
		displayOrder = next
	}

	now := time.Now()
	row := models.Flavor{
		ID:           uuid.New(),
		Name:         input.Name,
		DisplayOrder: displayOrder,
		IsSeasonal:   input.IsSeasonal,
		IsActive:     input.IsActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := r.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return dto.Flavor{}, err
	}
	return toFlavorDTOFromModel(row), nil
}

// Update updates an existing flavor.
func (r *FlavorBunRepository) Update(ctx context.Context, id uuid.UUID, input UpdateFlavorInput) (dto.Flavor, error) {
	row, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Flavor{}, err
	}

	model := models.Flavor{
		ID:           row.ID,
		Name:         row.Name,
		DisplayOrder: row.DisplayOrder,
		IsSeasonal:   row.IsSeasonal,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    time.Now(),
	}

	if input.Name != nil {
		model.Name = *input.Name
	}
	if input.DisplayOrder != nil {
		model.DisplayOrder = *input.DisplayOrder
	}
	if input.IsSeasonal != nil {
		model.IsSeasonal = *input.IsSeasonal
	}
	if input.IsActive != nil {
		model.IsActive = *input.IsActive
	}

	res, err := r.db.NewUpdate().Model(&model).WherePK().Exec(ctx)
	if err != nil {
		return dto.Flavor{}, err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Flavor{}, &apperror.NotFoundError{Resource: "flavor"}
	}

	return toFlavorDTOFromModel(model), nil
}

// Delete deletes a flavor by ID and compacts the display_order of the remaining flavors.
func (r *FlavorBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return runInTx(ctx, r.db, func(ctx context.Context, tx bun.Tx) error {
		var row models.Flavor
		if err := tx.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &apperror.NotFoundError{Resource: "flavor", Cause: err}
			}
			return err
		}

		model := models.Flavor{ID: id}
		res, err := tx.NewDelete().Model(&model).WherePK().Exec(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &apperror.NotFoundError{Resource: "flavor", Cause: err}
			}
			return err
		}
		if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
			return &apperror.NotFoundError{Resource: "flavor"}
		}

		_, err = tx.NewUpdate().
			Table("flavors").
			Set("display_order = display_order - 1").
			Where("display_order > ?", row.DisplayOrder).
			Exec(ctx)
		return err
	})
}

func toFlavorDTOFromModel(m models.Flavor) dto.Flavor {
	return dto.Flavor{
		ID:           m.ID,
		Name:         m.Name,
		DisplayOrder: m.DisplayOrder,
		IsSeasonal:   m.IsSeasonal,
		IsActive:     m.IsActive,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
