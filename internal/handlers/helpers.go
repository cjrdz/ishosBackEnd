// helpers.go — Shared HTTP utilities used across all handlers.
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

const maxJSONBodyBytes = 1 << 20 // 1 MiB

// decodeJSON reads the request body as JSON into the provided destination.
func decodeJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()

	// Read the full body before checking size; LimitReader is set to max+1 so we can
	// distinguish "exactly at limit" from "over limit" without a second read.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("request body is empty")
	}
	if len(body) > maxJSONBodyBytes {
		return fmt.Errorf("request body too large (max %d bytes)", maxJSONBodyBytes)
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Reject payloads containing more than one JSON value.
	if dec.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("invalid JSON: request body must contain a single JSON object")
	}
	return nil
}

// writeAppError maps typed application errors to consistent HTTP responses.
func writeAppError(w http.ResponseWriter, err error, fallbackMsg string) {
	status := apperror.StatusCode(err)
	if status == http.StatusInternalServerError {
		response.Error(w, status, fallbackMsg)
		return
	}
	response.Error(w, status, err.Error())
}
