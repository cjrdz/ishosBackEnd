package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Category holds the schema definition for the Category entity.
type Category struct {
	ent.Schema
}

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(100).
			Unique(),
		field.String("slug").
			MaxLen(100).
			Unique(),
		field.Text("description").
			Optional().
			Default(""),
		field.String("image_path").
			Optional().
			Nillable(),
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

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		// One category has many products
		edge.To("products", Product.Type),
	}
}
