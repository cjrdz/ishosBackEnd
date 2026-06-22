package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel error types for domain errors.
var (
	ErrNotFound      = errors.New("resource not found")
	ErrConflict      = errors.New("conflict")
	ErrForbidden     = errors.New("forbidden")
	ErrValidation    = errors.New("validation failed")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInternal      = errors.New("internal server error")
	ErrRateLimited   = errors.New("rate limited")
	ErrUnavailable   = errors.New("service unavailable")
)

// NotFoundError represents a missing resource.
type NotFoundError struct {
	Resource string
	Cause    error
}

func (e *NotFoundError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s not found: %v", e.Resource, e.Cause)
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// ValidationError represents invalid input data.
type ValidationError struct {
	Fields map[string]string
	Cause  error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation failed: %v", e.Cause)
	}
	return "validation failed"
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

// ConflictError represents a constraint violation.
type ConflictError struct {
	Resource string
	Reason   string
	Cause    error
}

func (e *ConflictError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("conflict: %s (%s)", e.Resource, e.Reason)
	}
	if e.Cause != nil {
		return fmt.Sprintf("conflict: %v", e.Cause)
	}
	return "conflict"
}

func (e *ConflictError) Unwrap() error {
	return ErrConflict
}

// UnauthorizedError represents auth failures (invalid token, expired, etc).
type UnauthorizedError struct {
	Reason string
	Cause  error
}

func (e *UnauthorizedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("unauthorized: %s", e.Reason)
	}
	if e.Cause != nil {
		return fmt.Sprintf("unauthorized: %v", e.Cause)
	}
	return "unauthorized"
}

func (e *UnauthorizedError) Unwrap() error {
	return ErrUnauthorized
}

// ForbiddenError represents insufficient permissions.
type ForbiddenError struct {
	Resource string
	Action   string
	Cause    error
}

func (e *ForbiddenError) Error() string {
	if e.Action != "" {
		return fmt.Sprintf("forbidden: cannot %s %s", e.Action, e.Resource)
	}
	if e.Cause != nil {
		return fmt.Sprintf("forbidden: %v", e.Cause)
	}
	return "forbidden"
}

func (e *ForbiddenError) Unwrap() error {
	return ErrForbidden
}

// RateLimitError represents rate limiting.
type RateLimitError struct {
	RetryAfter int // seconds
	Cause      error
}

func (e *RateLimitError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("rate limited: %v", e.Cause)
	}
	return "rate limited"
}

func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

// ServiceUnavailableError represents temporary service unavailability.
type ServiceUnavailableError struct {
	Message string
	Cause   error
}

func (e *ServiceUnavailableError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return fmt.Sprintf("service unavailable: %v", e.Cause)
	}
	return "service unavailable"
}

func (e *ServiceUnavailableError) Unwrap() error {
	return ErrUnavailable
}

// StatusCode maps typed app errors to HTTP status codes.
// Checks both typed error instances and sentinel error values via errors.Is().
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Check typed error instances first (more specific)
	switch err.(type) {
	case *NotFoundError:
		return http.StatusNotFound
	case *ValidationError:
		return http.StatusBadRequest
	case *ConflictError:
		return http.StatusConflict
	case *UnauthorizedError:
		return http.StatusUnauthorized
	case *ForbiddenError:
		return http.StatusForbidden
	case *RateLimitError:
		return http.StatusTooManyRequests
	case *ServiceUnavailableError:
		return http.StatusServiceUnavailable
	}

	// Check sentinel errors (handles wrapped errors)
	switch {
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
