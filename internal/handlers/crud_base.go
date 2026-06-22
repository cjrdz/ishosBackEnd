// crud_base.go — Reusable CRUD base handler patterns for common entities.
// Reduces boilerplate in handlers for flavors, addons, categories, etc.
// Usage: ParseUUID, ParsePagination to extract path/query params with validation.
package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/pkg/response"
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

// ParsePagination extracts page and limit from query parameters with defaults.
// Default: page=1, limit=50
func ParsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 1 {
			page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l >= 1 && l <= 100 {
			limit = l
		}
	}

	return page, limit
}

// RespondJSON writes a JSON response with the given status code.
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	response.JSON(w, statusCode, data)
}

// RespondCreated writes a 201 Created response with the created resource.
func RespondCreated(w http.ResponseWriter, data interface{}) {
	response.JSON(w, http.StatusCreated, data)
}

// RespondOK writes a 200 OK response.
func RespondOK(w http.ResponseWriter, data interface{}) {
	response.JSON(w, http.StatusOK, data)
}

// RespondNoContent writes a 204 No Content response.
func RespondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
