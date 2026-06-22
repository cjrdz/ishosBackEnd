// admin_setting_repo.go — Ent-backed repository for key/value admin settings stored as JSONB.
package repository

import (
	"context"

	"github.com/cjrdz/ishosBackEnd/ent"
	entAdminSetting "github.com/cjrdz/ishosBackEnd/ent/adminsetting"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type AdminSettingRepository struct {
	db *ent.Client
}

func NewAdminSettingRepository(db *ent.Client) *AdminSettingRepository {
	return &AdminSettingRepository{db: db}
}

func (r *AdminSettingRepository) GetByKey(ctx context.Context, key string) (dto.AdminSetting, error) {
	row, err := r.db.AdminSetting.Query().
		Where(entAdminSetting.KeyEQ(key)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return dto.AdminSetting{}, &apperror.NotFoundError{Resource: "admin_setting", Cause: err}
		}
		return dto.AdminSetting{}, err
	}
	return toAdminSettingDTO(row), nil
}

// UpsertByKeyWithAudit upserts a setting and records the changed_by field for auditing.
func (r *AdminSettingRepository) UpsertByKeyWithAudit(ctx context.Context, key string, value map[string]any, changedBy string) (dto.AdminSetting, error) {
	existing, err := r.db.AdminSetting.Query().
		Where(entAdminSetting.KeyEQ(key)).
		Only(ctx)
	if err == nil {
		row, updErr := r.db.AdminSetting.UpdateOneID(existing.ID).
			SetValue(value).
			SetChangedBy(changedBy).
			Save(ctx)
		if updErr != nil {
			return dto.AdminSetting{}, updErr
		}
		return toAdminSettingDTO(row), nil
	}
	// Any error other than NotFound is unexpected; surface it rather than creating a duplicate.
	if !ent.IsNotFound(err) {
		return dto.AdminSetting{}, err
	}

	row, err := r.db.AdminSetting.Create().
		SetKey(key).
		SetValue(value).
		SetChangedBy(changedBy).
		Save(ctx)
	if err != nil {
		return dto.AdminSetting{}, err
	}
	return toAdminSettingDTO(row), nil
}

func toAdminSettingDTO(s *ent.AdminSetting) dto.AdminSetting {
	if s == nil {
		return dto.AdminSetting{}
	}
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
