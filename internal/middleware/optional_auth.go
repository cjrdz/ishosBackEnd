// optional_auth.go — Role-based access control middleware.
package middleware

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// RoleRestrictedHandler restricts access to specific roles.
// Use with auth.GetRole(ctx) inside handlers to enforce role-based access.
func RoleRestrictedHandler(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := auth.GetRole(r.Context())
			if role == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			allowed := false
			for _, allowed_role := range allowedRoles {
				if role == allowed_role {
					allowed = true
					break
				}
			}

			if !allowed {
				response.Error(w, http.StatusForbidden, "insufficient permissions")
				log.Warn().Str("role", role).Strs("allowed_roles", allowedRoles).Msg("access denied: insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
