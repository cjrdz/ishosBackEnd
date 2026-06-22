package dto

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID               uuid.UUID  `json:"id"`
	OrderNumber      string     `json:"order_number"`
	CustomerName     string     `json:"customer_name"`
	CustomerPhone    string     `json:"customer_phone"`
	CustomerEmail    *string    `json:"customer_email,omitempty"`
	CustomerIP       *string    `json:"customer_ip,omitempty"`
	TotalAmount      float64    `json:"total_amount"`
	Status           string     `json:"status"`
	PaymentMethod    string     `json:"payment_method"`
	PaymentStatus    string     `json:"payment_status"`
	OrderType        string     `json:"order_type"`
	TableNumber      *int       `json:"table_number,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	RejectionReason  *string    `json:"rejection_reason,omitempty"`
	ReviewedByUserID *string    `json:"reviewed_by_user_id,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedByUserID  *string    `json:"created_by_user_id,omitempty"`
	IsArchived       bool       `json:"is_archived"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	StatusTimestamps map[string]time.Time `json:"status_timestamps,omitempty"`
	Items            []OrderItem          `json:"items,omitempty"`
}

type OrderItem struct {
	ID             uuid.UUID      `json:"id"`
	OrderID        uuid.UUID      `json:"order_id"`
	ProductID      uuid.UUID      `json:"product_id"`
	ProductName    string         `json:"product_name"`
	Quantity       int            `json:"quantity"`
	UnitPrice      float64        `json:"unit_price"`
	Subtotal       float64        `json:"subtotal"`
	Customizations map[string]any `json:"customizations,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Product        *Product       `json:"product,omitempty"`
}
