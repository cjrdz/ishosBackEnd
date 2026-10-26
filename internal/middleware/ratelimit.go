// ratelimit.go — Phone-first, IP-secondary rate limiting for public order creation
// and tracking, backed by Redis with an in-memory fail-strict fallback.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
)

const (
	defaultTrackingBurst  = 240
	defaultTrackingWindow = time.Hour
	failStrictPhoneBurst  = 1
	failStrictPhoneWindow = time.Hour
	failStrictPhoneBlock  = time.Hour
	failStrictIPBurst     = 1
	failStrictIPWindow    = time.Hour
	failStrictIPBlock     = time.Hour
	failStrictStaffBurst  = 1
	failStrictStaffWindow = time.Hour
	configCacheTTL        = 10 * time.Second
	rateLimitStoreTimeout = 2 * time.Second
)

// RateLimitResult carries the outcome of a single limit check so callers can
// write consistent response headers and decide whether to proceed.
type RateLimitResult struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

// WriteHeaders adds standard rate-limit headers to the response.
func (r *RateLimitResult) WriteHeaders(w http.ResponseWriter) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(r.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(r.Remaining))
	if !r.ResetAt.IsZero() {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(r.ResetAt.Unix(), 10))
	}
	if r.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(r.RetryAfter.Seconds())))
	}
}

// rateLimitStore is the backend contract for sliding-window + block checks.
type rateLimitStore interface {
	// CheckAndRecord evaluates the limit for key. If allowed, the attempt is recorded.
	CheckAndRecord(ctx context.Context, key string, burst int, window, block time.Duration) (RateLimitResult, error)
}

// memoryRateLimitStore is an in-process sliding-window store.
type memoryRateLimitStore struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	blocked  map[string]time.Time
}

func newMemoryRateLimitStore() *memoryRateLimitStore {
	return &memoryRateLimitStore{
		attempts: make(map[string][]time.Time),
		blocked:  make(map[string]time.Time),
	}
}

