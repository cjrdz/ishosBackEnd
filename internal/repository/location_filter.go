// location_filter.go — Helpers that restrict queries to a location scope.
package repository

import (
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

// whereLocation adds "<column> = ?" when the scope is a single location.
// column must be a trusted identifier such as "location_id" or "o.location_id".
func whereLocation(q *bun.SelectQuery, column string, loc scope.Location) *bun.SelectQuery {
	if id, ok := loc.ID(); ok {
		return q.Where("? = ?", bun.Ident(column), id)
	}
	return q
}

// locationSQL returns an " AND <column> = ?" fragment and its argument for raw
// queries, or nothing when the scope covers every location.
func locationSQL(column string, loc scope.Location) (string, []any) {
	if id, ok := loc.ID(); ok {
		return " AND " + column + " = ?", []any{id}
	}
	return "", nil
}
