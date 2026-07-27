// login_ratelimit.go — Distributed login rate limiting and lockout stores.
//
// Both stores prefer Redis when REDIS_URL is available and fall back to
// in-process maps so a single-instance deployment keeps working.
package auth

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const loginStoreTimeout = 2 * time.Second

// loginLimitStore tracks login attempts per IP over a sliding window.
type loginLimitStore interface {
	// allow returns true when ip has fewer than maxPerMin attempts in the last
	// minute. A successful attempt is also recorded.
	allow(ctx context.Context, ip string, maxPerMin int) (bool, error)
}

// loginLockoutStore tracks failed password attempts per identifier and can
// temporarily lock the identifier out.
type loginLockoutStore interface {
	// recordFailure increments the failure counter and returns the current
	// lockout state for identifier.
	recordFailure(ctx context.Context, identifier string, maxFailures, windowMins, durationMins int) (locked bool, retryAfter time.Duration, err error)
	// getLockout reports whether identifier is currently locked and for how long.
	getLockout(ctx context.Context, identifier string) (locked bool, retryAfter time.Duration, err error)
	// resetLockout clears any lockout for identifier.
	resetLockout(ctx context.Context, identifier string) error
}

// ── In-memory implementations ─────────────────────────────────────────

type memoryLoginLimitStore struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newMemoryLoginLimitStore() *memoryLoginLimitStore {
	return &memoryLoginLimitStore{attempts: make(map[string][]time.Time)}
}

func (s *memoryLoginLimitStore) allow(_ context.Context, ip string, maxPerMin int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	recent := s.attempts[ip][:0]
	for _, t := range s.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= maxPerMin {
		s.attempts[ip] = recent
		return false, nil
	}

	s.attempts[ip] = append(recent, now)
	return true, nil
}

type memoryLoginLockoutStore struct {
	mu       sync.Mutex
	lockouts map[string]lockout
}

type lockout struct {
	FailureCount int
	FirstFailure time.Time
	LastFailure  time.Time
	LockedUntil  time.Time
}

func newMemoryLoginLockoutStore() *memoryLoginLockoutStore {
	return &memoryLoginLockoutStore{lockouts: make(map[string]lockout)}
}

func (s *memoryLoginLockoutStore) recordFailure(_ context.Context, identifier string, maxFailures, windowMins, durationMins int) (bool, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	window := time.Duration(windowMins) * time.Minute
	duration := time.Duration(durationMins) * time.Minute

	state, exists := s.lockouts[identifier]
	if !exists || now.After(state.FirstFailure.Add(window)) {
		state = lockout{FailureCount: 1, FirstFailure: now, LastFailure: now}
	} else {
		state.FailureCount++
		state.LastFailure = now
		if state.FailureCount >= maxFailures {
			state.LockedUntil = now.Add(duration)
		}
	}

	s.lockouts[identifier] = state
	if !state.LockedUntil.IsZero() && now.Before(state.LockedUntil) {
		return true, time.Until(state.LockedUntil), nil
	}
	return false, 0, nil
}

func (s *memoryLoginLockoutStore) getLockout(_ context.Context, identifier string) (bool, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.lockouts[identifier]
	if !exists || state.LockedUntil.IsZero() {
		return false, 0, nil
	}

	now := time.Now()
	if now.Before(state.LockedUntil) {
		return true, state.LockedUntil.Sub(now), nil
	}
	return false, 0, nil
}

func (s *memoryLoginLockoutStore) resetLockout(_ context.Context, identifier string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.lockouts, identifier)
	return nil
}

// ── Redis implementations ─────────────────────────────────────────────

type redisLoginLimitStore struct {
	client *redis.Client
	prefix string
}

func newRedisLoginLimitStore(redisURL, prefix string) (*redisLoginLimitStore, error) {
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

	return &redisLoginLimitStore{client: client, prefix: strings.TrimSpace(prefix)}, nil
}

func (s *redisLoginLimitStore) key(ip string) string {
	if s.prefix != "" {
		return s.prefix + ":login:limit:" + ip
	}
	return "login:limit:" + ip
}

// loginLimitScript trims entries older than one minute, counts the window, and
// records the current attempt atomically. Returns allowed (1/0) and count.
var loginLimitScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local max = tonumber(ARGV[3])
local member = ARGV[4]

