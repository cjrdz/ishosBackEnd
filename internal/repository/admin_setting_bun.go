// admin_setting_bun.go — Bun-backed repository for key/value admin settings stored as JSONB.
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

// AdminSettingBunRepository implements services.AdminSettingRepository using Bun.
type AdminSettingBunRepository struct {
	db bun.IDB
}

// NewAdminSettingBunRepository creates a new Bun-backed admin setting repository.
func NewAdminSettingBunRepository(db bun.IDB) *AdminSettingBunRepository {
	return &AdminSettingBunRepository{db: db}
}

// GetByKey returns a setting by key.
func (r *AdminSettingBunRepository) GetByKey(ctx context.Context, key string) (dto.AdminSetting, error) {
	var row models.AdminSetting
	if err := r.db.NewSelect().Model(&row).Where("key = ?", key).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.AdminSetting{}, &apperror.NotFoundError{Resource: "admin_setting", Cause: err}
		}
		return dto.AdminSetting{}, err
	}
	return toAdminSettingDTOFromModel(row), nil
}

// UpsertByKeyWithAudit upserts a setting and records the changed_by field for auditing.
func (r *AdminSettingBunRepository) UpsertByKeyWithAudit(ctx context.Context, key string, value map[string]any, changedBy string) (dto.AdminSetting, error) {
	var existing models.AdminSetting
	err := r.db.NewSelect().Model(&existing).Where("key = ?", key).Scan(ctx)
	if err == nil {
		now := time.Now()
		existing.Value = value
		existing.ChangedBy = &changedBy
		existing.UpdatedAt = now
		if _, updErr := r.db.NewUpdate().Model(&existing).WherePK().Exec(ctx); updErr != nil {
			return dto.AdminSetting{}, updErr
		}
		return toAdminSettingDTOFromModel(existing), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return dto.AdminSetting{}, err
	}

	now := time.Now()
	row := models.AdminSetting{
		ID:        uuid.New(),
		Key:       key,
		Value:     value,
		ChangedBy: &changedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, insertErr := r.db.NewInsert().Model(&row).Exec(ctx); insertErr != nil {
		return dto.AdminSetting{}, insertErr
	}
	return toAdminSettingDTOFromModel(row), nil
}

func toAdminSettingDTOFromModel(s models.AdminSetting) dto.AdminSetting {
	d := dto.AdminSetting{
		ID:        s.ID,
		Key:       s.Key,
		Value:     s.Value,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.ChangedBy != nil {
		d.ChangedBy = *s.ChangedBy
	}
	return d
}
