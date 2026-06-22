package schema

import (
	"errors"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/config"
)

// AdminSetting stores global admin panel configuration values.
type AdminSetting struct {
	ent.Schema
}

func (AdminSetting) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("key").
			MaxLen(120).
			Unique().
			Validate(func(s string) error {
				if _, ok := config.SettingsRegistry[s]; !ok {
					return errors.New("invalid key: must be in settings registry")
				}
				return nil
			}),
		field.JSON("value", map[string]any{}).
			Default(map[string]any{}),
		field.String("changed_by").Nillable().Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
