package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// OrderItem holds the schema definition for each line in an order.
type OrderItem struct {
	ent.Schema
}

func (OrderItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		// Snapshot of product name in case it changes later
		field.String("product_name").MaxLen(200),
		field.Int("quantity").Positive(),
		field.Float("unit_price"),
		field.Float("subtotal"),
		// Optional JSON for toppings / flavour choices
		field.JSON("customizations", map[string]interface{}{}).
			Optional(),
	}
}

func (OrderItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("order", Order.Type).
			Ref("items").
			Unique().
			Required(),
		edge.From("product", Product.Type).
			Ref("order_items").
			Unique().
			Required(),
	}
}