redis.call("ZREMRANGEBYSCORE", key, "-inf", now - window)
local count = redis.call("ZCARD", key)
if count >= max then
  redis.call("PEXPIRE", key, window)
  return {0, count}
end

redis.call("ZADD", key, now, member)
redis.call("PEXPIRE", key, window)
return {1, count + 1}
`)

func (s *redisLoginLimitStore) allow(ctx context.Context, ip string, maxPerMin int) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, loginStoreTimeout)
	defer cancel()

	now := time.Now().UnixMilli()
	member := strconv.FormatInt(time.Now().UnixNano(), 10)
	res, err := loginLimitScript.Run(ctx, s.client, []string{s.key(ip)}, now, time.Minute.Milliseconds(), maxPerMin, member).Result()
	if err != nil {
		return false, err
	}
	vals := res.([]interface{})
	return vals[0].(int64) == 1, nil
}

type redisLoginLockoutStore struct {
	client *redis.Client
	prefix string
}

func newRedisLoginLockoutStore(redisURL, prefix string) (*redisLoginLockoutStore, error) {
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

	return &redisLoginLockoutStore{client: client, prefix: strings.TrimSpace(prefix)}, nil
}

func (s *redisLoginLockoutStore) key(identifier string) string {
	if s.prefix != "" {
		return s.prefix + ":login:lockout:" + identifier
	}
	return "login:lockout:" + identifier
}

// loginLockoutScript increments failures within a window, sets locked_until when
// maxFailures is reached, and returns locked (1/0) and retry-after millis.
var loginLockoutScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local maxFailures = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local duration = tonumber(ARGV[4])

local first = redis.call("HGET", key, "first_failure")
first = first and tonumber(first) or now

if now - first > window then
  redis.call("DEL", key)
  first = now
end

redis.call("HSET", key, "first_failure", first)
local count = redis.call("HINCRBY", key, "failure_count", 1)
redis.call("HSET", key, "last_failure", now)

if count >= maxFailures then
  local lockedUntil = now + duration
  redis.call("HSET", key, "locked_until", lockedUntil)
  redis.call("PEXPIRE", key, duration)
  return {1, duration}
end

redis.call("PEXPIRE", key, window)
return {0, 0}
`)

func (s *redisLoginLockoutStore) recordFailure(ctx context.Context, identifier string, maxFailures, windowMins, durationMins int) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, loginStoreTimeout)
	defer cancel()

	now := time.Now().UnixMilli()
	windowMillis := int64(windowMins) * time.Minute.Milliseconds()
	durationMillis := int64(durationMins) * time.Minute.Milliseconds()
	res, err := loginLockoutScript.Run(ctx, s.client, []string{s.key(identifier)}, now, maxFailures, windowMillis, durationMillis).Result()
	if err != nil {
		return false, 0, err
	}
	vals := res.([]interface{})
	locked := vals[0].(int64) == 1
	retryAfter := time.Duration(vals[1].(int64)) * time.Millisecond
	return locked, retryAfter, nil
}

// loginLockoutGetScript returns locked (1/0) and retry-after millis from an
// existing lockout, or 0 when not locked.
var loginLockoutGetScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local lockedUntil = redis.call("HGET", key, "locked_until")
lockedUntil = lockedUntil and tonumber(lockedUntil) or 0

if lockedUntil > now then
  return {1, lockedUntil - now}
end
return {0, 0}
`)

func (s *redisLoginLockoutStore) getLockout(ctx context.Context, identifier string) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, loginStoreTimeout)
	defer cancel()

	now := time.Now().UnixMilli()
	res, err := loginLockoutGetScript.Run(ctx, s.client, []string{s.key(identifier)}, now).Result()
	if err != nil {
		return false, 0, err
	}
	vals := res.([]interface{})
	locked := vals[0].(int64) == 1
	retryAfter := time.Duration(vals[1].(int64)) * time.Millisecond
	return locked, retryAfter, nil
}

func (s *redisLoginLockoutStore) resetLockout(ctx context.Context, identifier string) error {
	ctx, cancel := context.WithTimeout(ctx, loginStoreTimeout)
	defer cancel()
	return s.client.Del(ctx, s.key(identifier)).Err()
}
