package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Customer holds customer directory records used by the admin panel.
type Customer struct {
	ent.Schema
}

func (Customer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			MaxLen(200),
		field.Enum("user_type").
			Values("user", "company").
			Default("user"),
		field.String("phone").
			MaxLen(20).
			Unique(),
		field.String("email").
			Optional().
			Nillable().
			Unique(),
		field.Enum("status").
			Values("active", "inactive").
			Default("active"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
func (Customer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("orders", Order.Type).
			Ref("customer"),
	}
}

func (Customer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("name"),
	}
}
