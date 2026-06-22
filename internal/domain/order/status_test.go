// status_test.go — Tests for OrderStatus state machine.
package order

import (
	"testing"
)

func TestOrderStatusTransitions(t *testing.T) {
	tests := []struct {
		from          OrderStatus
		to            OrderStatus
		shouldSucceed bool
	}{
		// Valid transitions
		{StatusPendingReview, StatusReceived, true},
		{StatusPendingReview, StatusCancelled, true},
		{StatusReceived, StatusInProgress, true},
		{StatusReceived, StatusCancelled, true},
		{StatusInProgress, StatusReady, true},
		{StatusInProgress, StatusCancelled, true},
		{StatusReady, StatusDelivered, true},

		// Invalid transitions
		{StatusDelivered, StatusReceived, false},
		{StatusDelivered, StatusCancelled, false},
		{StatusCancelled, StatusReceived, false},
		{StatusReceived, StatusPendingReview, false},
	}

	for _, tt := range tests {
		t.Run(tt.from.String()+"->"+tt.to.String(), func(t *testing.T) {
			can := tt.from.CanTransitionTo(tt.to)
			if can != tt.shouldSucceed {
				t.Errorf("CanTransitionTo(%s, %s) = %v, want %v", tt.from, tt.to, can, tt.shouldSucceed)
			}
		})
	}
}

func TestOrderStatusFinal(t *testing.T) {
	tests := []struct {
		status  OrderStatus
		isFinal bool
	}{
		{StatusDelivered, true},
		{StatusCancelled, true},
		{StatusPendingReview, false},
		{StatusReceived, false},
		{StatusInProgress, false},
		{StatusReady, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			final := tt.status.IsFinal()
			if final != tt.isFinal {
				t.Errorf("IsFinal(%s) = %v, want %v", tt.status, final, tt.isFinal)
			}
		})
	}
}

func TestNewOrderStatus(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected OrderStatus
	}{
		{"pendiente_revision", true, StatusPendingReview},
		{"recibida", true, StatusReceived},
		{"en_proceso", true, StatusInProgress},
		{"lista", true, StatusReady},
		{"entregada", true, StatusDelivered},
		{"cancelada", true, StatusCancelled},
		{"invalid", false, ""},
		{"", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			status, err := NewOrderStatus(tt.input)
			if (err != nil) == tt.valid {
				t.Errorf("NewOrderStatus(%q) error = %v, want valid = %v", tt.input, err, tt.valid)
			}
			if tt.valid && status != tt.expected {
				t.Errorf("NewOrderStatus(%q) = %v, want %v", tt.input, status, tt.expected)
			}
		})
	}
}
