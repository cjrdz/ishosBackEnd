// response.go — Shared helpers for writing JSON and structured error responses.
package response

import (
	"encoding/json"
	"net/http"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
)

type errorBody struct {
	Error string `json:"error"`
}

// JSON writes a JSON-encoded body with the given status code.
func JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a standard error JSON response.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, errorBody{Error: msg})
}

// WriteOrError writes a successful JSON response or maps an error to HTTP response.
// This is the primary handler exit point: WriteOrError(w, result, err).
func WriteOrError(w http.ResponseWriter, result interface{}, err error) {
	if err != nil {
		status := apperror.StatusCode(err)
		Error(w, status, err.Error())
		return
	}
	JSON(w, http.StatusOK, result)
}
