package dto

import (
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	UserType  string    `json:"user_type"`
	Phone     string    `json:"phone"`
	Email     *string   `json:"email,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
