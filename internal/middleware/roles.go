// roles.go — Chi middlewares that enforce role-based access control after PBAuth.
package middleware

import (
	"net/http"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
)

// RequireAdmin blocks requests from non-admin roles.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := auth.GetRole(r.Context())
		if role != "admin" {
			http.Error(w, `{"error":"forbidden: admin only"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
