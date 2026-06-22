package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ContainerType holds the schema definition for container types (Mediano, Grande, Custom).
type ContainerType struct {
	ent.Schema
}

func (ContainerType) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(100).
			Unique(),
		field.Int("balls_per_container").
			Default(0),
		field.Bool("is_custom").
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

func (ContainerType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("stock_movements", StockMovement.Type),
	}
}

func (ContainerType) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_active"),
	}
}
