// order_command_service.go — State mutations for orders with event publishing.
package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	orderDomain "github.com/cjrdz/ishosBackEnd/internal/domain/order"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/events"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

// OrderCommandService handles all state mutations for orders.
type OrderCommandService struct {
	orderRepo    OrderRepository
	orderStore   OrderStore
	customerRepo CustomerRepository
	productRepo  ProductRepository
	inventory    *InventoryService
	dispatcher   *events.Dispatcher
	tracker      *TrackingTokenService
	auditSvc     audit.Service
}

// NewOrderCommandService creates a new command service.
func NewOrderCommandService(
	orderRepo OrderRepository,
	orderStore OrderStore,
	customerRepo CustomerRepository,
	productRepo ProductRepository,
	inventory *InventoryService,
	dispatcher *events.Dispatcher,
	tracker *TrackingTokenService,
	auditSvc audit.Service,
) *OrderCommandService {
	return &OrderCommandService{
		orderRepo:    orderRepo,
		orderStore:   orderStore,
		customerRepo: customerRepo,
		productRepo:  productRepo,
		inventory:    inventory,
		dispatcher:   dispatcher,
		tracker:      tracker,
		auditSvc:     auditSvc,
	}
}

// Create creates a new order with items in a transaction.
// Enriches customizations, validates products, links the order to a customer
// record, and publishes the creation event.
func (s *OrderCommandService) Create(ctx context.Context, input CreateOrderInput) (dto.Order, error) {
	customerID, err := s.resolveCustomerByPhone(ctx, input.CustomerPhone, input.CustomerName, input.CustomerEmail)
	if err != nil {
		return dto.Order{}, err
	}

	storeInput := repository.StoreOrderInput{
		CustomerID:    customerID,
		CustomerName:  input.CustomerName,
		CustomerPhone: input.CustomerPhone,
		CustomerEmail: input.CustomerEmail,
		CustomerIP:    input.CustomerIP,
		PaymentMethod: input.PaymentMethod,
		OrderType:     input.OrderType,
		TableNumber:   input.TableNumber,
		Notes:         input.Notes,
		CreatedByID:   input.CreatedByID,
		Items:         toStoreOrderItemInputs(input.Items),
	}

	createdOrder, err := s.createOrderWithRetry(ctx, storeInput)
	if err != nil {
		return dto.Order{}, err
	}

	if s.dispatcher != nil {
		trackingURL := ""
		s.dispatcher.PublishOrderStatusChanged(events.OrderStatusChanged{
			Status:      events.OrderStatusCreated,
			OrderNumber: createdOrder.OrderNumber,
			Phone:       input.CustomerPhone,
			Total:       createdOrder.TotalAmount,
			TrackingURL: trackingURL,
		})

		customerEmail := ""
		if input.CustomerEmail != nil {
			customerEmail = *input.CustomerEmail
		}
		domainEvent := orderDomain.NewOrderCreated(
			createdOrder.ID,
			createdOrder.OrderNumber,
			input.CustomerName,
			input.CustomerPhone,
			customerEmail,
			input.PaymentMethod,
			input.OrderType,
			createdOrder.TotalAmount,
			len(input.Items),
			trackingURL,
		)
		s.publishDomainEvent(ctx, domainEvent)
	}
	return createdOrder, nil
}

// createOrderWithRetry attempts to create an order, regenerating the order
// number on unique-constraint collisions. This prevents a single order-number
// collision from becoming a raw 500 error for the customer.
func (s *OrderCommandService) createOrderWithRetry(ctx context.Context, storeInput repository.StoreOrderInput) (dto.Order, error) {
	const maxAttempts = 3

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		storeInput.OrderNumber = GenerateOrderNumber()
		createdOrder, err := s.orderStore.CreateOrder(ctx, storeInput)
		if err == nil {
			return createdOrder, nil
		}
		lastErr = err
		if !apperror.IsUniqueViolation(err) {
			return dto.Order{}, err
		}
	}

	return dto.Order{}, fmt.Errorf("%w: unable to generate a unique order number after %d attempts: %v",
		apperror.ErrConflict, maxAttempts, lastErr)
}

// Approve transitions an order to approved status.
func (s *OrderCommandService) Approve(ctx context.Context, id uuid.UUID, reviewerID string, allowOverride bool) (dto.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	domainOrder, err := orderDomain.FromDTOOrder(&order)
	if err != nil {
		return dto.Order{}, err
	}
	if err := domainOrder.Approve(reviewerID, allowOverride); err != nil {
		return dto.Order{}, ErrInvalidOrderState
	}

	updated, err := s.orderRepo.Approve(ctx, id, &reviewerID)
	if err != nil {
		return dto.Order{}, err
	}

	if s.dispatcher != nil {
		trackingURL := ""
		s.dispatcher.PublishOrderStatusChanged(events.OrderStatusChanged{
			Status:      events.OrderStatusApproved,
			OrderNumber: updated.OrderNumber,
			Phone:       updated.CustomerPhone,
			TrackingURL: trackingURL,
		})

		domainEvent := orderDomain.NewOrderApproved(
			id,
			updated.OrderNumber,
			updated.CustomerPhone,
			reviewerID,
			trackingURL,
		)
		s.publishDomainEvent(ctx, domainEvent)
	}
	return updated, nil
}

