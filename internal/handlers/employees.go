// employees.go — Admin manages employee accounts via PocketBase auth collection.
//
// Employees are stored in the PocketBase "employees" auth collection (not in Neon).
// PocketBase handles password hashing, token generation, and session management.
//
// Why store employees in PocketBase, not Neon?
//   - PocketBase handles password hashing, recovery emails, MFA.
//   - We never touch plaintext passwords.
//   - Admin can deactivate accounts without deleting data.
//   - Token-based auth is fully handled by PocketBase.
//
// All handlers use standard Chi signatures: func(w http.ResponseWriter, r *http.Request).
// The PBAuth wrapper is used to interact with PocketBase's Go API directly —
// no HTTP calls are made.

package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// EmployeeHandler manages employee records in the PocketBase "employees"
// auth collection via the PBAuth Go API wrapper.
type EmployeeHandler struct {
	pb       *auth.PBAuth
	auditSvc audit.Service
}

func NewEmployeeHandler(pb *auth.PBAuth, auditSvc audit.Service) *EmployeeHandler {
	return &EmployeeHandler{pb: pb, auditSvc: auditSvc}
}

// List fetches all employee/admin records from the PocketBase auth collection.
//
// GET /api/v1/employees
func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	records, err := h.pb.ListEmployees()
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	employees := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		employees = append(employees, map[string]any{
			"id":    auth.RecordID(rec),
			"email": rec.Email(),
			"name":  rec.GetString("name"),
			"phone": rec.GetString("phone"),
			"role":  auth.RoleFromRecord(rec),
			"state": stateFromDisabled(auth.IsDisabled(rec)),
		})
	}

	response.JSON(w, http.StatusOK, response.NewPaginated(employees, 1, len(employees), len(employees)))
}

// Create registers a new employee in the PocketBase "employees" auth collection.
//
// POST /api/v1/employees
// Body: { "email", "password", "name", "phone", "role": "employee"|"admin" }
//
// PocketBase handles password hashing internally.
// The employee can then log in via POST /api/v1/auth/login.
func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		Role     string `json:"role"`
		State    string `json:"state"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateEmployeeCreate(body.Email, body.Password, body.Name, body.Phone, body.Role, body.State); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	disabled := body.State == "inactive"

	record, err := h.pb.CreateEmployee(body.Email, body.Password, body.Name, body.Phone, body.Role, disabled)
	if err != nil {
		log.Error().Err(err).Str("email", body.Email).Msg("Failed to create employee")
		response.Error(w, http.StatusInternalServerError, "Failed to create employee")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeAdded, actorID, "employee:" + auth.RecordID(record), map[string]any{
		"email": record.Email(),
		"name":  record.GetString("name"),
		"role":  auth.RoleFromRecord(record),
	}, ip)

	response.JSON(w, http.StatusCreated, map[string]any{
		"id":      auth.RecordID(record),
		"email":   record.Email(),
		"name":    record.GetString("name"),
		"phone":   record.GetString("phone"),
		"role":    auth.RoleFromRecord(record),
		"state":   stateFromDisabled(auth.IsDisabled(record)),
		"message": "Employee created. They can log in at the admin panel.",
	})
}

// Update edits an employee account.
//
// PATCH /api/v1/employees/{id}
func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "Employee ID is required")
		return
	}

	var body struct {
		Email    *string `json:"email"`
		Password *string `json:"password"`
		Name     *string `json:"name"`
		Phone    *string `json:"phone"`
		Role     *string `json:"role"`
		State    *string `json:"state"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateEmployeeUpdate(body.Email, body.Password, body.Name, body.Phone, body.Role, body.State); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	var disabled *bool
	if body.State != nil {
		value := *body.State == "inactive"
		disabled = &value
	}

	record, err := h.pb.UpdateEmployee(id, body.Email, body.Password, body.Name, body.Phone, body.Role, disabled)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update employee")
		response.Error(w, http.StatusInternalServerError, "Failed to update employee")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeUpdated, actorID, "employee:" + id, map[string]any{
		"email": record.Email(),
		"name":  record.GetString("name"),
		"role":  auth.RoleFromRecord(record),
	}, ip)

	response.JSON(w, http.StatusOK, map[string]any{
		"id":    auth.RecordID(record),
		"email": record.Email(),
		"name":  record.GetString("name"),
		"phone": record.GetString("phone"),
		"role":  auth.RoleFromRecord(record),
		"state": stateFromDisabled(auth.IsDisabled(record)),
	})
}

// Deactivate disables an employee account (they can no longer log in).
// The record is not deleted — only its "disabled" flag is set.
//
// DELETE /api/v1/employees/{id}
func (h *EmployeeHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "Employee ID is required")
		return
	}

	if err := h.pb.DeactivateEmployee(id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to deactivate employee")
		response.Error(w, http.StatusInternalServerError, "Failed to deactivate employee")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeRemoved, actorID, "employee:" + id, map[string]any{
		"action_detail": "deactivated",
	}, ip)

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Employee deactivated. They can no longer log in.",
	})
}

// Delete permanently removes an employee account.
//
// DELETE /api/v1/employees/{id}
func (h *EmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "Employee ID is required")
		return
	}

	if err := h.pb.DeleteEmployee(id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete employee")
		response.Error(w, http.StatusInternalServerError, "Failed to delete employee")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeRemoved, actorID, "employee:" + id, map[string]any{
		"action_detail": "deleted",
	}, ip)

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Employee deleted permanently.",
	})
}

// stateFromDisabled returns "active" or "inactive" based on the disabled flag.
func stateFromDisabled(disabled bool) string {
	if disabled {
		return "inactive"
	}
	return "active"
}
