// order_response.go — Maps dto.Order values to API response structs.
package services

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// OrderItemResponse is the public shape of an order item.
type OrderItemResponse struct {
	ID             uuid.UUID      `json:"id"`
	ProductID      uuid.UUID      `json:"product_id"`
	ProductName    string         `json:"product_name"`
	Quantity       int            `json:"quantity"`
	UnitPrice      float64        `json:"unit_price"`
	Subtotal       float64        `json:"subtotal"`
	Customizations map[string]any `json:"customizations,omitempty"`
}

// OrderResponse is the public shape of an order.
type OrderResponse struct {
	ID               uuid.UUID            `json:"id"`
	OrderNumber      string               `json:"order_number"`
	CustomerID       *uuid.UUID           `json:"customer_id,omitempty"`
	CustomerName     string               `json:"customer_name"`
	CustomerPhone    string               `json:"customer_phone"`
	CustomerEmail    *string              `json:"customer_email,omitempty"`
	Status           string               `json:"status"`
	PaymentMethod    string               `json:"payment_method"`
	PaymentStatus    string               `json:"payment_status"`
	OrderType        string               `json:"order_type"`
	TableNumber      *int                 `json:"table_number,omitempty"`
	TotalAmount      float64              `json:"total_amount"`
	AmountReceived   *float64             `json:"amount_received,omitempty"`
	Notes            *string              `json:"notes,omitempty"`
	RejectionReason  *string              `json:"rejection_reason,omitempty"`
	CreatedByUserID  *string              `json:"created_by_user_id,omitempty"`
	CreatedByName    string               `json:"created_by_name"`
	CreatedByRole    string               `json:"created_by_role"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	StatusTimestamps map[string]time.Time `json:"status_timestamps,omitempty"`
	Items            []OrderItemResponse  `json:"items"`
	IsArchived       bool                 `json:"is_archived"`
}

type employeeCacheEntry struct {
	name string
	role string
}

// OrderResponseMapper maps dto.Order values to OrderResponse.
type OrderResponseMapper struct {
	pb            *auth.PBAuth
	employeeCache *cache.TTLCache
}

// NewOrderResponseMapper creates a mapper. pb may be nil.
func NewOrderResponseMapper(pb *auth.PBAuth) *OrderResponseMapper {
	return &OrderResponseMapper{
		pb:            pb,
		employeeCache: cache.New(time.Minute),
	}
}

// Map converts a dto.Order to an OrderResponse.
func (m *OrderResponseMapper) Map(order dto.Order) OrderResponse {
	return m.mapInternal(order)
}

// OrderLiteResponse is a minimal list-view representation of an order.
// It omits items, created-by resolution, tracking tokens, and timestamps
// to keep list responses small and fast.
type OrderLiteResponse struct {
	ID           uuid.UUID `json:"id"`
	OrderNumber  string    `json:"order_number"`
	CustomerName string    `json:"customer_name"`
	Status       string    `json:"status"`
	TotalAmount  float64   `json:"total_amount"`
	CreatedAt    time.Time `json:"created_at"`
	IsArchived   bool      `json:"is_archived"`
}

// MapLite converts a dto.Order to an OrderLiteResponse.
func (m *OrderResponseMapper) MapLite(order dto.Order) OrderLiteResponse {
	return OrderLiteResponse{
		ID:           order.ID,
		OrderNumber:  order.OrderNumber,
		CustomerName: order.CustomerName,
		Status:       order.Status,
		TotalAmount:  order.TotalAmount,
		CreatedAt:    order.CreatedAt,
		IsArchived:   order.IsArchived,
	}
}

// PreloadEmployees fetches all employees from PocketBase and warms the cache
// in a single query. Call this before mapping a list of orders to avoid N+1
// lookups when resolving created_by names.
func (m *OrderResponseMapper) PreloadEmployees() {
	if m.pb == nil {
		return
	}
	records, err := m.pb.ListEmployees()
	if err != nil {
		return
	}
	for _, record := range records {
		id := auth.RecordID(record)
		name := strings.TrimSpace(record.GetString("name"))
		if name == "" {
			name = record.Email()
		}
		m.employeeCache.Set(id, employeeCacheEntry{
			name: name,
			role: auth.RoleFromRecord(record),
		})
	}
}

// lookupEmployee returns the cached name/role for an employee, fetching from
// PocketBase only on cache miss. This avoids N+1 lookups when mapping lists.
func (m *OrderResponseMapper) lookupEmployee(id string) employeeCacheEntry {
	if id == "" || m.pb == nil {
		return employeeCacheEntry{}
	}

	if v, ok := m.employeeCache.Get(id); ok {
		return v.(employeeCacheEntry)
	}

	record, err := m.pb.FindEmployeeByID(id)
	if err != nil || record == nil {
		m.employeeCache.Set(id, employeeCacheEntry{})
		return employeeCacheEntry{}
	}

	name := strings.TrimSpace(record.GetString("name"))
	if name == "" {
		name = record.Email()
	}

	entry := employeeCacheEntry{
		name: name,
		role: auth.RoleFromRecord(record),
	}
	m.employeeCache.Set(id, entry)
	return entry
}

func (m *OrderResponseMapper) mapInternal(order dto.Order) OrderResponse {
	items := make([]OrderItemResponse, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, OrderItemResponse{
			ID:             item.ID,
			ProductID:      item.ProductID,
			ProductName:    item.ProductName,
			Quantity:       item.Quantity,
			UnitPrice:      item.UnitPrice,
			Subtotal:       item.Subtotal,
			Customizations: item.Customizations,
		})
	}

	createdByName := order.CustomerName
	createdByRole := "customer"
	if order.CreatedByUserID != nil && *order.CreatedByUserID != "" {
		createdByRole = "employee"
		entry := m.lookupEmployee(*order.CreatedByUserID)
		if entry.name != "" {
			createdByName = entry.name
		} else {
			createdByName = *order.CreatedByUserID
		}
		if entry.role != "" {
			createdByRole = entry.role
		}
	}

	resp := OrderResponse{
		ID:               order.ID,
		OrderNumber:      order.OrderNumber,
		CustomerID:       order.CustomerID,
		CustomerName:     order.CustomerName,
		CustomerPhone:    order.CustomerPhone,
		CustomerEmail:    order.CustomerEmail,
		Status:           order.Status,
		PaymentMethod:    order.PaymentMethod,
		PaymentStatus:    order.PaymentStatus,
		OrderType:        order.OrderType,
		TableNumber:      order.TableNumber,
		TotalAmount:      order.TotalAmount,
		AmountReceived:   order.AmountReceived,
		Notes:            order.Notes,
		RejectionReason:  order.RejectionReason,
		CreatedByUserID:  order.CreatedByUserID,
		CreatedByName:    createdByName,
		CreatedByRole:    createdByRole,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
		StatusTimestamps: order.StatusTimestamps,
		Items:            items,
		IsArchived:       order.IsArchived,
	}

	return resp
}
