// logging.go — Structured logging with slog + request ID context helpers.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"
)

// ctxKey is the type for context keys to avoid collisions.
type ctxKey string

const (
	// requestIDKey stores the request ID in context.
	requestIDKey ctxKey = "request_id"
)

// RequestID retrieves the request ID from context, or returns empty string if not found.
func RequestID(ctx context.Context) string {
	rid, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return rid
}

// withRequestID stores a request ID in the context.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDMiddleware adds a unique request ID to each request for tracing and logging.
// The request ID is stored in context and exposed as X-Request-ID header.
// Used by slog handlers to correlate log lines across a single request.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New().String()
			ctx := withRequestID(r.Context(), requestID)
			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetupSlog initializes the default slog logger with JSON output to stderr.
// Configures appropriate log level and formatting for structured logging.
// Call this once during server startup.
func SetupSlog() {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))
}
