// pagination.go — Standardized pagination response and helpers.
package response

import (
	"math"
	"strconv"
)

// Paginated wraps a list of items with pagination metadata.
type Paginated[T any] struct {
	Data       []T                `json:"data"`
	Pagination PaginationMetadata `json:"pagination"`
}

// PaginationMetadata contains pagination information.
type PaginationMetadata struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NewPaginated creates a paginated response.
func NewPaginated[T any](items []T, page, perPage, total int) *Paginated[T] {
	if items == nil {
		items = make([]T, 0)
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	return &Paginated[T]{
		Data: items,
		Pagination: PaginationMetadata{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// PaginationParams parses page and per_page query parameters.
// Returns default values (page=1, perPage=20) if not provided or invalid.
type PaginationParams struct {
	Page    int
	PerPage int
}

// ParsePaginationParams extracts and validates pagination params from query string.
// Valid ranges: page >= 1, perPage 1-100. Defaults: page=1, perPage=20.
func ParsePaginationParams(pageStr, perPageStr string) PaginationParams {
	params := PaginationParams{Page: 1, PerPage: 20}

	if pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err == nil && page > 0 {
			params.Page = page
		}
	}

	if perPageStr != "" {
		perPage, err := strconv.Atoi(perPageStr)
		if err == nil && perPage > 0 {
			// Cap at 100 items per page
			if perPage > 100 {
				perPage = 100
			}
			params.PerPage = perPage
		}
	}

	return params
}
