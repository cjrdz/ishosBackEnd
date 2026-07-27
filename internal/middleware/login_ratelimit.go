// login_ratelimit.go — Login attempt rate limiting and lockout middleware.
//
// This middleware wraps login endpoints and checks if the client IP has
// exceeded the rate limit for login attempts. It integrates with the auth
// service for stateful lockout tracking.
package middleware

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// LoginRateLimit wraps the auth service to apply rate limiting to login attempts.
type LoginRateLimit struct {
	authService auth.Service
}

// NewLoginRateLimit creates a new login rate limit middleware.
func NewLoginRateLimit(authService auth.Service) *LoginRateLimit {
	return &LoginRateLimit{authService: authService}
}

// Middleware checks login rate limits before allowing the handler to execute.
func (lrl *LoginRateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)

		// Check if this IP has exceeded rate limits
		err := lrl.authService.CheckLoginAttempt(r.Context(), ip)
		if err != nil {
			status := apperror.StatusCode(err)
			if rlErr, ok := err.(*apperror.RateLimitError); ok {
				retryAfter := rlErr.RetryAfter
				if retryAfter > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				}
				response.JSON(w, status, map[string]any{
					"error":      err.Error(),
					"retryAfter": retryAfter,
				})
				log.Warn().Str("ip", ip).Err(err).Int("retry_after", retryAfter).Msg("login: rate limit exceeded")
				return
			}

			response.Error(w, status, err.Error())
			log.Warn().Str("ip", ip).Err(err).Msg("login: rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractClientIP returns the client IP from the request context.
func extractClientIP(r *http.Request) string {
	return auth.ClientIP(r)
}
