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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	orderDomain "github.com/cjrdz/ishosBackEnd/internal/domain/order"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/events"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

// OfferPriceSource provides the special base prices of active store offers.
type OfferPriceSource interface {
	ActiveOfferPrices(ctx context.Context) (map[uuid.UUID]float64, error)
}

// OrderCommandService handles all state mutations for orders.
type OrderCommandService struct {
	offers       OfferPriceSource
	orderRepo    OrderRepository
	orderStore   OrderStore
	customerRepo CustomerRepository
	productRepo  ProductRepository
	inventory    *InventoryService
	dispatcher   *events.Dispatcher
	tracker      *TrackingTokenService
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
) *OrderCommandService {
	return &OrderCommandService{
		orderRepo:    orderRepo,
		orderStore:   orderStore,
		customerRepo: customerRepo,
		productRepo:  productRepo,
		inventory:    inventory,
		dispatcher:   dispatcher,
		tracker:      tracker,
	}
}

// activeOfferPrices returns product → offer price for active offers. A
// settings read failure is logged and falls back to regular prices rather
// than blocking the order.
func (s *OrderCommandService) activeOfferPrices(ctx context.Context) map[uuid.UUID]float64 {
	if s.offers == nil {
		return nil
	}
	prices, err := s.offers.ActiveOfferPrices(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not load active offers; pricing at regular prices")
		return nil
	}
	return prices
}

// Create creates a new order with items in a transaction.
// Enriches customizations, validates products, links the order to a customer
// record, and publishes the creation event.
func (s *OrderCommandService) Create(ctx context.Context, input CreateOrderInput) (dto.Order, error) {
	// If this exact key already produced an order (a true retry, or a
	// double-submit that arrived sequentially rather than concurrently),
	// return the original instead of creating another.
	if input.IdempotencyKey != nil {
		existing, err := s.orderStore.FindOrderByIdempotencyKey(ctx, *input.IdempotencyKey)
		if err == nil {
			// A key reused at another location must not reveal that order.
			if existing.LocationID != input.LocationID {
				return dto.Order{}, fmt.Errorf("%w: esta orden ya se envió desde otra sucursal", apperror.ErrConflict)
			}
			return existing, nil
		}
		if !errors.Is(err, apperror.ErrNotFound) {
			return dto.Order{}, err
		}
		// ErrNotFound is expected: no prior order for this key, proceed.
	}

	if input.LocationID == uuid.Nil {
		return dto.Order{}, scope.ErrLocationRequired
	}

	if err := s.ensureStock(ctx, input.LocationID, input.Items, nil); err != nil {
		return dto.Order{}, err
	}

	customerID, err := s.resolveCustomerByPhone(ctx, input.CustomerPhone, input.CustomerName, input.CustomerEmail)
	if err != nil {
		return dto.Order{}, err
	}

	storeInput := repository.StoreOrderInput{
		LocationID:     input.LocationID,
		IdempotencyKey: input.IdempotencyKey,
		CustomerID:     customerID,
		CustomerName:   input.CustomerName,
		CustomerPhone:  input.CustomerPhone,
		CustomerEmail:  input.CustomerEmail,
		CustomerIP:     input.CustomerIP,
		PaymentMethod:  input.PaymentMethod,
		AmountReceived: input.AmountReceived,
		OrderType:      input.OrderType,
		TableNumber:    input.TableNumber,
		Notes:          input.Notes,
		CreatedByID:    input.CreatedByID,
		Items:          toStoreOrderItemInputs(input.Items),
		BasePrices:     s.activeOfferPrices(ctx),
		// Orders taken at the counter are accepted on creation; online
		// orders wait for staff review.
		Accepted: input.CreatedByID != nil && *input.CreatedByID != "",
	}

	createCtx := auditContext(ctx, audit.ActionOrderCreated, map[string]any{
		"order_number":   storeInput.OrderNumber,
		"customer_phone": storeInput.CustomerPhone,
		"items_count":    len(storeInput.Items),
	})
	createdOrder, err := s.createOrderWithRetry(createCtx, storeInput)
	if err != nil {
		// If we lost a concurrent race on the idempotency key itself, the
		// winning order already exists — fetch and return it instead of
		// surfacing a conflict to the customer who did nothing wrong.
		if input.IdempotencyKey != nil && apperror.IsUniqueViolation(err) {
			existing, findErr := s.orderStore.FindOrderByIdempotencyKey(ctx, *input.IdempotencyKey)
			if findErr == nil && existing.LocationID == input.LocationID {
				return existing, nil
			}
		}
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
		// A unique violation on the idempotency key cannot be fixed by
		// regenerating the order number; stop retrying immediately so the
		// caller can fetch the winning order.
		if isIdempotencyKeyConflict(err) {
			return dto.Order{}, err
		}
	}

	// The conflict message is returned to the client, so keep the database
	// error out of it and log it instead.
	log.Error().Err(lastErr).Int("attempts", maxAttempts).Msg("order number generation exhausted retries")
	return dto.Order{}, fmt.Errorf("%w: no se pudo generar un número de orden (%d intentos); intenta de nuevo",
		apperror.ErrConflict, maxAttempts)
}

