// Package middleware contains Chi-compatible middleware functions.
package middleware

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
)

// PBAuth is the Chi middleware that validates sessions with PocketBase.
//
// Flow:
//
//	Request → PBAuth → reads Authorization: Bearer <token>
//	          → calls PocketBase ValidateToken (no HTTP — direct Go API)
//	          → if invalid: returns 401
//	          → if valid: extracts role, stores record+role in context
//	          → calls next handler
//
// Public routes (e.g. GET /api/v1/products, POST /api/v1/orders)
// should NOT use this middleware — anyone can access them.
//
// Protected routes (admin/employee panels) must be wrapped with this.
type PBAuth struct {
	pb *auth.PBAuth
}

func NewPBAuth(pb *auth.PBAuth) *PBAuth {
	return &PBAuth{pb: pb}
}

// RequireSession middleware: validates PocketBase Bearer token, 401 if missing/invalid.
// Use this for API routes consumed by the Astro frontend via fetch().
func (a *PBAuth) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			log.Warn().Msg("PocketBase auth: missing or malformed Authorization header")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		record, err := a.pb.ValidateToken(token)
		if err != nil || record == nil {
			log.Warn().Err(err).Msg("PocketBase auth: invalid or expired token")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Check if the account has been disabled
		if auth.IsDisabled(record) {
			log.Warn().Str("id", auth.RecordID(record)).Msg("PocketBase auth: account disabled")
			http.Error(w, `{"error":"account disabled"}`, http.StatusForbidden)
			return
		}

		// Extract role from the employee record and store in context.
		role := auth.RoleFromRecord(record)
		ctx := auth.WithRecord(r.Context(), record)
		ctx = auth.WithRole(ctx, role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalSession middleware: validates PocketBase Bearer token if present,
// but allows the request to continue even if no token is provided.
// Use this for endpoints that can be accessed by both public users and authenticated staff.
//
// If token is present and valid:
//   - Record and role are stored in context
//   - Handler can check auth.GetRecord() to determine if user is authenticated
//
// If token is missing or invalid:
//   - Request continues without authentication context
//   - Handler treats request as public/anonymous
func (a *PBAuth) OptionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)

		// No token provided - continue as public request
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Token provided - try to validate it
		record, err := a.pb.ValidateToken(token)
		if err != nil || record == nil {
			// Invalid token - log but continue as public request
			log.Debug().Err(err).Msg("PocketBase auth: invalid token in optional auth, continuing as public")
			next.ServeHTTP(w, r)
			return
		}

		// Check if the account has been disabled
		if auth.IsDisabled(record) {
			log.Debug().Str("id", auth.RecordID(record)).Msg("PocketBase auth: disabled account in optional auth, continuing as public")
			next.ServeHTTP(w, r)
			return
		}

		// Valid token - store record and role in context
		role := auth.RoleFromRecord(record)
		ctx := auth.WithRecord(r.Context(), record)
		ctx = auth.WithRole(ctx, role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken reads the token from the Authorization header.
// Expected format: "Bearer <token>"
func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
