// service.go — Auth service interface and implementation.
//
// This consolidates all authentication concerns into a single Service interface:
// - Login validation and token generation
// - Token validation and claims extraction
// - Token revocation (logout)
// - Employee record management
// - Login rate limiting and lockout
//
// The service wraps PocketBase and provides a clean boundary between the HTTP
// layer and auth logic. This makes it easy to swap PocketBase for a different
// auth provider later without touching middleware or handlers.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/pocketbase/pocketbase/core"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
)

// Service defines the authentication contract.
type Service interface {
	// Authenticate validates credentials and returns a token + employee record.
	// Returns UnauthorizedError if credentials are invalid.
	Authenticate(ctx context.Context, email, password string) (token string, record *core.Record, err error)

	// Validate checks a bearer token and returns the employee record.
	// Returns UnauthorizedError if token is invalid, expired, or revoked.
	Validate(ctx context.Context, token string) (*core.Record, error)

	// Revoke invalidates a token (called on logout).
	Revoke(ctx context.Context, token string) error

	// GetRecord retrieves an employee by ID.
	GetRecord(ctx context.Context, id string) (*core.Record, error)

	// CreateEmployee creates a new employee (admin operation).
	CreateEmployee(ctx context.Context, email, password, name, phone, role string, disabled bool) (*core.Record, error)

	// UpdateEmployee updates an employee (admin operation).
	UpdateEmployee(ctx context.Context, id string, email, password, name, phone, role *string, disabled *bool) (*core.Record, error)

	// DeactivateEmployee disables an account.
	DeactivateEmployee(ctx context.Context, id string) error

	// DeleteEmployee permanently removes an employee.
	DeleteEmployee(ctx context.Context, id string) error

	// ListEmployees returns all employees.
	ListEmployees(ctx context.Context) ([]*core.Record, error)

	// CheckLoginAttempt checks if an IP has exceeded rate limits.
	// Returns nil if the attempt is allowed, RateLimitError if exceeded.
	CheckLoginAttempt(ctx context.Context, ip string) error

	// RecordLoginFailure records a failed login attempt for rate limiting.
	RecordLoginFailure(ctx context.Context, ip string) error

	// IsLoginLocked checks if an identifier (email) is locked out.
	// Returns (locked, retryAfterSeconds, error).
	IsLoginLocked(ctx context.Context, identifier string) (bool, int, error)

	// RecordLoginLockoutFailure records a failed login attempt for lockout tracking.
	RecordLoginLockoutFailure(ctx context.Context, identifier string) error

	// ResetLoginLockout clears the lockout for an identifier.
	ResetLoginLockout(ctx context.Context, identifier string) error
}

// ServiceImpl implements the Service interface using PocketBase.
type ServiceImpl struct {
	pb                *PBAuth
	loginLimiter      *LoginLimitTracker
	loginLockoutStore LoginLockoutStore
	validationCache   *tokenValidationCache
}

// tokenValidationCache caches successful PocketBase token validations for a
// short time to avoid repeated PB round-trips during a page load.
type tokenValidationCache struct {
	mu      sync.RWMutex
	entries map[string]tokenValidationEntry
	ttl     time.Duration
}

type tokenValidationEntry struct {
	record    *core.Record
	validated time.Time
}

func newTokenValidationCache(ttl time.Duration) *tokenValidationCache {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &tokenValidationCache{
		entries: make(map[string]tokenValidationEntry),
		ttl:     ttl,
	}
}

func (c *tokenValidationCache) get(token string) (*core.Record, bool) {
	c.mu.RLock()
	entry, ok := c.entries[token]
	c.mu.RUnlock()
	if !ok || time.Since(entry.validated) > c.ttl {
		return nil, false
	}
	return entry.record, true
}

func (c *tokenValidationCache) set(token string, record *core.Record) {
	c.mu.Lock()
	c.entries[token] = tokenValidationEntry{record: record, validated: time.Now()}
	c.mu.Unlock()
}

func (c *tokenValidationCache) invalidate(token string) {
	c.mu.Lock()
	delete(c.entries, token)
	c.mu.Unlock()
}

// LoginLimitTracker tracks login attempts per IP.
type LoginLimitTracker struct {
	mu           sync.Mutex
	attemptsByIP map[string][]time.Time
	maxPerMin    int
}

// LoginLockoutStore tracks failed logins and lockouts per identifier (IP/email).
type LoginLockoutStore struct {
	mu           sync.Mutex
	lockouts     map[string]Lockout
	maxFailures  int
	windowMins   int
	durationMins int
}