// isIdempotencyKeyConflict reports whether err is a Postgres unique-violation
// on the idempotency_keys primary key.
func isIdempotencyKeyConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName == "idempotency_keys_pkey"
	}
	return false
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

	approveCtx := auditContext(ctx, audit.ActionOrderApproved, map[string]any{
		"order_number": order.OrderNumber,
	})
	updated, err := s.orderRepo.Approve(approveCtx, id, &reviewerID, order.Status)
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

	rejectCtx := auditContext(ctx, audit.ActionOrderRejected, map[string]any{
		"order_number": order.OrderNumber,
		"reason":       reason,
	})
	updated, err := s.orderRepo.Reject(rejectCtx, id, reason, &reviewerID, order.Status)
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
	statusCtx := auditContext(ctx, audit.ActionOrderUpdated, map[string]any{
		"order_number": order.OrderNumber,
		"old_status":   oldStatus,
		"new_status":   status,
	})

	var updated dto.Order
	switch {
	case status == "en_proceso" && oldStatus != "en_proceso":
		orderWithItems, err := s.orderRepo.FindByIDWithItemsAndProducts(ctx, id)
		if err != nil {
			return dto.Order{}, err
		}
		deductions, err := s.inventory.DeductionsFor(ctx, orderWithItems.LocationID, orderWithItems.Items)
		if err != nil {
			return dto.Order{}, err
		}
		updated, err = s.orderStore.UpdateStatusWithStockAdjustment(statusCtx, id, status, oldStatus, deductions, false)
		if err != nil {
			return dto.Order{}, err
		}
	case status == "cancelada" && oldStatus != "cancelada":
		updated, err = s.orderStore.UpdateStatusWithStockAdjustment(statusCtx, id, status, oldStatus, nil, true)
		if err != nil {
			return dto.Order{}, err
		}
	default:
		updated, err = s.orderStore.UpdateStatus(statusCtx, id, status, oldStatus)
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
		CustomerName:   input.CustomerName,
		CustomerPhone:  input.CustomerPhone,
		CustomerEmail:  input.CustomerEmail,
		PaymentMethod:  input.PaymentMethod,
		AmountReceived: input.AmountReceived,
		OrderType:      input.OrderType,
		TableNumber:    input.TableNumber,
		Notes:          input.Notes,
		Items:          toStoreOrderItemInputs(input.Items),
		ExpectedStatus: existing.Status,
		BasePrices:     s.activeOfferPrices(ctx),
	}

	if input.CustomerPhone != existing.CustomerPhone {
		customerID, err := s.resolveCustomerByPhone(ctx, input.CustomerPhone, input.CustomerName, input.CustomerEmail)
		if err != nil {
			return dto.Order{}, err
		}
		storeInput.CustomerID = customerID
	}

	updateCtx := auditContext(ctx, audit.ActionOrderUpdated, map[string]any{
		"order_number":   existing.OrderNumber,
		"customer_phone": storeInput.CustomerPhone,
		"items_count":    len(storeInput.Items),
	})

	// Orders that have not deducted stock yet are checked against what the
	// location has now; deducted ones are checked inside the transaction.
	if len(input.Items) > 0 && !orderHasDeductedStock(existing.Status) {
		var previous []dto.OrderItem
		if current, err := s.orderRepo.FindByIDWithItemsAndProducts(ctx, id); err == nil {
			previous = current.Items
		}
		if err := s.ensureStock(ctx, existing.LocationID, input.Items, previous); err != nil {
			return dto.Order{}, err
		}
	}

	var updated dto.Order
	if len(input.Items) > 0 && orderHasDeductedStock(existing.Status) {
		newItems, err := s.resolveOrderItems(ctx, input.Items)
		if err != nil {
			return dto.Order{}, err
		}
		deductions, err := s.inventory.DeductionsFor(ctx, existing.LocationID, newItems)
		if err != nil {
			return dto.Order{}, err
		}
		updated, err = s.orderStore.UpdateOrderWithStockAdjustment(updateCtx, id, storeInput, deductions, true)
		if err != nil {
			return dto.Order{}, err
		}
	} else {
		updated, err = s.orderStore.UpdateOrder(updateCtx, id, storeInput)
		if err != nil {
			return dto.Order{}, err
		}
	}

	return updated, nil
}

