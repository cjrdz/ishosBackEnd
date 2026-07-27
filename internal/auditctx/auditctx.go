// Package auditctx carries transactional audit metadata through context.Context
// without creating an import cycle between the audit and repository packages.
package auditctx

import "context"

// LogSpec carries audit metadata that should be written inside the current
// database transaction. Repositories read it from the context and call the
// transactional audit logger with the open transaction.
type LogSpec struct {
	Action  string
	ActorID string
	Changes map[string]any
	IP      string
}

type contextKey string

const logSpecContextKey contextKey = "audit_log_spec"

// WithLogSpec attaches a transactional audit log specification to the context.
func WithLogSpec(ctx context.Context, spec LogSpec) context.Context {
	return context.WithValue(ctx, logSpecContextKey, spec)
}

// LogSpecFromContext retrieves the transactional audit log specification, if any.
func LogSpecFromContext(ctx context.Context) (LogSpec, bool) {
	spec, ok := ctx.Value(logSpecContextKey).(LogSpec)
	return spec, ok
}
