package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxJSONBodyBytes = 1 << 20 // 1 MiB

// decodeJSON reads the request body as JSON into the provided destination.
func decodeJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()

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
