// requests/order.go — Request-parsing DTOs for order handlers.
package requests

import (
	"github.com/google/uuid"
)

// CreateOrderRequest is the payload for POST /api/v1/orders.
type CreateOrderRequest struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	CustomerEmail string `json:"customer_email,omitempty"`
	PaymentMethod string `json:"payment_method"`
	OrderType     string `json:"order_type"`
	TableNumber   *int   `json:"table_number,omitempty"`
	Notes         string `json:"notes,omitempty"`
	Items         []struct {
		ProductID      uuid.UUID      `json:"product_id"`
		Quantity       int            `json:"quantity"`
		Customizations map[string]any `json:"customizations,omitempty"`
	} `json:"items"`
}

// UpdateOrderRequest is the payload for PATCH /api/v1/orders/{id}.
type UpdateOrderRequest struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	CustomerEmail string `json:"customer_email,omitempty"`
	PaymentMethod string `json:"payment_method"`
	OrderType     string `json:"order_type"`
	TableNumber   *int   `json:"table_number,omitempty"`
	Notes         string `json:"notes,omitempty"`
	Items         []struct {
		ProductID      uuid.UUID      `json:"product_id"`
		Quantity       int            `json:"quantity"`
		Customizations map[string]any `json:"customizations,omitempty"`
	} `json:"items,omitempty"`
}
