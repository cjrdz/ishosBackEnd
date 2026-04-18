// admin_setting_repo.go — Ent-backed repository for key/value admin settings stored as JSONB.
package repository

import (
	"context"

	"github.com/cjrdz/ishosBackEnd/ent"
	entAdminSetting "github.com/cjrdz/ishosBackEnd/ent/adminsetting"
)

type AdminSettingRepository struct {
	db *ent.Client
}

func NewAdminSettingRepository(db *ent.Client) *AdminSettingRepository {
	return &AdminSettingRepository{db: db}
}

func (r *AdminSettingRepository) GetByKey(ctx context.Context, key string) (*ent.AdminSetting, error) {
	return r.db.AdminSetting.Query().
		Where(entAdminSetting.KeyEQ(key)).
		Only(ctx)
}

func (r *AdminSettingRepository) UpsertByKey(ctx context.Context, key string, value map[string]any) (*ent.AdminSetting, error) {
	existing, err := r.GetByKey(ctx, key)
	if err == nil {
		return r.db.AdminSetting.UpdateOneID(existing.ID).
			SetValue(value).
			Save(ctx)
	}
	// Any error other than NotFound is unexpected; surface it rather than creating a duplicate.
	if !ent.IsNotFound(err) {
		return nil, err
	}

	return r.db.AdminSetting.Create().
		SetKey(key).
		SetValue(value).
		Save(ctx)
}
