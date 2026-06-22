// Package middleware contains Chi-compatible middleware functions.
package middleware

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// Auth is the Chi middleware that validates sessions with the auth service.
//
// Flow:
//
//	Request → Auth → reads Bearer token (from cookie or Authorization header)
//	        → calls authService.Validate(token)
//	        → if invalid: returns 401
//	        → if valid: extracts role, stores record+role in context
//	        → calls next handler
//
// Public routes (e.g. GET /api/v1/products, POST /api/v1/orders)
// should NOT use this middleware — anyone can access them.
//
// Protected routes (admin/employee panels) must be wrapped with this.
type Auth struct {
	authService auth.Service
}

func NewAuth(authService auth.Service) *Auth {
	return &Auth{authService: authService}
}

// RequireSession middleware: validates Bearer token, 401 if missing/invalid.
// Use this for API routes consumed by the Astro frontend via fetch().
func (a *Auth) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		requestID := RequestID(r.Context())

		if token == "" {
			log.Warn().Str("request_id", requestID).Msg("auth: missing or malformed Authorization header")
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		record, err := a.authService.Validate(r.Context(), token)
		if err != nil {
			status := apperror.StatusCode(err)
			response.Error(w, status, err.Error())
			log.Warn().Str("request_id", requestID).Err(err).Msg("auth: validation failed")
			return
		}

		// Extract role from the employee record and store in context.
		role := auth.RoleFromRecord(record)
		ctx := auth.WithRecord(r.Context(), record)
		ctx = auth.WithRole(ctx, role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalSession middleware: validates Bearer token if present,
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
func (a *Auth) OptionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)

		// No token provided - continue as public request
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Token provided - try to validate it
		record, err := a.authService.Validate(r.Context(), token)
		if err != nil {
			// Invalid token - log but continue as public request
			log.Debug().Err(err).Msg("auth: invalid token in optional auth, continuing as public")
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

// extractBearerToken reads the token from either:
// 1. HttpOnly cookie named "auth_token" (preferred - XSS safe)
// 2. Authorization header with "Bearer <token>" format (fallback)
func extractBearerToken(r *http.Request) string {
	// First try to get token from HttpOnly cookie (preferred)
	cookie, err := r.Cookie("auth_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Fallback to Authorization header
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