// Lockout represents a failed login attempt tracker.
type Lockout struct {
	FailureCount int
	LastFailure  time.Time
	LockedUntil  time.Time
	FirstFailure time.Time
}

// NewService creates a new auth service.
func NewService(pb *PBAuth, maxLoginAttemptsPerMin, maxFailures, lockoutWindowMins, lockoutDurationMins int) *ServiceImpl {
	if maxLoginAttemptsPerMin <= 0 {
		maxLoginAttemptsPerMin = 10
	}
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if lockoutWindowMins <= 0 {
		lockoutWindowMins = 15
	}
	if lockoutDurationMins <= 0 {
		lockoutDurationMins = 15
	}

	return &ServiceImpl{
		pb: pb,
		loginLimiter: &LoginLimitTracker{
			attemptsByIP: make(map[string][]time.Time),
			maxPerMin:    maxLoginAttemptsPerMin,
		},
		loginLockoutStore: LoginLockoutStore{
			lockouts:     make(map[string]Lockout),
			maxFailures:  maxFailures,
			windowMins:   lockoutWindowMins,
			durationMins: lockoutDurationMins,
		},
		validationCache: newTokenValidationCache(60 * time.Second),
	}
}

// Authenticate validates credentials and issues a token.
func (s *ServiceImpl) Authenticate(ctx context.Context, email, password string) (string, *core.Record, error) {
	token, record, err := s.pb.Login(email, password, time.Duration(s.pb.authTokenTTLSeconds)*time.Second)
	if err != nil {
		return "", nil, &apperror.UnauthorizedError{Reason: "invalid credentials", Cause: err}
	}
	return token, record, nil
}

// Validate checks a token and returns the employee record.
func (s *ServiceImpl) Validate(ctx context.Context, token string) (*core.Record, error) {
	// Check if token is revoked
	if IsTokenRevoked(token) {
		return nil, &apperror.UnauthorizedError{Reason: "token revoked"}
	}

	// Serve from short-lived cache when possible.
	if record, ok := s.validationCache.get(token); ok {
		if !IsDisabled(record) {
			return record, nil
		}
		s.validationCache.invalidate(token)
		return nil, &apperror.UnauthorizedError{Reason: "account disabled"}
	}

	// Validate with PocketBase
	record, err := s.pb.ValidateToken(token)
	if err != nil || record == nil {
		return nil, &apperror.UnauthorizedError{Reason: "token invalid or expired", Cause: err}
	}

	// Check if account is disabled
	if IsDisabled(record) {
		return nil, &apperror.UnauthorizedError{Reason: "account disabled"}
	}

	s.validationCache.set(token, record)
	return record, nil
}

// Revoke invalidates a token.
func (s *ServiceImpl) Revoke(ctx context.Context, token string) error {
	RevokeToken(token)
	s.validationCache.invalidate(token)
	return nil
}

// GetRecord retrieves an employee by ID.
func (s *ServiceImpl) GetRecord(ctx context.Context, id string) (*core.Record, error) {
	record, err := s.pb.FindEmployeeByID(id)
	if err != nil {
		return nil, &apperror.NotFoundError{Resource: "employee", Cause: err}
	}
	return record, nil
}

// CreateEmployee creates a new employee.
func (s *ServiceImpl) CreateEmployee(ctx context.Context, email, password, name, phone, role string, disabled bool) (*core.Record, error) {
	record, err := s.pb.CreateEmployee(email, password, name, phone, role, disabled)
	if err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}
	return record, nil
}

// UpdateEmployee updates an employee.
func (s *ServiceImpl) UpdateEmployee(ctx context.Context, id string, email, password, name, phone, role *string, disabled *bool) (*core.Record, error) {
	record, err := s.pb.UpdateEmployee(id, email, password, name, phone, role, disabled)
	if err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}
	return record, nil
}

// DeactivateEmployee disables an account.
func (s *ServiceImpl) DeactivateEmployee(ctx context.Context, id string) error {
	err := s.pb.DeactivateEmployee(id)
	if err != nil {
		return fmt.Errorf("deactivate employee: %w", err)
	}
	return nil
}

// DeleteEmployee permanently removes an employee.
func (s *ServiceImpl) DeleteEmployee(ctx context.Context, id string) error {
	err := s.pb.DeleteEmployee(id)
	if err != nil {
		return fmt.Errorf("delete employee: %w", err)
	}
	return nil
}

