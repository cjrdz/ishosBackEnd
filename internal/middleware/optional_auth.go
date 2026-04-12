package middleware

import "net/http"

// OptionalAuth wraps PBAuth optional session middleware for explicit route usage.
// It allows both anonymous and authenticated requests to continue, enriching
// context only when a valid Bearer token is present.
func OptionalAuth(pb *PBAuth) func(http.Handler) http.Handler {
	return pb.OptionalSession
}
