package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// RateLimit tracks order creation attempts per IP and phone number.
type RateLimit struct {
	ent.Schema
}

func (RateLimit) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("ip_address"),
		field.String("phone_number").Optional(),
		// action_type: "order_creation"
		field.String("action_type").Default("order_creation"),
		field.Int("attempts").Default(1),
		field.Time("last_attempt"),
		field.Time("blocked_until").Optional().Nillable(),
	}
}

func (RateLimit) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ip_address", "action_type"),
		index.Fields("phone_number", "action_type"),
	}
}