func (s *memoryRateLimitStore) CheckAndRecord(_ context.Context, key string, burst int, window, block time.Duration) (RateLimitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	if expiry, ok := s.blocked[key]; ok && now.Before(expiry) {
		return RateLimitResult{
			Allowed:    false,
			Limit:      burst,
			Remaining:  0,
			ResetAt:    expiry,
			RetryAfter: time.Until(expiry),
		}, nil
	}
	delete(s.blocked, key)

	cutoff := now.Add(-window)
	recent := s.attempts[key][:0]
	for _, t := range s.attempts[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= burst {
		expiry := now.Add(block)
		s.blocked[key] = expiry
		s.attempts[key] = recent
		return RateLimitResult{
			Allowed:    false,
			Limit:      burst,
			Remaining:  0,
			ResetAt:    expiry,
			RetryAfter: block,
		}, nil
	}

	recent = append(recent, now)
	s.attempts[key] = recent

	return RateLimitResult{
		Allowed:    true,
		Limit:      burst,
		Remaining:  burst - len(recent),
		ResetAt:    now.Add(window),
		RetryAfter: 0,
	}, nil
}

// redisRateLimitStore implements sliding-window + block with Redis.
type redisRateLimitStore struct {
	client *redis.Client
	prefix string
}

// rateLimitScript checks the block flag, trims the window, counts entries, blocks
// if the burst is consumed, and records the attempt atomically.
// Returns: 1 if allowed, 0 if blocked. Also returns the remaining count and reset TTL.
var rateLimitScript = redis.NewScript(`
local windowKey = KEYS[1]
local blockKey = KEYS[2]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local block = tonumber(ARGV[3])
local burst = tonumber(ARGV[4])
local member = ARGV[5]

local blockedTTL = redis.call("PTTL", blockKey)
if blockedTTL > 0 then
  return {0, 0, blockedTTL}
end

local cutoff = now - window
redis.call("ZREMRANGEBYSCORE", windowKey, "-inf", cutoff)
local count = redis.call("ZCARD", windowKey)
if count >= burst then
  redis.call("PSETEX", blockKey, block, "1")
  redis.call("PEXPIRE", windowKey, window)
  return {0, 0, block}
end

redis.call("ZADD", windowKey, now, member)
redis.call("PEXPIRE", windowKey, window)
return {1, burst - count - 1, window}
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

func (s *redisRateLimitStore) CheckAndRecord(ctx context.Context, key string, burst int, window, block time.Duration) (RateLimitResult, error) {
	nowMillis := time.Now().UnixMilli()
	member := strconv.FormatInt(time.Now().UnixNano(), 10)
	windowKey := key + ":window"
	blockKey := key + ":blocked"
	if s.prefix != "" {
		windowKey = s.prefix + ":" + windowKey
		blockKey = s.prefix + ":" + blockKey
	}

	res, err := rateLimitScript.Run(ctx, s.client, []string{windowKey, blockKey}, nowMillis, window.Milliseconds(), block.Milliseconds(), burst, member).Result()
	if err != nil {
		return RateLimitResult{}, err
	}

	vals := res.([]interface{})
	allowed := vals[0].(int64) == 1
	remaining := int(vals[1].(int64))
	resetMillis := vals[2].(int64)

	now := time.Now()
	result := RateLimitResult{
		Allowed:   allowed,
		Limit:     burst,
		Remaining: remaining,
	}
	if allowed {
		result.ResetAt = now.Add(window)
	} else {
		result.RetryAfter = time.Duration(resetMillis) * time.Millisecond
		result.ResetAt = now.Add(result.RetryAfter)
	}
	return result, nil
}

// OrderRateLimiter enforces configurable, phone-first public order limits.
type OrderRateLimiter struct {
	settingsSvc *services.AdminSettingService
	defaults    services.PublicOrderRateLimitConfig
	tracking    services.PublicOrderRateLimitConfig // tracking uses burst/window only
	primary     rateLimitStore
	fallback    rateLimitStore
	prefix      string

	configMu      sync.RWMutex
	cachedConfig  services.PublicOrderRateLimitConfig
	configFetched time.Time
}

// NewOrderRateLimiter builds a rate limiter driven by admin settings.
func NewOrderRateLimiter(settingsSvc *services.AdminSettingService, defaults services.PublicOrderRateLimitConfig, trackingBurst int, trackingWindow time.Duration, redisURL, redisPrefix string) *OrderRateLimiter {
	redisStore, err := newRedisRateLimitStore(redisURL, redisPrefix)
	if err != nil {
		log.Warn().Err(err).Msg("Rate limiter: Redis unavailable, using in-memory fallback")
	}

	var primary rateLimitStore
	if redisStore != nil {
		primary = redisStore
	}

	tracking := services.PublicOrderRateLimitConfig{
		IPBurst:         trackingBurst,
		IPWindowMinutes: int(trackingWindow.Minutes()),
	}
	if tracking.IPBurst <= 0 {
		tracking.IPBurst = defaultTrackingBurst
	}
	if tracking.IPWindowMinutes <= 0 {
		tracking.IPWindowMinutes = int(defaultTrackingWindow.Minutes())
	}

	return &OrderRateLimiter{
		settingsSvc:  settingsSvc,
		defaults:     defaults,
		tracking:     tracking,
		primary:      primary,
		fallback:     newMemoryRateLimitStore(),
		prefix:       strings.TrimSpace(redisPrefix),
		cachedConfig: defaults,
	}
}

func (rl *OrderRateLimiter) resolveConfig(ctx context.Context) services.PublicOrderRateLimitConfig {
	rl.configMu.RLock()
	if time.Since(rl.configFetched) < configCacheTTL {
		cached := rl.cachedConfig
		rl.configMu.RUnlock()
		return cached
	}
	rl.configMu.RUnlock()

	cfg := rl.defaults
	if rl.settingsSvc != nil {
		resolved, err := rl.settingsSvc.GetPublicOrderRateLimitConfig(ctx, rl.defaults)
		if err == nil {
			cfg = resolved
		} else {
			log.Warn().Err(err).Msg("Rate limiter: failed to load config from settings, using defaults")
		}
	}

	rl.configMu.Lock()
	rl.cachedConfig = cfg
	rl.configFetched = time.Now()
	rl.configMu.Unlock()
	return cfg
}

func (rl *OrderRateLimiter) applyPrefix(key string) string {
	if rl.prefix != "" {
		return rl.prefix + ":" + key
	}
	return key
}

func (rl *OrderRateLimiter) check(ctx context.Context, key string, burst int, window, block time.Duration) RateLimitResult {
	ctx, cancel := context.WithTimeout(ctx, rateLimitStoreTimeout)
	defer cancel()

	if rl.primary != nil {
		res, err := rl.primary.CheckAndRecord(ctx, key, burst, window, block)
		if err == nil {
			return res
		}
		log.Error().Err(err).Str("key", key).Msg("Rate limiter: primary store failed, using fail-strict fallback")
	}

	res, err := rl.fallback.CheckAndRecord(ctx, key, burst, window, block)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Rate limiter: fallback store failed")
		return RateLimitResult{Allowed: false, Limit: burst, Remaining: 0}
	}
	return res
}

func normalizePhone(phone string) string {
	normalized := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if normalized == "" {
		return strings.TrimSpace(phone)
	}
	return normalized
}

// CheckOrderIP checks the IP-level limit before the request body is parsed.
func (rl *OrderRateLimiter) CheckOrderIP(ctx context.Context, ip string) RateLimitResult {
	cfg := rl.resolveConfig(ctx)
	if ip == "" {
		// Unknown client IP: do not lump every such request into one shared
		// bucket (that would block the whole store). The per-phone limit applies.
		return RateLimitResult{Allowed: true, Limit: cfg.IPBurst, Remaining: cfg.IPBurst}
	}
	return rl.check(ctx, rl.applyPrefix("orders:ip:"+ip), cfg.IPBurst, time.Duration(cfg.IPWindowMinutes)*time.Minute, time.Duration(cfg.IPBlockMinutes)*time.Minute)
}

// CheckPhone checks and records the phone-level limit.
func (rl *OrderRateLimiter) CheckPhone(ctx context.Context, phone string) RateLimitResult {
	cfg := rl.resolveConfig(ctx)
	normalized := normalizePhone(phone)
	return rl.check(ctx, rl.applyPrefix("orders:phone:"+normalized), cfg.PhoneBurst, time.Duration(cfg.PhoneWindowMinutes)*time.Minute, time.Duration(cfg.PhoneBlockMinutes)*time.Minute)
}

// CheckStaff checks the finite staff throttle.
func (rl *OrderRateLimiter) CheckStaff(ctx context.Context, staffID string) RateLimitResult {
	cfg := rl.resolveConfig(ctx)
	return rl.check(ctx, rl.applyPrefix("orders:staff:"+staffID), cfg.StaffBurst, time.Duration(cfg.StaffWindowMinutes)*time.Minute, time.Duration(cfg.StaffWindowMinutes)*time.Minute)
}

// CheckTrackingIP checks the public tracking endpoint IP limit.
func (rl *OrderRateLimiter) CheckTrackingIP(ctx context.Context, ip string) RateLimitResult {
	if ip == "" {
		// See CheckOrderIP: avoid a single shared bucket for unknown IPs.
		return RateLimitResult{Allowed: true, Limit: rl.tracking.IPBurst, Remaining: rl.tracking.IPBurst}
	}
	return rl.check(ctx, rl.applyPrefix("tracking:ip:"+ip), rl.tracking.IPBurst, time.Duration(rl.tracking.IPWindowMinutes)*time.Minute, time.Duration(rl.tracking.IPWindowMinutes)*time.Minute)
}

// Middleware enforces IP-based rate limiting on order creation.
func (rl *OrderRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if record, ok := auth.GetRecord(r.Context()); ok && record != nil {
			staffID := auth.RecordID(record)
			res := rl.CheckStaff(r.Context(), staffID)
			res.WriteHeaders(w)
			if !res.Allowed {
				log.Warn().Str("staff_id", staffID).Msg("Staff order rate limit exceeded")
				responseRateLimited(w, res)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		ip := auth.ClientIP(r)
		res := rl.CheckOrderIP(r.Context(), ip)
		res.WriteHeaders(w)
		if !res.Allowed {
			log.Warn().Str("ip", ip).Msg("Order IP rate limit exceeded")
			responseRateLimited(w, res)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CheckAndRecordPhone checks the phone-level limit and records the attempt.
// Call this from the order handler after basic validation passes so invalid
// payloads do not consume the customer's burst quota.
func (rl *OrderRateLimiter) CheckAndRecordPhone(ctx context.Context, phone string) RateLimitResult {
	return rl.CheckPhone(ctx, phone)
}

func responseRateLimited(w http.ResponseWriter, res RateLimitResult) {
	w.Header().Set("Content-Type", "application/json")
	res.WriteHeaders(w)
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"too many requests, try later"}`))
}
