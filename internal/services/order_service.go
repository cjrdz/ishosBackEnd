// order_service.go — Creates, validates, and transitions orders; fires lifecycle notifications via the event dispatcher.
package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/events"
)

var (
	ErrOrderNotFound      = apperror.ErrNotFound
	ErrInvalidOrderState  = apperror.ErrConflict
	ErrProductNotFound    = apperror.ErrNotFound
	ErrProductUnavailable = apperror.ErrConflict
	ErrFlavorRequired     = apperror.ErrValidation
	ErrFlavorNotAllowed   = apperror.ErrValidation
	ErrAddonNotAllowed    = apperror.ErrValidation
	ErrToppingRequired    = apperror.ErrValidation
	ErrJaleaRequired      = apperror.ErrValidation
)

const defaultAddonGroupName = "extras"

var freeAddonAllowanceByGroup = map[string]int{
	"toppings": 1,
	"jalea":    1,
}

type OrderService struct {
	orderRepo      OrderRepository
	tracker        *TrackingTokenService
	dispatcher     *events.Dispatcher
	publicStoreURL string
	enrichment     *OrderEnrichmentService
	query          *OrderQueryService
	tracking       *OrderTrackingService
	command        *OrderCommandService
}

type CreateOrderInput struct {
	IdempotencyKey *uuid.UUID
	CustomerName   string
	CustomerPhone  string
	CustomerEmail  *string
	CustomerIP     *string
	PaymentMethod  string
	OrderType      string
	TableNumber    *int
	Notes          *string
	Items          []OrderItemInput
	CreatedByID    *string
}

type OrderItemInput struct {
	ProductID      uuid.UUID
	Quantity       int
	Customizations map[string]any
}

type UpdateOrderInput struct {
	CustomerName  string
	CustomerPhone string
	CustomerEmail *string
	PaymentMethod string
	OrderType     string
	TableNumber   *int
	Notes         *string
	Items         []OrderItemInput
}

func NewOrderService(orderRepo OrderRepository, orderStore OrderStore, customerRepo CustomerRepository, productRepo ProductRepository, inventory *InventoryService, tracker *TrackingTokenService, dispatcher *events.Dispatcher, publicStoreURL string) *OrderService {
	return &OrderService{
		orderRepo:      orderRepo,
		tracker:        tracker,
		dispatcher:     dispatcher,
		publicStoreURL: strings.TrimRight(strings.TrimSpace(publicStoreURL), "/"),
		enrichment:     NewOrderEnrichmentService(),
		query:          NewOrderQueryService(orderRepo),
		tracking:       NewOrderTrackingService(tracker, orderRepo, publicStoreURL),
		command:        NewOrderCommandService(orderRepo, orderStore, customerRepo, productRepo, inventory, dispatcher, tracker),
	}
}

func (s *OrderService) IssueTrackingAccess(ctx context.Context, orderNumber string) (*TrackingAccess, error) {
	return s.tracking.IssueTrackingAccess(ctx, orderNumber)
}

func (s *OrderService) ListPaginated(ctx context.Context, status string, archived bool, page, limit int) ([]dto.Order, int, error) {
	return s.query.ListPaginated(ctx, status, archived, page, limit)
}

func (s *OrderService) GetByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	return s.query.GetByIDWithItems(ctx, id)
}

func (s *OrderService) GetByIDWithItemsAndProducts(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	return s.query.GetByIDWithItemsAndProducts(ctx, id)
}

func (s *OrderService) GetByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error) {
	return s.query.GetByOrderNumber(ctx, orderNumber)
}

func (s *OrderService) GetByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error) {
	return s.query.GetByOrderNumberAndPhone(ctx, orderNumber, customerPhone)
}

func (s *OrderService) ListRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error) {
	return s.query.ListRecentByPhone(ctx, customerPhone, limit)
}

func (s *OrderService) Create(ctx context.Context, input CreateOrderInput) (dto.Order, error) {
	return s.command.Create(ctx, input)
}

func (s *OrderService) Approve(ctx context.Context, id uuid.UUID, reviewerID string, allowOverride bool) (dto.Order, error) {
	return s.command.Approve(ctx, id, reviewerID, allowOverride)
}

func (s *OrderService) Reject(ctx context.Context, id uuid.UUID, reason, reviewerID string) (dto.Order, error) {
	return s.command.Reject(ctx, id, reason, reviewerID)
}

func (s *OrderService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (dto.Order, error) {
	return s.command.UpdateStatus(ctx, id, status)
}

func (s *OrderService) Update(ctx context.Context, id uuid.UUID, input UpdateOrderInput) (dto.Order, error) {
	return s.command.Update(ctx, id, input)
}

func (s *OrderService) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	return s.command.UpdateNotes(ctx, id, notes)
}

func (s *OrderService) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error) {
	return s.command.SetArchived(ctx, id, archived)
}

func (s *OrderService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.command.Delete(ctx, id)
}

// nonEditableStatuses are statuses that prevent further edits or deletion.
var nonEditableStatuses = map[string]bool{
	"recibida":  true,
	"lista":     true,
	"entregada": true,
	"cancelada": true,
}

// IsOrderEditable returns false when the order status is locked (recibida, lista, entregada, cancelada).
func (s *OrderService) IsOrderEditable(status string) bool {
	return !nonEditableStatuses[status]
}
