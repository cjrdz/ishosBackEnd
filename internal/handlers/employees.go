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
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase/core"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// EmployeeHandler manages employee records in the PocketBase "employees"
// auth collection via the PBAuth Go API wrapper.
type EmployeeHandler struct {
	pb        *auth.PBAuth
	auditSvc  audit.Service
	locations *services.LocationService
}

func NewEmployeeHandler(pb *auth.PBAuth, auditSvc audit.Service, locations *services.LocationService) *EmployeeHandler {
	return &EmployeeHandler{pb: pb, auditSvc: auditSvc, locations: locations}
}

// resolveEmployeeLocation enforces "admins have no location; managers and
// employees belong to exactly one existing location" and returns the value to
// store.
func (h *EmployeeHandler) resolveEmployeeLocation(r *http.Request, role, locationID string) (string, error) {
	locationID = strings.TrimSpace(locationID)
	if role == auth.RoleAdmin {
		return "", nil
	}
	invalid := &apperror.ValidationError{Fields: map[string]string{"location_id": "Elige la sucursal donde trabaja"}}
	id, err := uuid.Parse(locationID)
	if err != nil {
		return "", invalid
	}
	if _, err := h.locations.Get(r.Context(), id); err != nil {
		return "", invalid
	}
	return id.String(), nil
}

func employeeJSON(rec *core.Record) map[string]any {
	return map[string]any{
		"id":          auth.RecordID(rec),
		"email":       rec.Email(),
		"name":        rec.GetString("name"),
		"phone":       rec.GetString("phone"),
		"role":        auth.RoleFromRecord(rec),
		"location_id": auth.LocationIDFromRecord(rec),
		"state":       stateFromDisabled(auth.IsDisabled(rec)),
	}
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
		employees = append(employees, employeeJSON(rec))
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
		Email      string `json:"email"`
		Password   string `json:"password"`
		Name       string `json:"name"`
		Phone      string `json:"phone"`
		Role       string `json:"role"`
		LocationID string `json:"location_id"`
		State      string `json:"state"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateEmployeeCreate(body.Email, body.Password, body.Name, body.Phone, body.Role, body.State); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}
	locationID, err := h.resolveEmployeeLocation(r, body.Role, body.LocationID)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	disabled := body.State == "inactive"

	record, err := h.pb.CreateEmployee(body.Email, body.Password, body.Name, body.Phone, body.Role, locationID, disabled)
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
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeAdded, actorID, "employee:"+auth.RecordID(record), map[string]any{
		"email":       record.Email(),
		"name":        record.GetString("name"),
		"role":        auth.RoleFromRecord(record),
		"location_id": auth.LocationIDFromRecord(record),
	}, ip)

	resp := employeeJSON(record)
	resp["message"] = "Employee created. They can log in at the admin panel."
	response.JSON(w, http.StatusCreated, resp)
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
		Email      *string `json:"email"`
		Password   *string `json:"password"`
		Name       *string `json:"name"`
		Phone      *string `json:"phone"`
		Role       *string `json:"role"`
		LocationID *string `json:"location_id"`
		State      *string `json:"state"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateEmployeeUpdate(body.Email, body.Password, body.Name, body.Phone, body.Role, body.State); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	// Validate the resulting role/location pair, not just the changed fields:
	// turning an admin into an employee must come with a location.
	existing, err := h.pb.FindEmployeeByID(id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Employee not found")
		return
	}
	finalRole := auth.RoleFromRecord(existing)
	if body.Role != nil {
		finalRole = strings.TrimSpace(*body.Role)
	}
	finalLocation := auth.LocationIDFromRecord(existing)
	if body.LocationID != nil {
		finalLocation = *body.LocationID
	}
	resolvedLocation, err := h.resolveEmployeeLocation(r, finalRole, finalLocation)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	var disabled *bool
	if body.State != nil {
		value := *body.State == "inactive"
		disabled = &value
	}

	record, err := h.pb.UpdateEmployee(id, body.Email, body.Password, body.Name, body.Phone, body.Role, &resolvedLocation, disabled)
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
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeUpdated, actorID, "employee:"+id, map[string]any{
		"email":       record.Email(),
		"name":        record.GetString("name"),
		"role":        auth.RoleFromRecord(record),
		"location_id": auth.LocationIDFromRecord(record),
	}, ip)

	response.JSON(w, http.StatusOK, employeeJSON(record))
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
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeRemoved, actorID, "employee:"+id, map[string]any{
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
	_ = h.auditSvc.Log(r.Context(), audit.ActionEmployeeRemoved, actorID, "employee:"+id, map[string]any{
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
