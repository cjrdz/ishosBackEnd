package dto

import (
	"time"

	"github.com/google/uuid"
)

// ExportOrderRow is a flattened, aggregation-ready row for order exports.
type ExportOrderRow struct {
	ID            uuid.UUID `bun:"id" json:"id"`
	OrderNumber   string    `bun:"order_number" json:"order_number"`
	CustomerName  string    `bun:"customer_name" json:"customer_name"`
	CustomerPhone string    `bun:"customer_phone" json:"customer_phone"`
	Status        string    `bun:"status" json:"status"`
	PaymentMethod string    `bun:"payment_method" json:"payment_method"`
	PaymentStatus string    `bun:"payment_status" json:"payment_status"`
	OrderType     string    `bun:"order_type" json:"order_type"`
	TableNumber   *int      `bun:"table_number" json:"table_number,omitempty"`
	TotalAmount   float64   `bun:"total_amount" json:"total_amount"`
	ItemsCount    int       `bun:"items_count" json:"items_count"`
	ItemsSummary  string    `bun:"items_summary" json:"items_summary"`
	Notes         *string   `bun:"notes" json:"notes,omitempty"`
	CreatedAt     time.Time `bun:"created_at" json:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at" json:"updated_at"`
}
