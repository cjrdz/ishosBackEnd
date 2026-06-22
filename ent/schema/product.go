package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Product holds the schema definition for the Product entity.
type Product struct {
	ent.Schema
}

func (Product) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(200),
		field.Text("description"),
		field.Float("price").
			Positive(),
		// image_path stores ONLY the object path, e.g. "products/chocolate.jpg"
		// The full public URL is built at the handler layer using the configured R2 public base URL.
		field.String("image_path").
			Optional().
			Nillable(),
		field.Bool("is_available").
			Default(true),
		field.Bool("exclude_global_flavors").
			Default(false),
		field.Bool("exclude_global_addons").
			Default(false),
		field.Bool("ball_based").
			Default(false),
		field.Int("ball_quantity").
			Default(0),
		field.Bool("allows_mixed_flavors").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Product) Edges() []ent.Edge {
	return []ent.Edge{
		// Product belongs to one category
		edge.From("category", Category.Type).
			Ref("products").
			Unique().
			Required(),
		// Product can appear in many order items
		edge.To("order_items", OrderItem.Type),
		// Optional: link to inventory item for unit-based stock tracking
		edge.To("inventory_item", InventoryItem.Type),
	}
}

func (Product) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_available"),
	}
}
