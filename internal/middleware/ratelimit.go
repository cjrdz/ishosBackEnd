package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosFactory-backend/internal/auth"
)

// OrderRateLimiter tracks order creation attempts in memory.
// For production with multiple instances, swap the map for Redis.
type OrderRateLimiter struct {
	mu sync.Mutex
	// ip → list of timestamps in the current hour
	ipAttempts map[string][]time.Time
	// phone → list of timestamps in the current day
	phoneAttempts map[string][]time.Time
	maxPerHour    int
	maxPerDay     int
}

func NewOrderRateLimiter(maxPerHour, maxPerDay int) *OrderRateLimiter {
	return &OrderRateLimiter{
		ipAttempts:    make(map[string][]time.Time),
		phoneAttempts: make(map[string][]time.Time),
		maxPerHour:    maxPerHour,
		maxPerDay:     maxPerDay,
	}
}

// CheckIP returns true when the IP is allowed to make another order.
func (rl *OrderRateLimiter) CheckIP(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Hour)

	// Keep only attempts within the last hour
	var recent []time.Time
	for _, t := range rl.ipAttempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.maxPerHour {
		log.Warn().Str("ip", ip).Msg("IP rate limit exceeded")
		return false
	}

	rl.ipAttempts[ip] = append(recent, now)
	return true
}

// CheckPhone returns true when the phone is allowed to make another order today.
func (rl *OrderRateLimiter) CheckPhone(phone string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	var recent []time.Time
	for _, t := range rl.phoneAttempts[phone] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.maxPerDay {
		log.Warn().Str("phone", phone).Msg("Phone rate limit exceeded")
		return false
	}

	rl.phoneAttempts[phone] = append(recent, now)
	return true
}

// Middleware returns a Chi middleware that enforces IP-based rate limiting on order creation.
// Place it only on the POST /api/v1/orders route.
// Note: Phone-based rate limiting is enforced in the handler after parsing the request body.
func (rl *OrderRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if record, ok := auth.GetRecord(r.Context()); ok && record != nil {
			next.ServeHTTP(w, r)
			return
		}
		ip := realIP(r)
		if !rl.CheckIP(ip) {
			http.Error(w, `{"error":"too many orders from your IP, try later"}`,
				http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CheckAndRecordPhone checks if a phone number can make an order and records the attempt.
// This should be called from the order handler after parsing the request body.
func (rl *OrderRateLimiter) CheckAndRecordPhone(phone string) bool {
	return rl.CheckPhone(phone)
}

func realIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