// ListEmployees returns all employees.
func (s *ServiceImpl) ListEmployees(ctx context.Context) ([]*core.Record, error) {
	records, err := s.pb.ListEmployees()
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	return records, nil
}

// CheckLoginAttempt checks if an IP is rate limited.
func (s *ServiceImpl) CheckLoginAttempt(ctx context.Context, ip string) error {
	if !s.loginLimiter.allow(ip) {
		return &apperror.RateLimitError{
			RetryAfter: 60, // 1 minute
			Cause:      fmt.Errorf("login rate limit exceeded"),
		}
	}

	// Check lockout status
	lockout, _ := s.loginLockoutStore.get(ip)
	now := time.Now()

	// If locked, check if lockout period has expired
	if !lockout.LockedUntil.IsZero() && now.Before(lockout.LockedUntil) {
		remainingSecs := int(lockout.LockedUntil.Sub(now).Seconds())
		return &apperror.RateLimitError{
			RetryAfter: remainingSecs,
			Cause:      fmt.Errorf("too many failed attempts, account locked"),
		}
	}

	return nil
}

// RecordLoginFailure records a failed login attempt for lockout tracking.
func (s *ServiceImpl) RecordLoginFailure(ctx context.Context, ip string) error {
	s.loginLimiter.recordAttempt(ip)
	s.loginLockoutStore.recordFailure(ip)
	return nil
}

// IsLoginLocked checks if an identifier is locked out.
func (s *ServiceImpl) IsLoginLocked(ctx context.Context, identifier string) (bool, int, error) {
	lockout, _ := s.loginLockoutStore.get(identifier)
	now := time.Now()
	if !lockout.LockedUntil.IsZero() && now.Before(lockout.LockedUntil) {
		return true, int(lockout.LockedUntil.Sub(now).Seconds()), nil
	}
	return false, 0, nil
}

// RecordLoginLockoutFailure records a failed login attempt for lockout tracking.
func (s *ServiceImpl) RecordLoginLockoutFailure(ctx context.Context, identifier string) error {
	s.loginLockoutStore.recordFailure(identifier)
	return nil
}

// ResetLoginLockout clears the lockout for an identifier.
func (s *ServiceImpl) ResetLoginLockout(ctx context.Context, identifier string) error {
	s.loginLockoutStore.reset(identifier)
	return nil
}

// --- LoginLimitTracker helpers ---

func (lt *LoginLimitTracker) allow(ip string) bool {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	attempts := lt.attemptsByIP[ip]
	recent := make([]time.Time, 0, len(attempts))
	for _, t := range attempts {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= lt.maxPerMin {
		log.Warn().Str("ip", ip).Msg("Login rate limit exceeded")
		lt.attemptsByIP[ip] = recent
		return false
	}

	lt.attemptsByIP[ip] = append(recent, now)
	return true
}

func (lt *LoginLimitTracker) recordAttempt(ip string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	now := time.Now()
	lt.attemptsByIP[ip] = append(lt.attemptsByIP[ip], now)
}

// --- LoginLockoutStore helpers ---

func (ls *LoginLockoutStore) get(id string) (Lockout, bool) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	lockout, exists := ls.lockouts[id]
	return lockout, exists
}

func (ls *LoginLockoutStore) recordFailure(id string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	now := time.Now()
	lockout, exists := ls.lockouts[id]

	if !exists {
		// First failure
		lockout = Lockout{
			FailureCount: 1,
			FirstFailure: now,
			LastFailure:  now,
		}
	} else {
		// Check if window has expired
		windowExpiry := lockout.FirstFailure.Add(time.Duration(ls.windowMins) * time.Minute)
		if now.After(windowExpiry) {
			// Reset counter
			lockout = Lockout{
				FailureCount: 1,
				FirstFailure: now,
				LastFailure:  now,
			}
		} else {
			// Increment counter
			lockout.FailureCount++
			lockout.LastFailure = now

			// Lock if max failures exceeded
			if lockout.FailureCount >= ls.maxFailures {
				lockout.LockedUntil = now.Add(time.Duration(ls.durationMins) * time.Minute)
				log.Warn().Str("identifier", id).Int("failures", lockout.FailureCount).Msg("Login lockout triggered")
			}
		}
	}

	ls.lockouts[id] = lockout
}

func (ls *LoginLockoutStore) reset(id string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	delete(ls.lockouts, id)
}

// ClientIP returns the client IP from the request context, as set by chi's
// ClientIP middleware (e.g. ClientIPFromXFFTrustedProxies).
func ClientIP(r *http.Request) string {
	return middleware.GetClientIP(r.Context())
}
