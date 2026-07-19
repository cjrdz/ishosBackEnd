// crud_base.go — Reusable CRUD base handler patterns for common entities.
// Reduces boilerplate in handlers for flavors, addons, categories, etc.
// Usage: ParseUUID to extract path params with validation.
package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ParseUUID extracts and validates a UUID from route parameters.
// Returns formatted error message ready for writeAppError if invalid.
func ParseUUID(r *http.Request, paramName string) (uuid.UUID, error) {
	idStr := chi.URLParam(r, paramName)
	if idStr == "" {
		return uuid.Nil, fmt.Errorf("missing %s parameter", paramName)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s format", paramName)
	}

	return id, nil
}
