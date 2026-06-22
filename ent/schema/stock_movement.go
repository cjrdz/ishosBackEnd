package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// StockMovement holds the schema definition for inventory movement audit log.
type StockMovement struct {
	ent.Schema
}

func (StockMovement) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.Enum("type").
			Values("entry", "sale", "adjustment"),
		field.Int("quantity").
			Default(0),
		field.Int("quantity_containers").
			Optional().
			Nillable(),
		field.UUID("inventory_item_id", uuid.UUID{}),
		field.UUID("container_type_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("flavor_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Text("reason").
			Optional().
			Nillable(),
		field.UUID("order_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("created_by_user_id").
			MaxLen(255).
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (StockMovement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("inventory_item", InventoryItem.Type).
			Ref("stock_movements").
			Unique().
			Required().
			Field("inventory_item_id"),
		edge.From("container_type", ContainerType.Type).
			Ref("stock_movements").
			Unique().
			Field("container_type_id"),
		edge.From("flavor", Flavor.Type).
			Ref("flavor_stock_movements").
			Unique().
			Field("flavor_id"),
	}
}

func (StockMovement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type"),
		index.Fields("inventory_item_id", "created_at"),
		index.Fields("order_id"),
	}
}
