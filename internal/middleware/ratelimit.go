// ratelimit.go — IP- and phone-based rate limiting for order creation and tracking,
// backed by Redis with an in-memory fallback for single-instance deployments.
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
)

type rateLimitStore interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

type memoryRateLimitStore struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newMemoryRateLimitStore() *memoryRateLimitStore {
	return &memoryRateLimitStore{attempts: make(map[string][]time.Time)}
}

func (s *memoryRateLimitStore) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// In-place filter: reusing the backing array avoids allocation.
	// Safe because we only compact leftward, never overwriting ahead of the read cursor.
	recent := s.attempts[key][:0]
	for _, attempt := range s.attempts[key] {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}

	if len(recent) >= limit {
		s.attempts[key] = append([]time.Time(nil), recent...)
		return false, nil
	}

	recent = append(recent, now)
	s.attempts[key] = append([]time.Time(nil), recent...)
	return true, nil
}

type redisRateLimitStore struct {
	client *redis.Client
	prefix string
}

var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
redis.call("ZREMRANGEBYSCORE", key, "-inf", now - window)
local count = redis.call("ZCARD", key)
if count >= limit then
  redis.call("PEXPIRE", key, window)
  return 0
end
redis.call("ZADD", key, now, member)
redis.call("PEXPIRE", key, window)
return 1
`)

func newRedisRateLimitStore(redisURL, prefix string) (*redisRateLimitStore, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		return nil, nil
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &redisRateLimitStore{client: client, prefix: strings.TrimSpace(prefix)}, nil
}

func (s *redisRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	nowMillis := time.Now().UnixMilli()
	member := strconv.FormatInt(time.Now().UnixNano(), 10)
	redisKey := key
	if s.prefix != "" {
		redisKey = s.prefix + ":" + key
	}

	result, err := slidingWindowScript.Run(ctx, s.client, []string{redisKey}, nowMillis, window.Milliseconds(), limit, member).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

// OrderRateLimiter tracks order creation and tracking attempts.
// It prefers Redis for multi-instance deployments and falls back to in-memory limits.
type OrderRateLimiter struct {
	primary            rateLimitStore
	fallback           rateLimitStore
	maxOrdersPerHour   int
	maxOrdersPerDay    int
	maxTrackingPerHour int
}

func NewOrderRateLimiter(maxPerHour, maxPerDay, maxTrackingPerHour int, redisURL, redisPrefix string) *OrderRateLimiter {
	redisStore, err := newRedisRateLimitStore(redisURL, redisPrefix)
	if err != nil {
		log.Warn().Err(err).Msg("Rate limiter: Redis unavailable, using in-memory fallback")
	}

	var primary rateLimitStore
	if redisStore != nil {
		primary = redisStore
	}

	return &OrderRateLimiter{
		primary:            primary,
		fallback:           newMemoryRateLimitStore(),
		maxOrdersPerHour:   maxPerHour,
		maxOrdersPerDay:    maxPerDay,
		maxTrackingPerHour: maxTrackingPerHour,
	}
}

func (rl *OrderRateLimiter) allow(key string, limit int, window time.Duration, logField, logValue, message string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if rl.primary != nil {
		allowed, err := rl.primary.Allow(ctx, key, limit, window)
		if err == nil {
			if !allowed {
				log.Warn().Str(logField, logValue).Msg(message)
			}
			return allowed
		}
		log.Error().Err(err).Str(logField, logValue).Msg("Rate limiter: primary store failed, falling back to memory")
	}

	allowed, err := rl.fallback.Allow(ctx, key, limit, window)
	if err != nil {
		log.Error().Err(err).Str(logField, logValue).Msg("Rate limiter: fallback store failed")
		return true
	}
	if !allowed {
		log.Warn().Str(logField, logValue).Msg(message)
	}
	return allowed
}

// CheckOrderIP returns true when the IP is allowed to create another order.
func (rl *OrderRateLimiter) CheckOrderIP(ip string) bool {
	return rl.allow("orders:ip:"+ip, rl.maxOrdersPerHour, time.Hour, "ip", ip, "Order IP rate limit exceeded")
}

// CheckTrackingIP returns true when the IP is allowed to query order tracking.
func (rl *OrderRateLimiter) CheckTrackingIP(ip string) bool {
	return rl.allow("tracking:ip:"+ip, rl.maxTrackingPerHour, time.Hour, "ip", ip, "Tracking IP rate limit exceeded")
}

// CheckPhone returns true when the phone is allowed to make another order today.
func (rl *OrderRateLimiter) CheckPhone(phone string) bool {
	// Strip non-digits so different phone formats (e.g. "+52 123-456" and "52123456") share the same rate-limit bucket.
	normalizedPhone := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if normalizedPhone == "" {
		normalizedPhone = strings.TrimSpace(phone)
	}
	return rl.allow("orders:phone:"+normalizedPhone, rl.maxOrdersPerDay, 24*time.Hour, "phone", normalizedPhone, "Phone rate limit exceeded")
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
		if !rl.CheckOrderIP(ip) {
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
