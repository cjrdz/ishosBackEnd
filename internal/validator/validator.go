// validator.go — ValidationError and ValidationErrors types shared across all request validation.
package validator

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}

	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
	}

	return strings.Join(msgs, "; ")
}

func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}
