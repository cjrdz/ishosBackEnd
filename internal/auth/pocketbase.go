// Package auth wraps PocketBase as an embedded auth engine for the Chi API.
//
// PocketBase is used as a Go library — NOT as an HTTP server. It provides:
//   - Employee auth collection with email/password authentication
//   - Password hashing and verification
//   - JWT token generation and validation
//   - Employee record management (CRUD)
//
// The Chi API on the main port uses the PBAuth wrapper to validate Bearer
// tokens and manage employee records without making any HTTP calls —
// everything goes through PocketBase's Go API against its embedded SQLite DB.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/rs/zerolog/log"
)

// PBAuth holds the embedded PocketBase instance used for authentication
// and employee management.
type PBAuth struct {
	App                 *pocketbase.PocketBase
	authTokenTTLSeconds int64
}

// New creates and bootstraps a new PocketBase instance.
//
// After calling New() the PocketBase database is ready and the "employees"
// auth collection is guaranteed to exist. The PocketBase HTTP server is NOT
// started — only the Go API is available.
func New(dataDir string, authTokenTTLHours int) (*PBAuth, error) {
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: dataDir,
	})

	// Bootstrap initialises the PocketBase database and internal state
	// without starting the HTTP server.
	if err := app.Bootstrap(); err != nil {
		return nil, fmt.Errorf("pocketbase bootstrap: %w", err)
	}

	ttlSeconds := int64(authTokenTTLHours) * int64(time.Hour/time.Second)
	pb := &PBAuth{App: app, authTokenTTLSeconds: ttlSeconds}

	// Ensure the employees auth collection exists.
	if err := pb.ensureEmployeesCollection(); err != nil {
		return nil, fmt.Errorf("ensure employees collection: %w", err)
	}

	return pb, nil
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// Login validates an employee's email and password, then generates a
// PocketBase-compatible JWT auth token.
//
// This is called from the Chi auth handler (POST /api/v1/auth/login) and
// does NOT make any HTTP requests — everything goes through PocketBase's
// embedded SQLite DB.
func (pb *PBAuth) Login(email, password string, tokenTTL time.Duration) (string, *core.Record, error) {
	record, err := pb.App.FindAuthRecordByEmail("employees", email)
	if err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}

	if !record.ValidatePassword(password) {
		return "", nil, fmt.Errorf("invalid credentials")
	}

	token, err := pb.GenerateAuthToken(record, tokenTTL)
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}

	return token, record, nil
}

// GenerateAuthToken creates a PocketBase-compatible JWT auth token for the
// given record. It delegates to PocketBase's built-in Record.NewAuthToken()
// which handles signing, claims and duration automatically.
//
// The generated token is fully compatible with ValidateToken / FindAuthRecordByToken.
func (pb *PBAuth) GenerateAuthToken(record *core.Record, tokenTTL time.Duration) (string, error) {
	var (
		token string
		err   error
	)

	if tokenTTL > 0 {
		token, err = record.NewStaticAuthToken(tokenTTL)
	} else {
		token, err = record.NewAuthToken()
	}
	if err != nil {
		return "", fmt.Errorf("create auth token: %w", err)
	}
	return token, nil
}

