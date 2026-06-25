package dto

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID                   uuid.UUID       `json:"id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Price                float64         `json:"price"`
	CategoryID           uuid.UUID       `json:"category_id"`
	ImagePath            *string         `json:"image_path,omitempty"`
	IsAvailable          bool            `json:"is_available"`
	ExcludeGlobalFlavors bool            `json:"exclude_global_flavors"`
	ExcludeGlobalAddons  bool            `json:"exclude_global_addons"`
	BallBased            bool            `json:"ball_based"`
	BallQuantity         int             `json:"ball_quantity"`
	AllowsMixedFlavors   bool            `json:"allows_mixed_flavors"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	Category             *Category       `json:"category,omitempty"`
	Flavors              []Flavor        `json:"flavors,omitempty"`
	Addons               []Addon         `json:"addons,omitempty"`
	InventoryItems       []InventoryItem `json:"inventory_items,omitempty"`
}
