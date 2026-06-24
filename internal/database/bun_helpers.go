// bun_helpers.go — Small helpers for Bun migrations.
package database

import (
	"errors"
	"strings"
)

// isDuplicateConstraint reports whether an error is Postgres complaining that
// a constraint (FK, index, etc.) already exists. Makes migrations idempotent.
func isDuplicateConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "duplicate key value violates unique constraint") ||
		strings.Contains(msg, "pg_tables_primary_key") ||
		errors.Is(err, errors.New("constraint already exists"))
}
