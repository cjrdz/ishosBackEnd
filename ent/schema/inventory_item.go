package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// InventoryItem holds the schema definition for tracked stock items.
type InventoryItem struct {
	ent.Schema
}

func (InventoryItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(200).
			Unique(),
		field.Enum("type").
			Values("ball_based", "unit_based").
			Default("ball_based"),
		field.Int("current_stock").
			Default(0).
			StructTag("json:\"current_stock\""),
		field.Int("low_stock_threshold").
			Default(0).
			StructTag("json:\"low_stock_threshold\""),
		field.UUID("product_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("flavor_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (InventoryItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("stock_movements", StockMovement.Type),
		// Optional: link to a product for unit-based items
		edge.From("product", Product.Type).
			Ref("inventory_item").
			Unique().
			Field("product_id"),
		// Optional: link to a flavor for ball-based items
		edge.From("flavor", Flavor.Type).
			Ref("inventory_item").
			Unique().
			Field("flavor_id"),
	}
}

func (InventoryItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type"),
		index.Fields("product_id"),
		index.Fields("flavor_id"),
	}
}
