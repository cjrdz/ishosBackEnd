package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Addon holds the schema definition for the Addon entity.
type Addon struct {
	ent.Schema
}

func (Addon) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(100).
			Unique(),
		field.Float("price").
			Default(0),
		field.String("group_name").
			Default("extras").
			MaxLen(50),
		field.Int("display_order").
			Default(0),
		field.Bool("is_active").
			Default(true),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Addon) Edges() []ent.Edge {
	return nil
}

func (Addon) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_name", "display_order", "is_active"),
	}
}
