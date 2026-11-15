package repository

import "github.com/google/uuid"

// CreateOrderInput is the data required to create an order.
type CreateOrderInput struct {
	OrderNumber     string
	LocationID      uuid.UUID
	CustomerID      *uuid.UUID
	CustomerName    string
	CustomerPhone   string
	CustomerEmail   *string
	CustomerIP      *string
	TotalAmount     float64
	Status          string
	PaymentMethod   string
	AmountReceived  *float64
	OrderType       string
	TableNumber     *int
	Notes           *string
	CreatedByUserID *string
}

// CreateOrderItemInput is the data required to create an order item.
type CreateOrderItemInput struct {
	OrderID        uuid.UUID
	ProductID      uuid.UUID
	ProductName    string
	Quantity       int
	UnitPrice      float64
	Subtotal       float64
	Customizations map[string]interface{}
}
