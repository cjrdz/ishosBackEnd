// order_query_service.go — Read-only queries for orders with consistent error handling.
package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// OrderQueryService provides read-only order queries with error normalization.
type OrderQueryService struct {
	orderRepo OrderRepository
}

// NewOrderQueryService creates a new query service.
func NewOrderQueryService(orderRepo OrderRepository) *OrderQueryService {
	return &OrderQueryService{
		orderRepo: orderRepo,
	}
}

// ListPaginated retrieves paginated orders by status.
// archived=false returns active orders; archived=true returns only archived orders.
func (s *OrderQueryService) ListPaginated(ctx context.Context, status string, archived bool, page, limit int) ([]dto.Order, int, error) {
	return s.orderRepo.ListPaginated(ctx, status, archived, page, limit)
}

// GetByIDWithItems retrieves an order by ID with its items loaded.
func (s *OrderQueryService) GetByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	return s.orderRepo.FindByIDWithItems(ctx, id)
}

// GetByIDWithItemsAndProducts retrieves an order by ID with its items and the
// full product records needed for inventory deductions.
func (s *OrderQueryService) GetByIDWithItemsAndProducts(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	return s.orderRepo.FindByIDWithItemsAndProducts(ctx, id)
}

// GetByOrderNumber retrieves an order by its order number.
// Returns ErrOrderNotFound if order doesn't exist.
func (s *OrderQueryService) GetByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error) {
	return s.orderRepo.FindByOrderNumber(ctx, orderNumber)
}

// GetByOrderNumberAndPhone retrieves an order by number and phone for public access.
// Returns ErrOrderNotFound if order doesn't exist.
func (s *OrderQueryService) GetByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error) {
	return s.orderRepo.FindByOrderNumberAndPhone(ctx, orderNumber, customerPhone)
}

// ListRecentByPhone retrieves recent orders for a customer by phone number.
func (s *OrderQueryService) ListRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error) {
	return s.orderRepo.FindRecentByPhone(ctx, customerPhone, limit)
}
