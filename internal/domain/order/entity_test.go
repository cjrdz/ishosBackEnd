// entity_test.go — Tests for Order aggregate and OrderItem value objects.
package order

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewOrder(t *testing.T) {
	tests := []struct {
		name          string
		orderNumber   string
		customerName  string
		customerPhone string
		paymentMethod string
		orderType     string
		shouldSucceed bool
	}{
		{
			"Valid order",
			"ORD-20250215-0001",
			"Juan Pérez",
			"+1234567890",
			"efectivo",
			"para_llevar",
			true,
		},
		{
			"Missing order number",
			"",
			"Juan Pérez",
			"+1234567890",
			"efectivo",
			"para_llevar",
			false,
		},
		{
			"Missing customer name",
			"ORD-20250215-0001",
			"",
			"+1234567890",
			"efectivo",
			"para_llevar",
			false,
		},
		{
			"Missing customer phone",
			"ORD-20250215-0001",
			"Juan Pérez",
			"",
			"efectivo",
			"para_llevar",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := NewOrder(tt.orderNumber, tt.customerName, tt.customerPhone, tt.paymentMethod, tt.orderType)
			if (err != nil) == tt.shouldSucceed {
				t.Errorf("NewOrder error = %v, want success = %v", err, tt.shouldSucceed)
			}
			if tt.shouldSucceed && order == nil {
				t.Errorf("NewOrder returned nil for valid input")
			}
		})
	}
}

func TestOrderAddItem(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")
	productID := uuid.New()

	item, _ := NewOrderItem(productID, "Pizza", 2, 10.0, nil)
	err := order.AddItem(*item)
	if err != nil {
		t.Errorf("AddItem failed: %v", err)
	}

	if len(order.Items) != 1 {
		t.Errorf("Items count = %d, want 1", len(order.Items))
	}
}

func TestOrderCalculateTotal(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")

	items := []struct {
		productID uuid.UUID
		name      string
		qty       int
		price     float64
	}{
		{uuid.New(), "Item1", 2, 10.0},
		{uuid.New(), "Item2", 1, 20.0},
	}

	for _, item := range items {
		orderItem, _ := NewOrderItem(item.productID, item.name, item.qty, item.price, nil)
		_ = order.AddItem(*orderItem)
	}

	order.CalculateTotal()
	expected := 40.0 // 2*10 + 1*20

	if order.TotalAmount != expected {
		t.Errorf("TotalAmount = %f, want %f", order.TotalAmount, expected)
	}
}

func TestOrderApprove(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")

	if order.Status != StatusPendingReview {
		t.Errorf("Initial status = %s, want %s", order.Status, StatusPendingReview)
	}

	err := order.Approve("reviewer-123", false)
	if err != nil {
		t.Errorf("Approve failed: %v", err)
	}

	if order.Status != StatusReceived {
		t.Errorf("After approve status = %s, want %s", order.Status, StatusReceived)
	}

	if order.ReviewedByUserID != "reviewer-123" {
		t.Errorf("ReviewedByUserID = %s, want reviewer-123", order.ReviewedByUserID)
	}

	if order.ReviewedAt == nil {
		t.Errorf("ReviewedAt is nil, want timestamp")
	}
}

func TestOrderReject(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")

	err := order.Reject("reviewer-123", "Out of stock")
	if err != nil {
		t.Errorf("Reject failed: %v", err)
	}

	if order.Status != StatusCancelled {
		t.Errorf("After reject status = %s, want %s", order.Status, StatusCancelled)
	}

	if order.RejectionReason != "Out of stock" {
		t.Errorf("RejectionReason = %s, want 'Out of stock'", order.RejectionReason)
	}
}

func TestOrderTransitionTo(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")

	// Transition: Pending → Received
	if err := order.Approve("reviewer", false); err != nil {
		t.Fatalf("Failed to approve: %v", err)
	}

	// Transition: Received → InProgress
	if err := order.TransitionTo(StatusInProgress); err != nil {
		t.Fatalf("Failed to transition to InProgress: %v", err)
	}

	if order.Status != StatusInProgress {
		t.Errorf("Status = %s, want %s", order.Status, StatusInProgress)
	}

	// Try invalid transition: InProgress → Received (should fail)
	err := order.TransitionTo(StatusReceived)
	if err == nil {
		t.Errorf("TransitionTo should fail for invalid transition")
	}
}

func TestOrderCanBeEdited(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")

	if !order.CanBeEdited() {
		t.Errorf("Pending order should be editable")
	}

	_ = order.Approve("reviewer", false)
	if !order.CanBeEdited() {
		t.Errorf("Received order should be editable")
	}

	_ = order.TransitionTo(StatusInProgress)
	if order.CanBeEdited() {
		t.Errorf("InProgress order should not be editable")
	}
}

func TestOrderItemUpdateQuantity(t *testing.T) {
	item, _ := NewOrderItem(uuid.New(), "Item", 2, 10.0, nil)
	initialSubtotal := item.Subtotal

	if initialSubtotal != 20.0 {
		t.Errorf("Initial subtotal = %f, want 20.0", initialSubtotal)
	}

	_ = item.UpdateQuantity(5)
	if item.Quantity != 5 {
		t.Errorf("Quantity = %d, want 5", item.Quantity)
	}

	if item.Subtotal != 50.0 {
		t.Errorf("Subtotal = %f, want 50.0", item.Subtotal)
	}
}

func TestOrderGetItemByProductID(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")
	productID := uuid.New()

	item, _ := NewOrderItem(productID, "Item", 1, 10.0, nil)
	_ = order.AddItem(*item)

	found := order.GetItemByProductID(productID)
	if found == nil {
		t.Fatalf("GetItemByProductID returned nil")
	}

	if found.ProductID != productID {
		t.Errorf("ProductID mismatch")
	}

	notFound := order.GetItemByProductID(uuid.New())
	if notFound != nil {
		t.Errorf("GetItemByProductID should return nil for non-existent product")
	}
}

func TestOrderIsEmpty(t *testing.T) {
	order, _ := NewOrder("ORD-001", "John", "+123", "cash", "takeout")

	if !order.IsEmpty() {
		t.Errorf("New order should be empty")
	}

	item, _ := NewOrderItem(uuid.New(), "Item", 1, 10.0, nil)
	_ = order.AddItem(*item)

	if order.IsEmpty() {
		t.Errorf("Order with items should not be empty")
	}
}