// Reject transitions an order to rejected status.
func (s *OrderCommandService) Reject(ctx context.Context, id uuid.UUID, reason, reviewerID string) (dto.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	domainOrder, err := orderDomain.FromDTOOrder(&order)
	if err != nil {
		return dto.Order{}, err
	}
	if err := domainOrder.Reject(reviewerID, reason); err != nil {
		return dto.Order{}, ErrInvalidOrderState
	}

	updated, err := s.orderRepo.Reject(ctx, id, reason, &reviewerID)
	if err != nil {
		return dto.Order{}, err
	}

	if s.dispatcher != nil {
		trackingURL := ""
		s.dispatcher.PublishOrderStatusChanged(events.OrderStatusChanged{
			Status:      events.OrderStatusRejected,
			OrderNumber: updated.OrderNumber,
			Phone:       updated.CustomerPhone,
			Reason:      reason,
			TrackingURL: trackingURL,
		})

		domainEvent := orderDomain.NewOrderRejected(
			id,
			updated.OrderNumber,
			updated.CustomerPhone,
			reviewerID,
			reason,
			trackingURL,
		)
		s.publishDomainEvent(ctx, domainEvent)
	}
	return updated, nil
}

// UpdateStatus updates an order's status.
//
// Stock-affecting transitions (en_proceso and cancelada) are committed in the
// same database transaction as the status change so a stock failure rolls back
// the status update and surfaces an error instead of silently drifting inventory.
func (s *OrderCommandService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (dto.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	domainOrder, err := orderDomain.FromDTOOrder(&order)
	if err != nil {
		return dto.Order{}, err
	}
	newStatus, err := orderDomain.NewOrderStatus(status)
	if err != nil {
		return dto.Order{}, ErrInvalidOrderState
	}
	if err := domainOrder.TransitionTo(newStatus); err != nil {
		return dto.Order{}, ErrInvalidOrderState
	}

	oldStatus := order.Status

	var updated dto.Order
	switch {
	case status == "en_proceso" && oldStatus != "en_proceso":
		orderWithItems, err := s.orderRepo.FindByIDWithItems(ctx, id)
		if err != nil {
			return dto.Order{}, err
		}
		deductions, err := s.inventory.DeductionsFor(ctx, orderWithItems.Items)
		if err != nil {
			return dto.Order{}, err
		}
		updated, err = s.orderStore.UpdateStatusWithStockAdjustment(ctx, id, status, deductions, false)
		if err != nil {
			return dto.Order{}, err
		}
	case status == "cancelada" && oldStatus != "cancelada":
		updated, err = s.orderStore.UpdateStatusWithStockAdjustment(ctx, id, status, nil, true)
		if err != nil {
			return dto.Order{}, err
		}
	default:
		updated, err = s.orderStore.UpdateStatus(ctx, id, status)
		if err != nil {
			return dto.Order{}, err
		}
	}

	if status == "lista" {
		if s.dispatcher != nil {
			trackingURL := ""
			s.dispatcher.PublishOrderStatusChanged(events.OrderStatusChanged{
				Status:      events.OrderStatusReady,
				OrderNumber: updated.OrderNumber,
				Phone:       updated.CustomerPhone,
				TrackingURL: trackingURL,
			})

			domainEvent := orderDomain.NewOrderReady(
				id,
				updated.OrderNumber,
				updated.CustomerPhone,
				trackingURL,
			)
			s.publishDomainEvent(ctx, domainEvent)
		}
	}

	return updated, nil
}

// Update updates an order's metadata and items in a transaction.
// When the customer phone changes, the order is re-linked to the matching
// customer record (creating one if necessary).
//
// If the order has already had stock deducted and its items are being replaced,
// the old stock is restored and the new stock is deducted inside the same
// transaction as the order update so the two cannot diverge.
func (s *OrderCommandService) Update(ctx context.Context, id uuid.UUID, input UpdateOrderInput) (dto.Order, error) {
	existing, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	storeInput := repository.StoreUpdateOrderInput{
		CustomerName:  input.CustomerName,
		CustomerPhone: input.CustomerPhone,
		CustomerEmail: input.CustomerEmail,
		PaymentMethod: input.PaymentMethod,
		OrderType:     input.OrderType,
		TableNumber:   input.TableNumber,
		Notes:         input.Notes,
		Items:         toStoreOrderItemInputs(input.Items),
	}

	if input.CustomerPhone != existing.CustomerPhone {
		customerID, err := s.resolveCustomerByPhone(ctx, input.CustomerPhone, input.CustomerName, input.CustomerEmail)
		if err != nil {
			return dto.Order{}, err
		}
		storeInput.CustomerID = customerID
	}

	if len(input.Items) > 0 && orderHasDeductedStock(existing.Status) {
		newItems, err := s.resolveOrderItems(ctx, input.Items)
		if err != nil {
			return dto.Order{}, err
		}
		deductions, err := s.inventory.DeductionsFor(ctx, newItems)
		if err != nil {
			return dto.Order{}, err
		}
		return s.orderStore.UpdateOrderWithStockAdjustment(ctx, id, storeInput, deductions, true)
	}

	return s.orderStore.UpdateOrder(ctx, id, storeInput)
}

// orderHasDeductedStock reports whether an order in the given status has already
// had stock deducted from inventory.
func orderHasDeductedStock(status string) bool {
	switch status {
	case "en_proceso", "lista", "entregada":
		return true
	default:
		return false
	}
}

// resolveOrderItems resolves the products for service-level item inputs so they
// can be passed to inventory deduction logic.
func (s *OrderCommandService) resolveOrderItems(ctx context.Context, items []OrderItemInput) ([]dto.OrderItem, error) {
	result := make([]dto.OrderItem, 0, len(items))
	for _, item := range items {
		product, err := s.productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			return nil, err
		}
		result = append(result, dto.OrderItem{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
			Product:        &product,
		})
	}
	return result, nil
}

