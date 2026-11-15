package dto

import (
	"time"

	"github.com/google/uuid"
)

// HoursWindow is one opening window in store-local "HH:MM".
type HoursWindow struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// Location is a sucursal.
type Location struct {
	ID                  uuid.UUID                `json:"id"`
	Slug                string                   `json:"slug"`
	Name                string                   `json:"name"`
	Address             string                   `json:"address"`
	MapsURL             string                   `json:"maps_url"`
	MapsEmbedURL        string                   `json:"maps_embed_url"`
	WhatsApp            string                   `json:"whatsapp"`
	Phone               string                   `json:"phone"`
	Hours               map[string][]HoursWindow `json:"hours"`
	OrdersEnabled       bool                     `json:"orders_enabled"`
	OnlineOrdersEnabled bool                     `json:"online_orders_enabled"`
	IsActive            bool                     `json:"is_active"`
	SortOrder           int                      `json:"sort_order"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
}

// SoldOutItems lists what is marked "Agotado" at one location.
type SoldOutItems struct {
	LocationID uuid.UUID   `json:"location_id"`
	Products   []uuid.UUID `json:"products"`
	Flavors    []uuid.UUID `json:"flavors"`
	// OutOfStockProducts / OutOfStockFlavors are tracked in inventory with
	// no stock left at the location. They are unavailable like the manual
	// "Agotado" lists but clear themselves when stock is entered.
	OutOfStockProducts []uuid.UUID `json:"out_of_stock_products"`
	OutOfStockFlavors  []uuid.UUID `json:"out_of_stock_flavors"`
}
