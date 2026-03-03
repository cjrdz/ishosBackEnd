// auth.go — Authentication handlers (Chi + PocketBase edition).
//
// How authentication works end-to-end:
//
//  1. Employee logs in via POST /api/v1/auth/login
//     Body: { "email": "admin@ishos.com", "password": "secret" }
//     → This handler validates credentials via PocketBase's Go API.
//     → Returns a PocketBase-compatible JWT token + employee data.
//
//  2. Frontend stores the JWT token.
//
//  3. Frontend sends requests with Authorization: Bearer <token>.
//
//  4. Protected routes use the PBAuth Chi middleware which validates
//     the token via PocketBase's Go API (no HTTP calls) and stores
//     the employee record and role in the request context.
//
//  5. GetSession reads the employee info from request context.

package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosFactory-backend/internal/auth"
	"github.com/cjrdz/ishosFactory-backend/internal/validator"
	"github.com/cjrdz/ishosFactory-backend/pkg/response"
)

// AuthHandler provides login, session inspection and logout endpoints.
type AuthHandler struct {
	pb *auth.PBAuth
}

func NewAuthHandler(pb *auth.PBAuth) *AuthHandler {
	return &AuthHandler{pb: pb}
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

// Logout is a helper endpoint. With PocketBase token-based auth the frontend
// simply discards the JWT — there is no server-side session to destroy.
//
// This endpoint exists for frontend convenience and returns a confirmation
// message. If needed you can extend it to add the token to a deny-list.
//
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully. Discard the token on the client.",
	})
}
