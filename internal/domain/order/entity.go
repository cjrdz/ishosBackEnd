// entity.go — Order aggregate root and value objects.
// Business logic for order domain, separate from database representation (models.Order).
package order

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Order is the aggregate root for order domain logic.
// It's independent of the database representation (models.Order).
type Order struct {
	ID               uuid.UUID
	OrderNumber      string
	Status           OrderStatus
	CustomerID       uuid.UUID
	CustomerName     string
	CustomerPhone    string
	CustomerEmail    string
	PaymentMethod    string
	PaymentStatus    string
	OrderType        string
	TableNumber      *int
	Notes            string
	RejectionReason  string
	ReviewedByUserID string
	ReviewedAt       *time.Time
	CreatedByUserID  string
	IsArchived       bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StatusTimestamps map[string]time.Time
	Items            []OrderItem
	TotalAmount      float64
}

// OrderItem is a line item in an order.
type OrderItem struct {
	ID             uuid.UUID
	ProductID      uuid.UUID
	ProductName    string
	Quantity       int
	UnitPrice      float64
	Subtotal       float64
	Customizations Customizations
}

// NewOrder creates a new order with default values.
func NewOrder(
	orderNumber string,
	customerName string,
	customerPhone string,
	paymentMethod string,
	orderType string,
) (*Order, error) {
	if orderNumber == "" {
		return nil, fmt.Errorf("order number cannot be empty")
	}
	if customerName == "" {
		return nil, fmt.Errorf("customer name cannot be empty")
	}
	if customerPhone == "" {
		return nil, fmt.Errorf("customer phone cannot be empty")
	}

	now := time.Now()
	return &Order{
		ID:               uuid.New(),
		OrderNumber:      orderNumber,
		Status:           StatusPendingReview,
		CustomerName:     customerName,
		CustomerPhone:    customerPhone,
		PaymentMethod:    paymentMethod,
		OrderType:        orderType,
		CreatedAt:        now,
		UpdatedAt:        now,
		StatusTimestamps: map[string]time.Time{string(StatusPendingReview): now},
		Items:            []OrderItem{},
	}, nil
}

// AddItem adds a line item to the order.
func (o *Order) AddItem(item OrderItem) error {
	if item.ProductID == uuid.Nil {
		return fmt.Errorf("product ID cannot be empty")
	}
	if item.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if item.UnitPrice < 0 {
		return fmt.Errorf("unit price cannot be negative")
	}

	o.Items = append(o.Items, item)
	o.UpdatedAt = time.Now()
	return nil
}

// CalculateTotal recalculates the total amount from all items.
func (o *Order) CalculateTotal() {
	total := 0.0
	for _, item := range o.Items {
		total += item.Subtotal
	}
	o.TotalAmount = total
	o.UpdatedAt = time.Now()
}

// Approve transitions order from pending to received (first approval).
func (o *Order) Approve(reviewerID string, allowOverride bool) error {
	// Allow override to re-approve cancelled orders
	if !allowOverride && !o.Status.CanTransitionTo(StatusReceived) {
		return fmt.Errorf("cannot approve order in status %s", o.Status)
	}

	if allowOverride && !o.Status.CanTransitionTo(StatusReceived) && o.Status != StatusCancelled {
		return fmt.Errorf("cannot force-approve order in status %s", o.Status)
	}

	o.Status = StatusReceived
	o.ReviewedByUserID = reviewerID
	now := time.Now()
	o.ReviewedAt = &now
	o.UpdatedAt = now
	o.recordStatusTimestamp(StatusReceived, now)
	return nil
}

// Reject transitions order to cancelled status.
func (o *Order) Reject(reviewerID string, reason string) error {
	if !o.Status.CanTransitionTo(StatusCancelled) {
		return fmt.Errorf("cannot reject order in status %s", o.Status)
	}

	o.Status = StatusCancelled
	o.RejectionReason = reason
	o.ReviewedByUserID = reviewerID
	now := time.Now()
	o.ReviewedAt = &now
	o.UpdatedAt = now
	o.recordStatusTimestamp(StatusCancelled, now)
	return nil
}

// TransitionTo moves the order to a new status if valid.
func (o *Order) TransitionTo(newStatus OrderStatus) error {
	if !o.Status.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", o.Status, newStatus)
	}

	o.Status = newStatus
	now := time.Now()
	o.UpdatedAt = now
	o.recordStatusTimestamp(newStatus, now)
	return nil
}

// IsEmpty returns true if the order has no items.
func (o *Order) IsEmpty() bool {
	return len(o.Items) == 0
}

// CanBeEdited returns true if the order can still be modified.
func (o *Order) CanBeEdited() bool {
	// Can only edit pending and received orders
	return o.Status == StatusPendingReview || o.Status == StatusReceived
}

// GetItemByProductID retrieves an item by product ID.
func (o *Order) GetItemByProductID(productID uuid.UUID) *OrderItem {
	for i := range o.Items {
		if o.Items[i].ProductID == productID {
			return &o.Items[i]
		}
	}
	return nil
}

// recordStatusTimestamp records the first time a status was reached.
func (o *Order) recordStatusTimestamp(status OrderStatus, t time.Time) {
	if o.StatusTimestamps == nil {
		o.StatusTimestamps = make(map[string]time.Time)
	}
	key := string(status)
	if _, exists := o.StatusTimestamps[key]; !exists {
		o.StatusTimestamps[key] = t
	}
}

// NewOrderItem creates a new line item.
func NewOrderItem(
	productID uuid.UUID,
	productName string,
	quantity int,
	unitPrice float64,
	customizations Customizations,
) (*OrderItem, error) {
	if productID == uuid.Nil {
		return nil, fmt.Errorf("product ID cannot be empty")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if unitPrice < 0 {
		return nil, fmt.Errorf("unit price cannot be negative")
	}

	subtotal := float64(quantity) * unitPrice
	if customizations == nil {
		customizations = NewCustomizations()
	}

	return &OrderItem{
		ID:             uuid.New(),
		ProductID:      productID,
		ProductName:    productName,
		Quantity:       quantity,
		UnitPrice:      unitPrice,
		Subtotal:       subtotal,
		Customizations: customizations,
	}, nil
}

// UpdateQuantity updates the quantity and recalculates subtotal.
func (oi *OrderItem) UpdateQuantity(quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	oi.Quantity = quantity
	oi.Subtotal = float64(quantity) * oi.UnitPrice
	return nil
}
