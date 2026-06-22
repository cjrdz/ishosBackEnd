package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AuditLog holds the schema definition for the AuditLog entity.
type AuditLog struct {
	ent.Schema
}

// Fields of the AuditLog.
func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// action: "order.created", "order.approved", "settings.updated", etc.
		field.String("action"),

		// actor_id: Employee ID who performed the action
		field.String("actor_id").
			Optional().
			Nillable(),

		// resource: "order:12345", "settings:store_hours", "employee:emp-123"
		field.String("resource"),

		// changes: JSON object with before/after values or action details
		field.JSON("changes", map[string]any{}).
			Optional(),

		// ip: Client IP address where action originated
		field.String("ip").
			Optional().
			Nillable(),

		// created_at: When the action was performed
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Indexes of the AuditLog.
func (AuditLog) Indexes() []ent.Index {
	return []ent.Index{
		// Query by action type and time range
		index.Fields("action", "created_at"),
		// Query by who did what and when
		index.Fields("actor_id", "created_at"),
		// Query audit trail for specific resource
		index.Fields("resource"),
		// Cleanup: find old entries for retention policy
		index.Fields("created_at"),
	}
}