// ensureStock checks the order's tracked flavours and products against the
// location's current stock. It is skipped when no inventory service is wired.
func (s *OrderCommandService) ensureStock(ctx context.Context, locationID uuid.UUID, items []OrderItemInput, previous []dto.OrderItem) error {
	if s.inventory == nil || len(items) == 0 {
		return nil
	}
	resolved, err := s.resolveOrderItems(ctx, items)
	if err != nil {
		// Unknown products are reported by order validation with a clearer message.
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.inventory.EnsureAvailable(ctx, locationID, resolved, previous)
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

// SetArchived toggles an order's archive flag. Manual archive/unarchive is
// allowed for any status and is intended for admin-only use.
func (s *OrderCommandService) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	archiveCtx := auditContext(ctx, audit.ActionOrderArchived, map[string]any{
		"order_number": order.OrderNumber,
		"archived":     archived,
	})
	return s.orderRepo.SetArchived(archiveCtx, id, archived)
}

// UpdateNotes updates an order's notes field.
func (s *OrderCommandService) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return dto.Order{}, err
	}

	noteValue := ""
	if notes != nil {
		noteValue = *notes
	}
	notesCtx := auditContext(ctx, audit.ActionOrderUpdated, map[string]any{
		"order_number": order.OrderNumber,
		"notes":        noteValue,
	})

	return s.orderRepo.UpdateNotes(notesCtx, id, notes)
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

	deleteCtx := auditContext(ctx, audit.ActionOrderDeleted, map[string]any{
		"order_number": order.OrderNumber,
	})
	if err := s.orderRepo.Delete(deleteCtx, id); err != nil {
		return err
	}

	return nil
}

// resolveCustomerByPhone returns the customer ID for the given phone, creating a
// new customer record when one does not already exist. It handles the small race
// window where two concurrent orders for the same phone could both attempt to create
// the customer by retrying the lookup on a unique-constraint violation.
func (s *OrderCommandService) resolveCustomerByPhone(ctx context.Context, phone, name string, email *string) (*uuid.UUID, error) {
	phone = strings.TrimSpace(phone)
	name = strings.TrimSpace(name)
	// Counter orders may come without a phone: there is no customer record
	// to link them to.
	if phone == "" {
		return nil, nil
	}

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

func auditContext(ctx context.Context, action string, changes map[string]any) context.Context {
	return audit.WithLogSpec(ctx, audit.LogSpec{
		Action:  action,
		ActorID: audit.ActorFromContext(ctx),
		IP:      audit.IPFromContext(ctx),
		Changes: changes,
	})
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
