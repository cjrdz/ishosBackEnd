// login_ratelimit.go — IP-based sliding window rate limiter for login endpoints.
package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// LoginRateLimiter limits login attempts per IP within a rolling 1-minute window.
// This helps reduce brute-force attempts against POST /api/v1/auth/login.
type LoginRateLimiter struct {
	mu                 sync.Mutex
	attemptsByIP       map[string][]time.Time
	maxAttemptsPerMins int
}

func NewLoginRateLimiter(maxAttemptsPerMinute int) *LoginRateLimiter {
	if maxAttemptsPerMinute <= 0 {
		maxAttemptsPerMinute = 10
	}

	return &LoginRateLimiter{
		attemptsByIP:       make(map[string][]time.Time),
		maxAttemptsPerMins: maxAttemptsPerMinute,
	}
}

func (rl *LoginRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	attempts := rl.attemptsByIP[ip]
	recent := make([]time.Time, 0, len(attempts))
	for _, t := range attempts {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.maxAttemptsPerMins {
		log.Warn().Str("ip", ip).Msg("Login rate limit exceeded")
		rl.attemptsByIP[ip] = recent
		return false
	}

	rl.attemptsByIP[ip] = append(recent, now)
	return true
}

func (rl *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := loginClientIP(r)
		if !rl.allow(ip) {
			http.Error(w, `{"error":"too many login attempts, try again in a minute"}`,
				http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loginClientIP(r *http.Request) string {
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		return cfIP
	}

	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
