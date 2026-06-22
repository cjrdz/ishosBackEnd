package dto

import (
	"time"

	"github.com/google/uuid"
)

type Flavor struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
	IsSeasonal   bool      `json:"is_seasonal"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
