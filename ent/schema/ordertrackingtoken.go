// ordertrackingtoken.go — Schema for OrderTrackingToken entity.
// Stores HMAC-signed tracking tokens for order status checks.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OrderTrackingToken holds the schema definition for the OrderTrackingToken entity.
type OrderTrackingToken struct {
	ent.Schema
}

// Fields of the OrderTrackingToken.
func (OrderTrackingToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").
			Unique().
			Immutable().
			NotEmpty().
			Comment("SHA256-HMAC hash of the tracking token"),
		field.String("order_number").
			NotEmpty().
			Comment("Order number for lookups"),
		field.Time("expires_at").
			Default(func() time.Time { return time.Now().UTC().Add(30 * 24 * time.Hour) }).
			Comment("Token expiration time"),
		field.Time("issued_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Comment("Token issuance time"),
		field.Time("revoked_at").
			Optional().
			Comment("Token revocation time (if revoked)"),
	}
}

// Indexes of the OrderTrackingToken.
func (OrderTrackingToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_number"),
		index.Fields("expires_at"),
		index.Fields("revoked_at"),
	}
}
