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
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// AuthHandler provides login, session inspection and logout endpoints.
type AuthHandler struct {
	pb  *auth.PBAuth
	cfg *config.Config
}

func NewAuthHandler(pb *auth.PBAuth, cfg *config.Config) *AuthHandler {
	return &AuthHandler{pb: pb, cfg: cfg}
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

	token, record, err := h.pb.Login(body.Email, body.Password)
	if err != nil {
		log.Warn().Str("email", body.Email).Msg("Login failed: invalid credentials")
		response.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if the account has been disabled.
	if auth.IsDisabled(record) {
		log.Warn().Str("email", body.Email).Msg("Login failed: account disabled")
		response.Error(w, http.StatusForbidden, "Account has been deactivated")
		return
	}

	log.Info().Str("email", body.Email).Str("role", auth.RoleFromRecord(record)).Msg("Login successful")

	// Set HttpOnly cookie with the auth token
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,                                // Prevent JS access (XSS protection)
		Secure:   h.cfg.Env == "production",           // HTTPS only in production
		SameSite: http.SameSiteLaxMode,                // CSRF protection
		MaxAge:   int((7 * 24 * time.Hour).Seconds()), // 7 days
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
	// Clear the auth_token cookie by setting MaxAge to -1
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // This tells the browser to delete the cookie
	})

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully.",
	})
}
