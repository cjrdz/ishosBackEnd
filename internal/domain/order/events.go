// events.go — Domain events for order lifecycle.
// These represent significant business events that other parts of the system care about.
package order

import (
	"time"

	"github.com/google/uuid"
)

// DomainEvent is the base interface for all domain events.
type DomainEvent interface {
	// EventType returns the type name of the event
	EventType() string
	// Timestamp returns when the event occurred
	Timestamp() time.Time
	// AggregateID returns the order ID associated with this event
	AggregateID() uuid.UUID
}

// BaseDomainEvent provides common event properties.
type BaseDomainEvent struct {
	AggregateIDVal uuid.UUID
	TimestampVal   time.Time
}

func (e *BaseDomainEvent) AggregateID() uuid.UUID {
	return e.AggregateIDVal
}

func (e *BaseDomainEvent) Timestamp() time.Time {
	return e.TimestampVal
}

// OrderCreated event occurs when a new order is created and submitted.
type OrderCreated struct {
	BaseDomainEvent
	OrderNumber   string
	CustomerName  string
	CustomerPhone string
	CustomerEmail string
	PaymentMethod string
	OrderType     string
	TotalAmount   float64
	ItemCount     int
	TrackingURL   string
}

func (e *OrderCreated) EventType() string {
	return "OrderCreated"
}

func NewOrderCreated(
	orderID uuid.UUID,
	orderNumber string,
	customerName string,
	customerPhone string,
	customerEmail string,
	paymentMethod string,
	orderType string,
	totalAmount float64,
	itemCount int,
	trackingURL string,
) *OrderCreated {
	return &OrderCreated{
		BaseDomainEvent: BaseDomainEvent{
			AggregateIDVal: orderID,
			TimestampVal:   time.Now(),
		},
		OrderNumber:   orderNumber,
		CustomerName:  customerName,
		CustomerPhone: customerPhone,
		CustomerEmail: customerEmail,
		PaymentMethod: paymentMethod,
		OrderType:     orderType,
		TotalAmount:   totalAmount,
		ItemCount:     itemCount,
		TrackingURL:   trackingURL,
	}
}

// OrderApproved event occurs when an order is approved by a reviewer.
type OrderApproved struct {
	BaseDomainEvent
	OrderNumber   string
	CustomerPhone string
	ReviewerID    string
	TrackingURL   string
}

func (e *OrderApproved) EventType() string {
	return "OrderApproved"
}

func NewOrderApproved(
	orderID uuid.UUID,
	orderNumber string,
	customerPhone string,
	reviewerID string,
	trackingURL string,
) *OrderApproved {
	return &OrderApproved{
		BaseDomainEvent: BaseDomainEvent{
			AggregateIDVal: orderID,
			TimestampVal:   time.Now(),
		},
		OrderNumber:   orderNumber,
		CustomerPhone: customerPhone,
		ReviewerID:    reviewerID,
		TrackingURL:   trackingURL,
	}
}

// OrderRejected event occurs when an order is rejected/cancelled by a reviewer.
type OrderRejected struct {
	BaseDomainEvent
	OrderNumber   string
	CustomerPhone string
	ReviewerID    string
	Reason        string
	TrackingURL   string
}

func (e *OrderRejected) EventType() string {
	return "OrderRejected"
}

func NewOrderRejected(
	orderID uuid.UUID,
	orderNumber string,
	customerPhone string,
	reviewerID string,
	reason string,
	trackingURL string,
) *OrderRejected {
	return &OrderRejected{
		BaseDomainEvent: BaseDomainEvent{
			AggregateIDVal: orderID,
			TimestampVal:   time.Now(),
		},
		OrderNumber:   orderNumber,
		CustomerPhone: customerPhone,
		ReviewerID:    reviewerID,
		Reason:        reason,
		TrackingURL:   trackingURL,
	}
}

// OrderReady event occurs when an order transitions to "lista" (ready) status.
// This is significant because it notifies the customer the order is ready for pickup.
type OrderReady struct {
	BaseDomainEvent
	OrderNumber   string
	CustomerPhone string
	TrackingURL   string
}

func (e *OrderReady) EventType() string {
	return "OrderReady"
}

func NewOrderReady(
	orderID uuid.UUID,
	orderNumber string,
	customerPhone string,
	trackingURL string,
) *OrderReady {
	return &OrderReady{
		BaseDomainEvent: BaseDomainEvent{
			AggregateIDVal: orderID,
			TimestampVal:   time.Now(),
		},
		OrderNumber:   orderNumber,
		CustomerPhone: customerPhone,
		TrackingURL:   trackingURL,
	}
}

// OrderUpdated event occurs when order details are modified (items, metadata, customizations).
type OrderUpdated struct {
	BaseDomainEvent
	OrderNumber string
	Changes     map[string]interface{}
}

func (e *OrderUpdated) EventType() string {
	return "OrderUpdated"
}

func NewOrderUpdated(
	orderID uuid.UUID,
	orderNumber string,
	changes map[string]interface{},
) *OrderUpdated {
	return &OrderUpdated{
		BaseDomainEvent: BaseDomainEvent{
			AggregateIDVal: orderID,
			TimestampVal:   time.Now(),
		},
		OrderNumber: orderNumber,
		Changes:     changes,
	}
}

// OrderDeleted event occurs when an order is deleted from the system.
type OrderDeleted struct {
	BaseDomainEvent
	OrderNumber string
}

func (e *OrderDeleted) EventType() string {
	return "OrderDeleted"
}

func NewOrderDeleted(
	orderID uuid.UUID,
	orderNumber string,
) *OrderDeleted {
	return &OrderDeleted{
		BaseDomainEvent: BaseDomainEvent{
			AggregateIDVal: orderID,
			TimestampVal:   time.Now(),
		},
		OrderNumber: orderNumber,
	}
}

// EventPublisher defines the interface for publishing domain events.
type EventPublisher interface {
	Publish(event DomainEvent) error
}

// EventStore defines the interface for storing domain events (event sourcing).
type EventStore interface {
	Append(event DomainEvent) error
	LoadEvents(orderID uuid.UUID) ([]DomainEvent, error)
}
