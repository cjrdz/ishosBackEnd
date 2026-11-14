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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase/core"
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
	authService auth.Service
	cfg         *config.Config
	settingsSvc *services.AdminSettingService
	locations   *services.LocationService
	defaults    services.TokenTTLConfig
}

func NewAuthHandler(pb *auth.PBAuth, authService auth.Service, cfg *config.Config, settingsSvc *services.AdminSettingService, locations *services.LocationService) *AuthHandler {
	return &AuthHandler{
		pb:          pb,
		authService: authService,
		cfg:         cfg,
		settingsSvc: settingsSvc,
		locations:   locations,
		defaults: services.TokenTTLConfig{
			AuthCookieTTLHours:    cfg.AuthCookieTTLHours,
			AuthTokenTTLHours:     cfg.AuthTokenTTLHours,
			TrackingTokenTTLHours: cfg.TrackingTokenTTLHours,
		},
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

	identifier := normalizeLockoutIdentifier(body.Email)
	ip := auth.ClientIP(r)
	if locked, retryAfter, err := h.authService.IsLoginLocked(r.Context(), identifier); err == nil && locked {
		if retryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		}
		log.Warn().Str("ip", ip).Str("identifier", identifier).Msg("Login blocked by lockout")
		response.JSON(w, http.StatusTooManyRequests, map[string]any{
			"error":      "Too many failed attempts, try again later",
			"retryAfter": retryAfter,
		})
		return
	}

	ttlConfig := h.resolveTokenTTLConfig(r)
	token, record, err := h.pb.Login(body.Email, body.Password, time.Duration(ttlConfig.AuthTokenTTLHours)*time.Hour)
	if err != nil {
		_ = h.authService.RecordLoginLockoutFailure(r.Context(), identifier)
		log.Warn().Str("ip", ip).Str("identifier", identifier).Msg("Login failed: invalid credentials")
		response.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if the account has been disabled.
	if auth.IsDisabled(record) {
		log.Warn().Str("ip", ip).Str("identifier", identifier).Msg("Login failed: account disabled")
		response.Error(w, http.StatusForbidden, "Account has been deactivated")
		return
	}

	_ = h.authService.ResetLoginLockout(r.Context(), identifier)

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
		"employee": h.sessionJSON(r, record),
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

	response.JSON(w, http.StatusOK, h.sessionJSON(r, record))
}

// sessionJSON describes the signed-in employee, including the location they
// work at (empty for admins, who switch between locations).
func (h *AuthHandler) sessionJSON(r *http.Request, record *core.Record) map[string]any {
	out := map[string]any{
		"id":            auth.RecordID(record),
		"email":         record.Email(),
		"name":          record.GetString("name"),
		"phone":         record.GetString("phone"),
		"role":          auth.RoleFromRecord(record),
		"active":        !auth.IsDisabled(record),
		"location_id":   nil,
		"location_name": nil,
	}
	if raw := auth.LocationIDFromRecord(record); raw != "" && h.locations != nil {
		if id, err := uuid.Parse(raw); err == nil {
			if l, err := h.locations.Get(r.Context(), id); err == nil {
				out["location_id"] = l.ID
				out["location_name"] = l.Name
			}
		}
	}
	return out
}

// Logout validates the current token, revokes it, and clears the auth cookie.
//
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := extractAuthToken(r)
	if token == "" {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if _, err := h.authService.Validate(r.Context(), token); err != nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.authService.Revoke(r.Context(), token); err != nil {
		log.Warn().Err(err).Msg("auth: failed to revoke token during logout")
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

// ResetLoginLockout lets admins clear login lockouts for a specific employee.
//
// POST /api/v1/auth/lockout/reset
// Body: { "employee_id": "pb-record-id" } OR { "email": "employee@ishos.com" }
func (h *AuthHandler) ResetLoginLockout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EmployeeID string `json:"employee_id"`
		Email      string `json:"email"`
	}

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	employeeID := strings.TrimSpace(body.EmployeeID)
	email := normalizeLockoutIdentifier(body.Email)

	if employeeID == "" && email == "" {
		response.Error(w, http.StatusBadRequest, "employee_id or email is required")
		return
	}
	if employeeID != "" && email != "" {
		response.Error(w, http.StatusBadRequest, "Provide either employee_id or email, not both")
		return
	}

	var recordEmail string
	if employeeID != "" {
		record, err := h.pb.FindEmployeeByID(employeeID)
		if err != nil || record == nil {
			response.Error(w, http.StatusNotFound, "Employee not found")
			return
		}
		recordEmail = normalizeLockoutIdentifier(record.Email())
		email = recordEmail
	} else {
		record, err := h.pb.FindEmployeeByEmail(email)
		if err != nil || record == nil {
			response.Error(w, http.StatusNotFound, "Employee not found")
			return
		}
		recordEmail = normalizeLockoutIdentifier(record.Email())
		employeeID = auth.RecordID(record)
		email = recordEmail
	}

	if email == "" {
		response.Error(w, http.StatusBadRequest, "Invalid employee email")
		return
	}

	_ = h.authService.ResetLoginLockout(r.Context(), email)
	response.JSON(w, http.StatusOK, map[string]any{
		"message":     "Login lockout reset successfully",
		"scope":       "employee",
		"employee_id": employeeID,
		"email":       email,
	})
}

func normalizeLockoutIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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
