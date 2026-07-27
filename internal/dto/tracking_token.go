package dto

import "time"

// TrackingTokenRecord is the stored state of an order tracking token.
type TrackingTokenRecord struct {
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}
