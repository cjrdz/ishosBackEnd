package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Flavor holds the schema definition for the Flavor entity.
type Flavor struct {
	ent.Schema
}

func (Flavor) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(100).
			Unique(),
		field.Int("display_order").
			Default(0),
		field.Bool("is_seasonal").
			Default(false),
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

func (Flavor) Edges() []ent.Edge {
	return []ent.Edge{
		// Flavor can be linked to many products
		edge.To("products", Product.Type),
	}
}

func (Flavor) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("display_order", "is_active"),
	}
}
