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
	CreateEmployee(ctx context.Context, email, password, name, phone, role, locationID string, disabled bool) (*core.Record, error)

	// UpdateEmployee updates an employee (admin operation).
	UpdateEmployee(ctx context.Context, id string, email, password, name, phone, role, locationID *string, disabled *bool) (*core.Record, error)

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
	pb                     *PBAuth
	loginLimiter           loginLimitStore
	loginLockout           loginLockoutStore
	fallbackLoginLimiter   loginLimitStore
	fallbackLoginLockout   loginLockoutStore
	maxLoginAttemptsPerMin int
	maxFailures            int
	lockoutWindowMins      int
	lockoutDurationMins    int
	validationCache        *tokenValidationCache
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
		// Short TTL limits how long a role change (e.g. admin -> employee) is
		// delayed in the validation cache.
		ttl = 5 * time.Second
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

// NewService creates a new auth service.
// When redisURL is provided, login rate limiting and lockout are backed by
// Redis so multiple instances share state; otherwise an in-process fallback is
// used.
func NewService(pb *PBAuth, maxLoginAttemptsPerMin, maxFailures, lockoutWindowMins, lockoutDurationMins int, redisURL, redisPrefix string) *ServiceImpl {
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

	fallbackLimiter := newMemoryLoginLimitStore()
	fallbackLockout := newMemoryLoginLockoutStore()

	var limiter loginLimitStore = fallbackLimiter
	var lockout loginLockoutStore = fallbackLockout

	if redisURL != "" {
		if rl, err := newRedisLoginLimitStore(redisURL, redisPrefix); err == nil {
			limiter = rl
		} else {
			log.Warn().Err(err).Msg("auth: Redis login limiter unavailable, using in-memory fallback")
		}
		if ls, err := newRedisLoginLockoutStore(redisURL, redisPrefix); err == nil {
			lockout = ls
		} else {
			log.Warn().Err(err).Msg("auth: Redis login lockout unavailable, using in-memory fallback")
		}
	}

	return &ServiceImpl{
		pb:                     pb,
		loginLimiter:           limiter,
		loginLockout:           lockout,
		fallbackLoginLimiter:   fallbackLimiter,
		fallbackLoginLockout:   fallbackLockout,
		maxLoginAttemptsPerMin: maxLoginAttemptsPerMin,
		maxFailures:            maxFailures,
		lockoutWindowMins:      lockoutWindowMins,
		lockoutDurationMins:    lockoutDurationMins,
		validationCache:        newTokenValidationCache(5 * time.Second),
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
func (s *ServiceImpl) CreateEmployee(ctx context.Context, email, password, name, phone, role, locationID string, disabled bool) (*core.Record, error) {
	record, err := s.pb.CreateEmployee(email, password, name, phone, role, locationID, disabled)
	if err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}
	return record, nil
}

// UpdateEmployee updates an employee.
func (s *ServiceImpl) UpdateEmployee(ctx context.Context, id string, email, password, name, phone, role, locationID *string, disabled *bool) (*core.Record, error) {
	record, err := s.pb.UpdateEmployee(id, email, password, name, phone, role, locationID, disabled)
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

// CheckLoginAttempt checks if an IP is rate limited or locked out.
func (s *ServiceImpl) CheckLoginAttempt(ctx context.Context, ip string) error {
	allowed, err := s.loginLimiter.allow(ctx, ip, s.maxLoginAttemptsPerMin)
	if err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("auth: login limiter store failed, using fallback")
		allowed, err = s.fallbackLoginLimiter.allow(ctx, ip, s.maxLoginAttemptsPerMin)
		if err != nil {
			return fmt.Errorf("login limiter unavailable: %w", err)
		}
	}
	if !allowed {
		log.Warn().Str("ip", ip).Msg("Login rate limit exceeded")
		return &apperror.RateLimitError{
			RetryAfter: 60, // 1 minute
			Cause:      fmt.Errorf("login rate limit exceeded"),
		}
	}

	locked, retryAfter, err := s.loginLockout.getLockout(ctx, ip)
	if err != nil {
		log.Error().Err(err).Str("identifier", ip).Msg("auth: login lockout store failed, using fallback")
		locked, retryAfter, err = s.fallbackLoginLockout.getLockout(ctx, ip)
		if err != nil {
			return fmt.Errorf("login lockout unavailable: %w", err)
		}
	}
	if locked {
		return &apperror.RateLimitError{
			RetryAfter: int(retryAfter.Seconds()),
			Cause:      fmt.Errorf("too many failed attempts, account locked"),
		}
	}

	return nil
}

// RecordLoginFailure records a failed login attempt for lockout tracking.
func (s *ServiceImpl) RecordLoginFailure(ctx context.Context, ip string) error {
	_, _, err := s.loginLockout.recordFailure(ctx, ip, s.maxFailures, s.lockoutWindowMins, s.lockoutDurationMins)
	if err != nil {
		log.Error().Err(err).Str("identifier", ip).Msg("auth: login lockout store failed, using fallback")
		_, _, err = s.fallbackLoginLockout.recordFailure(ctx, ip, s.maxFailures, s.lockoutWindowMins, s.lockoutDurationMins)
	}
	return err
}

// IsLoginLocked checks if an identifier is locked out.
func (s *ServiceImpl) IsLoginLocked(ctx context.Context, identifier string) (bool, int, error) {
	locked, retryAfter, err := s.loginLockout.getLockout(ctx, identifier)
	if err != nil {
		log.Error().Err(err).Str("identifier", identifier).Msg("auth: login lockout store failed, using fallback")
		locked, retryAfter, err = s.fallbackLoginLockout.getLockout(ctx, identifier)
		if err != nil {
			return false, 0, err
		}
	}
	return locked, int(retryAfter.Seconds()), nil
}

// RecordLoginLockoutFailure records a failed login attempt for lockout tracking.
func (s *ServiceImpl) RecordLoginLockoutFailure(ctx context.Context, identifier string) error {
	_, _, err := s.loginLockout.recordFailure(ctx, identifier, s.maxFailures, s.lockoutWindowMins, s.lockoutDurationMins)
	if err != nil {
		log.Error().Err(err).Str("identifier", identifier).Msg("auth: login lockout store failed, using fallback")
		_, _, err = s.fallbackLoginLockout.recordFailure(ctx, identifier, s.maxFailures, s.lockoutWindowMins, s.lockoutDurationMins)
	}
	return err
}

// ResetLoginLockout clears the lockout for an identifier.
func (s *ServiceImpl) ResetLoginLockout(ctx context.Context, identifier string) error {
	err := s.loginLockout.resetLockout(ctx, identifier)
	if err != nil {
		log.Error().Err(err).Str("identifier", identifier).Msg("auth: login lockout store failed, using fallback")
		err = s.fallbackLoginLockout.resetLockout(ctx, identifier)
	}
	return err
}

// ClientIP returns the client IP from the request context, as set by chi's
// ClientIP middleware (e.g. ClientIPFromXFFTrustedProxies).
func ClientIP(r *http.Request) string {
	return middleware.GetClientIP(r.Context())
}
