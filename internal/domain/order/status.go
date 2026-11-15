// status.go — OrderStatus state machine with transition validation.
// Encodes valid order lifecycle: Created → Approved → Ready → Delivered
// with optional Rejected rejection path from Pending.
package order

import "fmt"

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

// OrderStatus enum values.
const (
	StatusPendingReview OrderStatus = "pendiente_revision"
	StatusReceived      OrderStatus = "recibida"
	StatusInProgress    OrderStatus = "en_proceso"
	StatusReady         OrderStatus = "lista"
	StatusDelivered     OrderStatus = "entregada"
	StatusCancelled     OrderStatus = "cancelada"
)

// IsValid returns true if the status is one of the defined values.
func (s OrderStatus) IsValid() bool {
	switch s {
	case StatusPendingReview, StatusReceived, StatusInProgress, StatusReady, StatusDelivered, StatusCancelled:
		return true
	default:
		return false
	}
}

// String returns the string representation of the status.
func (s OrderStatus) String() string {
	return string(s)
}

// Transitions returns all valid target statuses from the current status.
// Returns error if current status is invalid.
func (s OrderStatus) Transitions() ([]OrderStatus, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("invalid status: %s", s)
	}

	// State machine transitions
	switch s {
	case StatusPendingReview:
		// From pending review: approve → received, reject → cancelled
		return []OrderStatus{StatusReceived, StatusCancelled}, nil

	case StatusReceived:
		// From received: move to processing or cancel
		return []OrderStatus{StatusInProgress, StatusCancelled}, nil

	case StatusInProgress:
		// From processing: ready or cancel
		return []OrderStatus{StatusReady, StatusCancelled}, nil

	case StatusReady:
		// From ready: deliver, or cancel (admins/managers, with a reason;
		// they choose what goes back to stock)
		return []OrderStatus{StatusDelivered, StatusCancelled}, nil

	case StatusDelivered:
		// Final state: no transitions
		return []OrderStatus{}, nil

	case StatusCancelled:
		// Final state: no transitions
		return []OrderStatus{}, nil

	default:
		return nil, fmt.Errorf("unknown status: %s", s)
	}
}

// CanTransitionTo returns true if a transition from current to target is valid.
func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	transitions, err := s.Transitions()
	if err != nil {
		return false
	}

	for _, t := range transitions {
		if t == target {
			return true
		}
	}
	return false
}

// IsFinal returns true if the status is a terminal state (no further transitions).
func (s OrderStatus) IsFinal() bool {
	return s == StatusDelivered || s == StatusCancelled
}

// IsApproved returns true if the order has been approved (any state after review).
func (s OrderStatus) IsApproved() bool {
	return s == StatusReceived || s == StatusInProgress || s == StatusReady || s == StatusDelivered
}

// IsPending returns true if the order is waiting for review.
func (s OrderStatus) IsPending() bool {
	return s == StatusPendingReview
}

// IsRejected returns true if the order has been cancelled/rejected.
func (s OrderStatus) IsRejected() bool {
	return s == StatusCancelled
}

// NewOrderStatus creates a new OrderStatus, validating the input.
func NewOrderStatus(s string) (OrderStatus, error) {
	status := OrderStatus(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid order status: %s", s)
	}
	return status, nil
}
