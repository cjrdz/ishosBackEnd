// query_builder.go — Type-safe SQL query helpers for PostgreSQL dialect hardening.
// Provides safe wrappers around common query patterns to prevent SQL injection.
package database

import (
	"fmt"
	"strings"
	"time"
)

// PaginationParams holds safe pagination parameters.
type PaginationParams struct {
	Limit  int
	Offset int
}

// ValidatePagination ensures pagination parameters are safe.
// Returns max 100 results, min 1.
func ValidatePagination(limit, offset int) PaginationParams {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return PaginationParams{Limit: limit, Offset: offset}
}

// SortDirection is a safe sort direction (ASC or DESC only).
type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

// SafeSortDirection validates and returns a safe sort direction.
func SafeSortDirection(dir string) SortDirection {
	if dir == "desc" || dir == "DESC" {
		return SortDesc
	}
	return SortAsc
}

// TimeRangeFilter holds safe time range parameters.
type TimeRangeFilter struct {
	StartTime time.Time
	EndTime   time.Time
}

// ValidateTimeRange ensures start < end and both are within reasonable bounds.
// Prevents queries that could scan too much data.
func ValidateTimeRange(start, end time.Time) TimeRangeFilter {
	now := time.Now()

	// Clamp to past 1 year
	minTime := now.AddDate(-1, 0, 0)
	if start.Before(minTime) {
		start = minTime
	}

	// Clamp to future 30 days
	maxTime := now.AddDate(0, 0, 30)
	if end.After(maxTime) {
		end = maxTime
	}

	// Ensure start < end
	if !start.Before(end) {
		start = now.AddDate(-30, 0, 0)
		end = now
	}

	return TimeRangeFilter{StartTime: start, EndTime: end}
}

// IDList safely formats a list of IDs for IN clauses.
// Returns empty string if list is empty (caller should check).
// Example: "id IN ($1, $2, $3)" with args provided separately.
type IDList struct {
	Placeholders string
	Count        int
}

// NewIDList creates a safe placeholder string for parameterized queries.
// Do NOT concatenate IDs directly; pass as separate args to Query/Exec.
func NewIDList(count int) IDList {
	if count <= 0 {
		return IDList{Placeholders: "", Count: 0}
	}

	// Generate: $1, $2, $3, ... $n
	str := ""
	for i := 1; i <= count; i++ {
		if i > 1 {
			str += ", "
		}
		str += fmt.Sprintf("$%d", i)
	}
	return IDList{Placeholders: str, Count: count}
}

// QueryStringFilter safely validates a search string.
// Allows alphanumeric, spaces, and common punctuation only.
// Returns trimmed string; empty string if invalid.
func QueryStringFilter(input string) string {
	// This is a simple allowlist; customize per your needs.
	const allowedChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 -._@:"

	result := ""
	for _, r := range input {
		if strings.ContainsRune(allowedChars, r) {
			result += string(r)
		}
	}
	return strings.TrimSpace(result)
}