// UpdateNotes updates an order's notes field.
func (s *OrderCommandService) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	if _, err := s.orderRepo.FindByID(ctx, id); err != nil {
		return dto.Order{}, err
	}

	return s.orderRepo.UpdateNotes(ctx, id, notes)
}

// Delete deletes an order and revokes any associated tracking tokens.
func (s *OrderCommandService) Delete(ctx context.Context, id uuid.UUID) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if s.tracker != nil {
		if revokeErr := s.tracker.RevokeAllByOrder(ctx, order.OrderNumber); revokeErr != nil {
			return revokeErr
		}
	}

	return s.orderRepo.Delete(ctx, id)
}

// resolveCustomerByPhone returns the customer ID for the given phone, creating a
// new customer record when one does not already exist. It handles the small race
// window where two concurrent orders for the same phone could both attempt to create
// the customer by retrying the lookup on a unique-constraint violation.
func (s *OrderCommandService) resolveCustomerByPhone(ctx context.Context, phone, name string, email *string) (*uuid.UUID, error) {
	phone = strings.TrimSpace(phone)
	name = strings.TrimSpace(name)

	customer, err := s.customerRepo.FindByPhone(ctx, phone)
	if err == nil {
		return &customer.ID, nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return nil, err
	}

	created, createErr := s.customerRepo.Create(ctx, repository.CreateCustomerInput{
		Name:     name,
		UserType: "user",
		Phone:    phone,
		Email:    email,
		Status:   "active",
	})
	if createErr == nil {
		return &created.ID, nil
	}

	// Unique violation: another request created the customer first; look it up again.
	customer, err = s.customerRepo.FindByPhone(ctx, phone)
	if err == nil {
		return &customer.ID, nil
	}
	return nil, createErr
}

// GenerateOrderNumber generates a unique order number using timestamp and random digit.
func GenerateOrderNumber() string {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		randomNum = big.NewInt(int64(time.Now().Nanosecond() % 9000))
	}
	return fmt.Sprintf("ORD-%s-%04d", time.Now().Format("20060102"), randomNum.Int64()+1000)
}

// toStoreOrderItemInputs maps service-level item inputs to the repository store vocabulary.
func toStoreOrderItemInputs(items []OrderItemInput) []repository.StoreOrderItemInput {
	result := make([]repository.StoreOrderItemInput, 0, len(items))
	for _, item := range items {
		result = append(result, repository.StoreOrderItemInput{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
		})
	}
	return result
}

// publishDomainEvent writes a domain event to the audit log.
func (s *OrderCommandService) publishDomainEvent(ctx context.Context, evt orderDomain.DomainEvent) {
	if s.auditSvc == nil || evt == nil {
		return
	}

	actorID := audit.ActorFromContext(ctx)
	ip := audit.IPFromContext(ctx)

	var action, resource string
	var changes map[string]any

	switch e := evt.(type) {
	case *orderDomain.OrderCreated:
		action = audit.ActionOrderCreated
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{
			"order_number":  e.OrderNumber,
			"customer_name": e.CustomerName,
			"payment":       e.PaymentMethod,
			"items":         e.ItemCount,
			"total":         e.TotalAmount,
		}
	case *orderDomain.OrderApproved:
		action = audit.ActionOrderApproved
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{"order_number": e.OrderNumber}
		if actorID == "" {
			actorID = e.ReviewerID
		}
	case *orderDomain.OrderRejected:
		action = audit.ActionOrderRejected
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{"order_number": e.OrderNumber, "reason": e.Reason}
		if actorID == "" {
			actorID = e.ReviewerID
		}
	case *orderDomain.OrderReady:
		action = audit.ActionOrderReady
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{"order_number": e.OrderNumber}
	case *orderDomain.OrderUpdated:
		action = audit.ActionOrderUpdated
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = e.Changes
	case *orderDomain.OrderDeleted:
		action = audit.ActionOrderDeleted
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{"order_number": e.OrderNumber}
	default:
		return
	}

	_ = s.auditSvc.Log(ctx, action, actorID, resource, changes, ip)
}
