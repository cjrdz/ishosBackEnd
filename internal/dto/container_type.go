package dto

import (
	"time"

	"github.com/google/uuid"
)

type ContainerType struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	BallsPerContainer int       `json:"balls_per_container"`
	IsCustom          bool      `json:"is_custom"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
