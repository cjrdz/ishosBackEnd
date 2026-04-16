// auth.go — Authentication handlers (Chi + PocketBase edition).
//
// How authentication works end-to-end:
//
//  1. Employee logs in via POST /api/v1/auth/login
//     Body: { "email": "admin@ishos.com", "password": "secret" }
//     → This handler validates credentials via PocketBase's Go API.
//     → Returns a PocketBase-compatible JWT token + employee data.
//     → Sets HttpOnly cookie for secure token storage.
//
//  2. Frontend can use:
//     - HttpOnly cookie (automatic, most secure) — sent with every request
//     - Authorization Bearer header (fallback) — for API clients
//
//  3. Protected routes use the PBAuth Chi middleware which validates
//     the token via:
//     - HttpOnly cookie (preferred)
//     - Authorization: Bearer <token> header (fallback)
//
//  4. Middleware stores the employee record and role in request context.
//
//  5. GetSession reads the employee info from request context.

package handlers

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// AuthHandler provides login, session inspection and logout endpoints.
type AuthHandler struct {
	pb          *auth.PBAuth
	cfg         *config.Config
	settingsSvc *services.AdminSettingService
	defaults    services.TokenTTLConfig
	lockout     *loginLockout
}

func NewAuthHandler(pb *auth.PBAuth, cfg *config.Config, settingsSvc *services.AdminSettingService) *AuthHandler {
	return &AuthHandler{
		pb:          pb,
		cfg:         cfg,
		settingsSvc: settingsSvc,
		defaults: services.TokenTTLConfig{
			AuthCookieTTLHours:    cfg.AuthCookieTTLHours,
			AuthTokenTTLHours:     cfg.AuthTokenTTLHours,
			TrackingTokenTTLHours: cfg.TrackingTokenTTLHours,
		},
		lockout: newLoginLockout(
			cfg.LoginLockoutMaxFailures,
			time.Duration(cfg.LoginLockoutWindowMinutes)*time.Minute,
			time.Duration(cfg.LoginLockoutDurationMinutes)*time.Minute,
		),
	}
}

// Login authenticates an employee via email/password and returns a JWT token.
//
// POST /api/v1/auth/login
// Body: { "email": "...", "password": "..." }
//
// The token is PocketBase-compatible — the same PBAuth middleware that
// validates tokens from PocketBase's built-in endpoints will also
// accept tokens generated here.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ip := authClientIP(r)
	if h.lockout.IsLocked(ip) {
		log.Warn().Str("ip", ip).Msg("Login blocked by lockout")
		response.Error(w, http.StatusTooManyRequests, "Too many failed attempts, try again later")
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateLogin(body.Email, body.Password); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	ttlConfig := h.resolveTokenTTLConfig(r)
	token, record, err := h.pb.Login(body.Email, body.Password, time.Duration(ttlConfig.AuthTokenTTLHours)*time.Hour)
	if err != nil {
		h.lockout.RecordFailure(ip)
		log.Warn().Str("ip", ip).Msg("Login failed: invalid credentials")
		response.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if the account has been disabled.
	if auth.IsDisabled(record) {
		h.lockout.RecordFailure(ip)
		log.Warn().Str("ip", ip).Msg("Login failed: account disabled")
		response.Error(w, http.StatusForbidden, "Account has been deactivated")
		return
	}

	h.lockout.Reset(ip)

	log.Info().Str("email", body.Email).Str("role", auth.RoleFromRecord(record)).Msg("Login successful")

	cookieTTL := ttlConfig.AuthCookieTTLHours
	if cookieTTL <= 0 {
		cookieTTL = 24
	}

	// Set HttpOnly cookie with the auth token
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,                 // Prevent JS access (XSS protection)
		Secure:   true,                 // Always HTTPS only
		SameSite: http.SameSiteLaxMode, // CSRF protection
		MaxAge:   int((time.Duration(cookieTTL) * time.Hour).Seconds()),
	})

	response.JSON(w, http.StatusOK, map[string]any{
		"token": token,
		"employee": map[string]any{
			"id":    auth.RecordID(record),
			"email": record.Email(),
			"name":  record.GetString("name"),
			"phone": record.GetString("phone"),
			"role":  auth.RoleFromRecord(record),
		},
	})
}

func (h *AuthHandler) resolveTokenTTLConfig(r *http.Request) services.TokenTTLConfig {
	if h.settingsSvc == nil {
		return h.defaults
	}

	resolved, err := h.settingsSvc.GetTokenTTLConfig(r.Context(), h.defaults)
	if err != nil {
		return h.defaults
	}

	return resolved
}

// GetSession returns the current authenticated employee's data.
// The route MUST be protected with the PBAuth.RequireSession middleware.
//
// GET /api/v1/auth/session
func (h *AuthHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	record, ok := auth.GetRecord(r.Context())
	if !ok || record == nil {
		response.Error(w, http.StatusUnauthorized, "No active session")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"id":     auth.RecordID(record),
		"email":  record.Email(),
		"name":   record.GetString("name"),
		"phone":  record.GetString("phone"),
		"role":   auth.RoleFromRecord(record),
		"active": !auth.IsDisabled(record),
	})
}

// Logout clears the auth token cookie and returns a confirmation message.
//
// With PocketBase token-based auth the token validity is time-based.
// This endpoint clears the HttpOnly cookie on the client side.
//
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if token := extractAuthToken(r); token != "" {
		auth.RevokeToken(token)
	}

	// Clear the auth_token cookie by setting MaxAge to -1
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // This tells the browser to delete the cookie
	})

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully.",
	})
}

type loginLockout struct {
	mu           sync.Mutex
	failuresByIP map[string][]time.Time
	lockedUntil  map[string]time.Time
	maxFailures  int
	window       time.Duration
	lockoutFor   time.Duration
}

func newLoginLockout(maxFailures int, window, lockoutFor time.Duration) *loginLockout {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	if lockoutFor <= 0 {
		lockoutFor = 15 * time.Minute
	}

	return &loginLockout{
		failuresByIP: make(map[string][]time.Time),
		lockedUntil:  make(map[string]time.Time),
		maxFailures:  maxFailures,
		window:       window,
		lockoutFor:   lockoutFor,
	}
}

func (l *loginLockout) IsLocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	until, ok := l.lockedUntil[ip]
	if !ok {
		return false
	}
	if until.After(now) {
		return true
	}
	delete(l.lockedUntil, ip)
	return false
}

func (l *loginLockout) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	current := l.failuresByIP[ip]
	recent := make([]time.Time, 0, len(current)+1)
	for _, t := range current {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	l.failuresByIP[ip] = recent

	if len(recent) >= l.maxFailures {
		l.lockedUntil[ip] = now.Add(l.lockoutFor)
		l.failuresByIP[ip] = nil
	}
}

func (l *loginLockout) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failuresByIP, ip)
	delete(l.lockedUntil, ip)
}

func authClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func extractAuthToken(r *http.Request) string {
	if cookie, err := r.Cookie("auth_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}

	return ""
}
