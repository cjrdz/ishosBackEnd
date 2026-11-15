package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// InactivityTracker records the last activity time per bearer token.
// It is intentionally simple (in-memory, per-process) for Phase 2a and can be
// replaced with a distributed store once multi-instance coordination is needed.
type InactivityTracker struct {
	mu       sync.RWMutex
	activity map[string]time.Time
}

// NewInactivityTracker creates an empty activity tracker.
func NewInactivityTracker() *InactivityTracker {
	return &InactivityTracker{
		activity: make(map[string]time.Time),
	}
}

// Record marks the token as active now.
func (t *InactivityTracker) Record(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.activity[token] = time.Now()
}

// Reset removes the token from tracking (e.g. after logout).
func (t *InactivityTracker) Reset(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.activity, token)
}

// LastSeen returns the last activity time and whether the token was tracked.
func (t *InactivityTracker) LastSeen(token string) (time.Time, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	last, ok := t.activity[token]
	return last, ok
}

// InactivityMiddleware enforces a per-token inactivity timeout. It must run
// after the session is validated (e.g. after Auth.RequireSession) so the token
// is trusted and the request is authenticated.
type InactivityMiddleware struct {
	tracker     *InactivityTracker
	settingsSvc *services.AdminSettingService
	defaults    services.TokenTTLConfig
}

// NewInactivityMiddleware builds the middleware with the given tracker and
// settings service. The defaults are used when no persisted config exists.
func NewInactivityMiddleware(tracker *InactivityTracker, settingsSvc *services.AdminSettingService, defaults services.TokenTTLConfig) *InactivityMiddleware {
	return &InactivityMiddleware{
		tracker:     tracker,
		settingsSvc: settingsSvc,
		defaults:    defaults,
	}
}

// Middleware wraps handlers that require an active session.
func (m *InactivityMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			// No token means this route is not authenticated; let it through.
			next.ServeHTTP(w, r)
			return
		}
		// Only track tokens the session middleware accepted, so routes with an
		// optional session cannot be used to fill the tracker with junk tokens.
		if record, ok := auth.GetRecord(r.Context()); !ok || record == nil {
			next.ServeHTTP(w, r)
			return
		}

		timeout := m.timeout(r.Context())
		if timeout <= 0 {
			timeout = 15 * time.Minute
		}

		last, tracked := m.tracker.LastSeen(token)
		if tracked && time.Since(last) > timeout {
			log.Warn().
				Str("client_ip", auth.ClientIP(r)).
				Msg("session expired due to inactivity")
			auth.RevokeToken(token)
			m.tracker.Reset(token)
			response.WriteOrError(w, nil, &apperror.UnauthorizedError{Reason: "Tu sesión se cerró por inactividad. Inicia sesión de nuevo."})
			return
		}

		m.tracker.Record(token)
		next.ServeHTTP(w, r)
	})
}

func (m *InactivityMiddleware) timeout(ctx context.Context) time.Duration {
	if m.settingsSvc == nil {
		return time.Duration(m.defaults.InactivityLogoutSeconds) * time.Second
	}
	cfg, err := m.settingsSvc.GetTokenTTLConfig(ctx, m.defaults)
	if err != nil {
		return time.Duration(m.defaults.InactivityLogoutSeconds) * time.Second
	}
	return time.Duration(cfg.InactivityLogoutSeconds) * time.Second
}
