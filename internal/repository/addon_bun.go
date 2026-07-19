// addon_bun.go — Bun-backed repository for addon CRUD.
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

// AddonBunRepository implements services.AddonRepository using Bun.
type AddonBunRepository struct {
	db bun.IDB
}

// NewAddonBunRepository creates a new Bun-backed addon repository.
func NewAddonBunRepository(db bun.IDB) *AddonBunRepository {
	return &AddonBunRepository{db: db}
}

// FindAll returns all addons, optionally filtered to active ones.
func (r *AddonBunRepository) FindAll(ctx context.Context, onlyActive bool) ([]dto.Addon, error) {
	var rows []models.Addon
	q := r.db.NewSelect().Model(&rows).Order("display_order ASC")
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	result := make([]dto.Addon, len(rows))
	for i, row := range rows {
		result[i] = toAddonDTOFromModel(row)
	}
	return result, nil
}

// FindByID returns an addon by ID.
func (r *AddonBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Addon, error) {
	var row models.Addon
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Addon{}, &apperror.NotFoundError{Resource: "addon", Cause: err}
		}
		return dto.Addon{}, err
	}
	return toAddonDTOFromModel(row), nil
}

// nextAddonDisplayOrder returns the next available display_order for addons.
func (r *AddonBunRepository) nextDisplayOrder(ctx context.Context, db bun.IDB) (int, error) {
	var max sql.NullInt64
	if err := db.NewSelect().
		Table("addons").
		ColumnExpr("COALESCE(MAX(display_order), -1)").
		Scan(ctx, &max); err != nil {
		return 0, err
	}
	return int(max.Int64) + 1, nil
}

// Create creates a new addon. If input.DisplayOrder is nil, it is auto-assigned.
func (r *AddonBunRepository) Create(ctx context.Context, input CreateAddonInput) (dto.Addon, error) {
	displayOrder := 0
	if input.DisplayOrder != nil {
		displayOrder = *input.DisplayOrder
	} else {
		next, err := r.nextDisplayOrder(ctx, r.db)
		if err != nil {
			return dto.Addon{}, err
		}
		displayOrder = next
	}

	now := time.Now()
	row := models.Addon{
		ID:           uuid.New(),
		Name:         input.Name,
		Price:        input.Price,
		GroupName:    input.GroupName,
		DisplayOrder: displayOrder,
		IsActive:     input.IsActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := r.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return dto.Addon{}, err
	}
	return toAddonDTOFromModel(row), nil
}

// Update updates an existing addon.
func (r *AddonBunRepository) Update(ctx context.Context, id uuid.UUID, input UpdateAddonInput) (dto.Addon, error) {
	row, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Addon{}, err
	}

	model := models.Addon{
		ID:           row.ID,
		Name:         row.Name,
		Price:        row.Price,
		GroupName:    row.GroupName,
		DisplayOrder: row.DisplayOrder,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    time.Now(),
	}

	if input.Name != nil {
		model.Name = *input.Name
	}
	if input.Price != nil {
		model.Price = *input.Price
	}
	if input.GroupName != nil {
		model.GroupName = *input.GroupName
	}
	if input.DisplayOrder != nil {
		model.DisplayOrder = *input.DisplayOrder
	}
	if input.IsActive != nil {
		model.IsActive = *input.IsActive
	}

	res, err := r.db.NewUpdate().Model(&model).WherePK().Exec(ctx)
	if err != nil {
		return dto.Addon{}, err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Addon{}, &apperror.NotFoundError{Resource: "addon"}
	}

	return toAddonDTOFromModel(model), nil
}

// Delete deletes an addon by ID and compacts the display_order of the remaining addons.
func (r *AddonBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		var row models.Addon
		if err := tx.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &apperror.NotFoundError{Resource: "addon", Cause: err}
			}
			return err
		}

		model := models.Addon{ID: id}
		res, err := tx.NewDelete().Model(&model).WherePK().Exec(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &apperror.NotFoundError{Resource: "addon", Cause: err}
			}
			return err
		}
		if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
			return &apperror.NotFoundError{Resource: "addon"}
		}

		_, err = tx.NewUpdate().
			Table("addons").
			Set("display_order = display_order - 1").
			Where("display_order > ?", row.DisplayOrder).
			Exec(ctx)
		return err
	})
}

func toAddonDTOFromModel(m models.Addon) dto.Addon {
	return dto.Addon{
		ID:           m.ID,
		Name:         m.Name,
		Price:        m.Price,
		GroupName:    m.GroupName,
		DisplayOrder: m.DisplayOrder,
		IsActive:     m.IsActive,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
