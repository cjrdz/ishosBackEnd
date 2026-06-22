package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RevokedToken holds the schema definition for the RevokedToken entity.
// This entity tracks JWT tokens that have been explicitly revoked (e.g., on logout).
type RevokedToken struct {
	ent.Schema
}

// Fields of the RevokedToken.
func (RevokedToken) Fields() []ent.Field {
	return []ent.Field{
		// token_hash: SHA-256 hash of the JWT (never store raw tokens)
		field.String("token_hash").
			Unique(),

		// expires_at: When the token expires (used for cleanup)
		field.Time("expires_at"),

		// revoked_at: When the token was revoked
		field.Time("revoked_at").
			Default(time.Now).
			Immutable(),
	}
}

// Indexes of the RevokedToken.
func (RevokedToken) Indexes() []ent.Index {
	return []ent.Index{
		// Check revocation status: WHERE token_hash = ? AND expires_at > ?
		index.Fields("token_hash", "expires_at"),
		// Cleanup expired tokens: DELETE WHERE expires_at <= NOW()
		index.Fields("expires_at"),
	}
}
