package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// decodeJSON reads the request body as JSON into the provided destination.
func decodeJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
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
