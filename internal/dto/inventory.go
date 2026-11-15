package dto

import (
	"time"

	"github.com/google/uuid"
)

type InventoryItem struct {
	ID                uuid.UUID  `json:"id"`
	LocationID        uuid.UUID  `json:"location_id"`
	Name              string     `json:"name"`
	Type              string     `json:"type"`
	CurrentStock      int        `json:"current_stock"`
	LowStockThreshold int        `json:"low_stock_threshold"`
	ProductID         *uuid.UUID `json:"product_id,omitempty"`
	FlavorID          *uuid.UUID `json:"flavor_id,omitempty"`
	// Reserved is stock held by accepted orders not yet in preparation
	// (filled by the inventory dashboard).
	Reserved  int       `json:"reserved"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StockMovement struct {
	ID                 uuid.UUID  `json:"id"`
	LocationID         uuid.UUID  `json:"location_id"`
	Type               string     `json:"type"`
	Quantity           int        `json:"quantity"`
	QuantityContainers *int       `json:"quantity_containers,omitempty"`
	InventoryItemID    uuid.UUID  `json:"inventory_item_id"`
	ContainerTypeID    *uuid.UUID `json:"container_type_id,omitempty"`
	FlavorID           *uuid.UUID `json:"flavor_id,omitempty"`
	OrderID            *uuid.UUID `json:"order_id,omitempty"`
	Reason             *string    `json:"reason,omitempty"`
	CreatedByUserID    *string    `json:"created_by_user_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}
