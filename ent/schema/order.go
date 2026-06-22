package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Order holds the schema definition for the Order entity.
type Order struct {
	ent.Schema
}

func (Order) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// Human-readable order number, e.g. "ORD-20250215-4821"
		field.String("order_number").
			Unique(),

		// Customer info — no account needed, just contact details
		field.String("customer_name").MaxLen(200),
		field.String("customer_phone").MaxLen(20),
		field.String("customer_email").Optional().Nillable(),
		field.String("customer_ip").Optional().Nillable(),

		field.Float("total_amount"),

		// status flow:
		// pendiente_revision → recibida → en_proceso → lista → entregada
		//                   ↘ cancelada (rejected at review)
		field.Enum("status").
			Values(
				"pendiente_revision",
				"recibida",
				"en_proceso",
				"lista",
				"entregada",
				"cancelada",
			).
			Default("pendiente_revision"),

		field.Enum("payment_method").
			Values("efectivo", "tarjeta", "transferencia", "otro").
			Default("efectivo"),

		field.Enum("payment_status").
			Values("pendiente", "confirmado").
			Default("pendiente"),

		field.Enum("order_type").
			Values("en_local", "para_llevar").
			Default("para_llevar"),

		field.Int("table_number").
			Optional().
			Nillable().
			Positive(),

		field.Text("notes").Optional().Nillable(),
		field.Text("rejection_reason").Optional().Nillable(),

		// Who reviewed this order (PocketBase employee record ID)
		field.String("reviewed_by_user_id").
			Optional().
			Nillable().
			StorageKey("reviewed_by_ory_id"), // Keep old column name for DB compatibility

		field.Time("reviewed_at").Optional().Nillable(),

		// If the order was created in-person by an employee (PocketBase employee record ID)
		field.String("created_by_user_id").
			Optional().
			Nillable().
			StorageKey("created_by_ory_id"), // Keep old column name for DB compatibility

		field.Bool("is_archived").Default(false),

		// Tracks when each status was first reached: {"recibida":"2026-06-21T...", ...}
		field.JSON("status_timestamps", map[string]time.Time{}).
			Optional(),

		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Order) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", OrderItem.Type),
	}
}

func (Order) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("customer_phone"),
		index.Fields("created_at"),
		// Composite index for the common list query: filter by status,
		// order by created_at descending.
		index.Fields("status", "created_at"),
	}
}
