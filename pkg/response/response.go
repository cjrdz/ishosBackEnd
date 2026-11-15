// response.go — Shared helpers for writing JSON and structured error responses.
package response

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

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
// Error messages are only exposed for recognized application errors; internal
// errors are returned with a generic message.
func WriteOrError(w http.ResponseWriter, result interface{}, err error) {
	if err != nil {
		status := apperror.StatusCode(err)
		msg := apperror.PublicMessage(err)
		if !apperror.IsAppError(err) {
			// The client only sees the generic message; keep the cause in the
			// logs so production 500s are diagnosable.
			log.Error().Err(err).Int("status", status).Msg("unhandled error returned to client")
		}
		Error(w, status, msg)
		return
	}
	JSON(w, http.StatusOK, result)
}
