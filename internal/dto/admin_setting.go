package dto

import (
	"time"

	"github.com/google/uuid"
)

type AdminSetting struct {
	ID        uuid.UUID      `json:"id"`
	Key       string         `json:"key"`
	Value     map[string]any `json:"value"`
	ChangedBy string         `json:"changed_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