// ValidateToken checks a PocketBase auth token and returns the corresponding
// employee record. Returns nil if the token is invalid or expired.
//
// This is called from the Chi auth middleware and does NOT make any HTTP
// requests — it validates directly against PocketBase's embedded SQLite DB.
func (pb *PBAuth) ValidateToken(token string) (*core.Record, error) {
	record, err := pb.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ---------------------------------------------------------------------------
// Employee CRUD
// ---------------------------------------------------------------------------

// FindEmployeeByID returns an employee record by its PocketBase record ID.
func (pb *PBAuth) FindEmployeeByID(id string) (*core.Record, error) {
	return pb.App.FindRecordById("employees", id)
}

// FindEmployeeByEmail returns an employee auth record by email.
func (pb *PBAuth) FindEmployeeByEmail(email string) (*core.Record, error) {
	return pb.App.FindAuthRecordByEmail("employees", strings.TrimSpace(email))
}

// ListEmployees returns all admin, manager and employee records.
func (pb *PBAuth) ListEmployees() ([]*core.Record, error) {
	return pb.App.FindRecordsByFilter(
		"employees",
		"role = 'admin' || role = 'manager' || role = 'employee'",
		"",
		200,
		0,
	)
}

// CreateEmployee creates a new employee in the PocketBase auth collection.
// PocketBase handles password hashing internally.
// locationID is required for managers and employees and empty for admins
// (the handler validates this).
func (pb *PBAuth) CreateEmployee(email, password, name, phone, role, locationID string, disabled bool) (*core.Record, error) {
	collection, err := pb.App.FindCollectionByNameOrId("employees")
	if err != nil {
		return nil, fmt.Errorf("find employees collection: %w", err)
	}

	record := core.NewRecord(collection)
	record.SetEmail(email)
	record.SetPassword(password)
	record.Set("name", name)
	record.Set("phone", phone)
	record.Set("role", role)
	record.Set("location_id", locationID)
	record.Set("disabled", disabled)

	if err := pb.App.Save(record); err != nil {
		return nil, fmt.Errorf("save employee: %w", err)
	}

	return record, nil
}

// UpdateEmployee updates mutable employee fields.
// Password is optional; when omitted the current password remains unchanged.
func (pb *PBAuth) UpdateEmployee(id string, email, password, name, phone, role, locationID *string, disabled *bool) (*core.Record, error) {
	record, err := pb.App.FindRecordById("employees", id)
	if err != nil {
		return nil, fmt.Errorf("find employee: %w", err)
	}

	if email != nil {
		record.SetEmail(strings.TrimSpace(*email))
	}
	if password != nil && strings.TrimSpace(*password) != "" {
		record.SetPassword(*password)
	}
	if name != nil {
		record.Set("name", strings.TrimSpace(*name))
	}
	if phone != nil {
		record.Set("phone", strings.TrimSpace(*phone))
	}
	if role != nil {
		record.Set("role", strings.TrimSpace(*role))
	}
	if locationID != nil {
		record.Set("location_id", strings.TrimSpace(*locationID))
	}
	if disabled != nil {
		record.Set("disabled", *disabled)
	}

	if err := pb.App.Save(record); err != nil {
		return nil, fmt.Errorf("save employee: %w", err)
	}

	return record, nil
}

// AssignMissingLocations gives every manager and employee without a location
// the default one (the original shop). Accounts created before locations
// existed would otherwise be refused at login. Returns how many changed.
func (pb *PBAuth) AssignMissingLocations(defaultLocationID string) (int, error) {
	records, err := pb.App.FindRecordsByFilter(
		"employees",
		"(role = 'manager' || role = 'employee') && location_id = ''",
		"",
		0,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("find employees without location: %w", err)
	}
	for _, record := range records {
		record.Set("location_id", defaultLocationID)
		if err := pb.App.Save(record); err != nil {
			return 0, fmt.Errorf("assign location to employee %s: %w", record.Id, err)
		}
	}
	return len(records), nil
}

// DeactivateEmployee disables an employee account so they can no longer log in.
// The record is not deleted — only its "disabled" flag is set.
// Existing tokens are effectively invalidated because the middleware checks
// the disabled flag on every request.
func (pb *PBAuth) DeactivateEmployee(id string) error {
	record, err := pb.App.FindRecordById("employees", id)
	if err != nil {
		return fmt.Errorf("find employee: %w", err)
	}

	record.Set("disabled", true)

	if err := pb.App.Save(record); err != nil {
		return fmt.Errorf("save employee: %w", err)
	}

	return nil
}

// DeleteEmployee permanently removes an employee record.
func (pb *PBAuth) DeleteEmployee(id string) error {
	record, err := pb.App.FindRecordById("employees", id)
	if err != nil {
		return fmt.Errorf("find employee: %w", err)
	}

	if err := pb.App.Delete(record); err != nil {
		return fmt.Errorf("delete employee: %w", err)
	}

	return nil
}

// StartAdminUI starts PocketBase's built-in HTTP server (admin dashboard + API)
// on the given address (e.g. "0.0.0.0:8090").
//
// This is intended for development only so you can manage the employees
// collection, inspect auth tokens, etc. via http://localhost:<port>/_/.
//
// The method blocks until the server is shut down, so call it in a goroutine:
//
//	go func() {
//	    if err := pb.StartAdminUI("127.0.0.1:8090", []string{"http://localhost:8090"}); err != nil {
//	        log.Error().Err(err).Msg("PocketBase admin UI stopped")
//	    }
//	}()
//
// Bootstrap must have been called before (New already does this).
func (pb *PBAuth) StartAdminUI(httpAddr string, allowedOrigins []string) error {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:8090"}
	}

	err := apis.Serve(pb.App, apis.ServeConfig{
		HttpAddr:        httpAddr,
		ShowStartBanner: true,
		AllowedOrigins:  allowedOrigins,
	})

	// apis.Serve returns http.ErrServerClosed on graceful shutdown — not an error.
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close cleanly shuts down the PocketBase app.
func (pb *PBAuth) Close() error {
	if pb.App != nil {
		if err := pb.App.ClearBootstrap(); err != nil {
			return fmt.Errorf("reset bootstrap state: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Bootstrap: employees auth collection
// ---------------------------------------------------------------------------

// ensureEmployeesCollection creates the "employees" auth collection if it does
// not already exist. The collection stores admin and employee accounts with
// email/password authentication and custom role, name, phone fields.
func (pb *PBAuth) ensureEmployeesCollection() error {
	if existing, err := pb.App.FindCollectionByNameOrId("employees"); err == nil {
		if pb.ensureEmployeesCollectionFields(existing) || pb.ensureEmployeesAuthTokenDuration(existing) {
			if err := pb.App.Save(existing); err != nil {
				return fmt.Errorf("update employees collection fields: %w", err)
			}
		}
		log.Debug().Msg("Employees auth collection already exists")
		return nil
	}

	log.Info().Msg("Creating 'employees' auth collection...")

	collection := core.NewAuthCollection("employees")
	collection.Name = "employees"

	pb.ensureEmployeesCollectionFields(collection)
	pb.ensureEmployeesAuthTokenDuration(collection)

	if err := pb.App.Save(collection); err != nil {
		return fmt.Errorf("create employees collection: %w", err)
	}

	log.Info().Msg("Employees auth collection created successfully")
	return nil
}

func (pb *PBAuth) ensureEmployeesCollectionFields(collection *core.Collection) bool {
	changed := false

	if collection.Fields.GetByName("name") == nil {
		collection.Fields.Add(&core.TextField{
			Name:     "name",
			Required: false,
			Max:      200,
		})
		changed = true
	}

	if collection.Fields.GetByName("phone") == nil {
		collection.Fields.Add(&core.TextField{
			Name:     "phone",
			Required: false,
			Max:      20,
		})
		changed = true
	}

	if existing, ok := collection.Fields.GetByName("role").(*core.SelectField); ok {
		// Collections created before locations only allowed admin/employee.
		if !slices.Equal(existing.Values, EmployeeRoles) {
			existing.Values = append([]string{}, EmployeeRoles...)
			changed = true
		}
	} else if collection.Fields.GetByName("role") == nil {
		collection.Fields.Add(&core.SelectField{
			Name:      "role",
			Required:  true,
			Values:    append([]string{}, EmployeeRoles...),
			MaxSelect: 1,
		})
		changed = true
	}

	// The location (sucursal) a manager or employee works at; empty for admins.
	if collection.Fields.GetByName("location_id") == nil {
		collection.Fields.Add(&core.TextField{
			Name: "location_id",
			Max:  36,
		})
		changed = true
	}

	if collection.Fields.GetByName("disabled") == nil {
		collection.Fields.Add(&core.BoolField{
			Name: "disabled",
		})
		changed = true
	}

	return changed
}

func (pb *PBAuth) ensureEmployeesAuthTokenDuration(collection *core.Collection) bool {
	desiredDurationSeconds := pb.authTokenTTLSeconds
	if desiredDurationSeconds < 10 {
		desiredDurationSeconds = 7 * 24 * int64(time.Hour/time.Second)
	}

	if collection.AuthToken.Duration == desiredDurationSeconds {
		return false
	}

	collection.AuthToken.Duration = desiredDurationSeconds
	return true
}

// ---------------------------------------------------------------------------
// Record helpers
// ---------------------------------------------------------------------------

// Roles. Admins see every location; managers and employees only their own.
const (
	RoleAdmin    = "admin"
	RoleManager  = "manager"
	RoleEmployee = "employee"
)

// EmployeeRoles lists the values of the employees.role field.
var EmployeeRoles = []string{RoleAdmin, RoleManager, RoleEmployee}

// LocationIDFromRecord returns the location an employee record belongs to
// ("" for admins or unassigned accounts).
func LocationIDFromRecord(record *core.Record) string {
	if record == nil {
		return ""
	}
	return record.GetString("location_id")
}

// RoleFromRecord extracts the role string from a PocketBase employee record.
func RoleFromRecord(record *core.Record) string {
	if record == nil {
		return ""
	}
	return record.GetString("role")
}

// RecordID returns the PocketBase record ID string.
func RecordID(record *core.Record) string {
	if record == nil {
		return ""
	}
	return record.Id
}

// IsDisabled checks whether an employee record is disabled.
func IsDisabled(record *core.Record) bool {
	if record == nil {
		return true
	}
	return record.GetBool("disabled")
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

// contextKey is an unexported type for context keys in this package.
type contextKey string

const authRecordKey contextKey = "pb_auth_record"
const roleKey contextKey = "pb_role"

// WithRecord stores the PocketBase auth record in the request context.
func WithRecord(ctx context.Context, record *core.Record) context.Context {
	return context.WithValue(ctx, authRecordKey, record)
}

// GetRecord retrieves the PocketBase auth record from context.
func GetRecord(ctx context.Context) (*core.Record, bool) {
	r, ok := ctx.Value(authRecordKey).(*core.Record)
	return r, ok
}

// WithRole stores the resolved role string in context.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// GetRole retrieves the role string from context.
func GetRole(ctx context.Context) string {
	r, _ := ctx.Value(roleKey).(string)
	return r
}
