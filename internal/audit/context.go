// context.go — Context helpers for passing audit metadata through the request chain.
package audit

import (
	"context"
	"net/http"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
)

type contextKey string

const (
	ipContextKey    contextKey = "audit_client_ip"
	actorContextKey contextKey = "audit_actor_id"
)

// WithIP stores the client IP address in the context for audit logging.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipContextKey, ip)
}

// IPFromContext retrieves the client IP from the context.
// Returns empty string if not set.
func IPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(ipContextKey).(string)
	return ip
}

// ContextFromRequest wraps a request context with client IP and optional actor ID for audit logging.
func ContextFromRequest(r *http.Request, actorID string) context.Context {
	ctx := WithIP(r.Context(), auth.ClientIP(r))
	if actorID != "" {
		ctx = WithActor(ctx, actorID)
	}
	return ctx
}

// WithActor stores the actor ID in the context for audit logging.
func WithActor(ctx context.Context, actorID string) context.Context {
	return context.WithValue(ctx, actorContextKey, actorID)
}

// ActorFromContext retrieves the actor ID from the context.
// Returns empty string if not set.
func ActorFromContext(ctx context.Context) string {
	actor, _ := ctx.Value(actorContextKey).(string)
	return actor
}
