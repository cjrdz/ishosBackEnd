// order_store.go — Transactional order persistence seam.
//
// OrderStore hides Ent transaction orchestration, product validation, and
// customization enrichment behind a small interface so that services do not
// need to open transactions or import Ent order/orderitem builders.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// StoreOrderItemInput is the raw order item supplied to OrderStore.
type StoreOrderItemInput struct {
	ProductID      uuid.UUID
	Quantity       int
	Customizations map[string]any
}

// StoreOrderInput is the raw data required to create an order transactionally.
type StoreOrderInput struct {
	OrderNumber   string
	CustomerName  string
	CustomerPhone string
	CustomerEmail *string
	CustomerIP    *string
	PaymentMethod string
	OrderType     string
	TableNumber   *int
	Notes         *string
	CreatedByID   *string
	Items         []StoreOrderItemInput
}

// StoreUpdateOrderInput is the raw data required to update an order transactionally.
type StoreUpdateOrderInput struct {
	CustomerName  string
	CustomerPhone string
	CustomerEmail *string
	PaymentMethod string
	OrderType     string
	TableNumber   *int
	Notes         *string
	Items         []StoreOrderItemInput
}

// EnrichmentService validates and enriches order customizations with pricing.
// The production implementation lives in the services package; this interface
// lets OrderStore stay in the repository package without creating an import cycle.
type EnrichmentService interface {
	EnrichCustomizations(product dto.Product, raw map[string]any) (map[string]any, float64, error)
}

// productResolver is the minimal surface OrderStore needs from product lookup.
type productResolver interface {
	FindByIDWithCategoryAndOptions(ctx context.Context, id uuid.UUID) (dto.Product, error)
}

// OrderStore implements transactional order creation and update.
type OrderStore struct {
	db         *ent.Client
	enrichment EnrichmentService
}

// NewOrderStore creates a new transactional order store.
func NewOrderStore(db *ent.Client, enrichment EnrichmentService) *OrderStore {
	return &OrderStore{db: db, enrichment: enrichment}
}

// CreateOrder creates a new order with its items inside a single transaction.
// It validates products, enriches customizations, and computes the total amount.
func (s *OrderStore) CreateOrder(ctx context.Context, input StoreOrderInput) (dto.Order, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return dto.Order{}, err
	}

	txProductRepo := NewProductRepository(tx.Client())

	total, snapshots, err := s.resolveItems(ctx, txProductRepo, input.Items)
	if err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	now := time.Now()
	order, err := tx.Order.Create().
		SetOrderNumber(input.OrderNumber).
		SetCustomerName(input.CustomerName).
		SetCustomerPhone(input.CustomerPhone).
		SetNillableCustomerEmail(input.CustomerEmail).
		SetNillableCustomerIP(input.CustomerIP).
		SetTotalAmount(total).
		SetStatus(entOrder.StatusPendienteRevision).
		SetPaymentMethod(entOrder.PaymentMethod(input.PaymentMethod)).
		SetOrderType(entOrder.OrderType(input.OrderType)).
		SetNillableTableNumber(input.TableNumber).
		SetNillableNotes(input.Notes).
		SetNillableCreatedByUserID(input.CreatedByID).
		SetStatusTimestamps(map[string]time.Time{
			string(entOrder.StatusPendienteRevision): now,
		}).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	if err := s.createItems(ctx, tx, order.ID, snapshots); err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return dto.Order{}, err
	}

	return s.toDTOOrder(order, snapshots), nil
}

// UpdateOrder updates an order and replaces its items inside a single transaction.
// When Items is empty, only the order metadata is updated and existing items are kept.
func (s *OrderStore) UpdateOrder(ctx context.Context, id uuid.UUID, input StoreUpdateOrderInput) (dto.Order, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return dto.Order{}, err
	}

	existingOrder, err := tx.Order.Get(ctx, id)
	if err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	update := tx.Order.UpdateOneID(id).
		SetCustomerName(input.CustomerName).
		SetCustomerPhone(input.CustomerPhone).
		SetPaymentMethod(entOrder.PaymentMethod(input.PaymentMethod)).
		SetOrderType(entOrder.OrderType(input.OrderType)).
		SetNillableCustomerEmail(input.CustomerEmail).
		SetNillableNotes(input.Notes).
		SetUpdatedAt(time.Now())

	if input.TableNumber == nil {
		update = update.ClearTableNumber()
	} else {
		update = update.SetTableNumber(*input.TableNumber)
	}

	if len(input.Items) == 0 {
		updated, err := update.Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return dto.Order{}, err
		}
		if err := tx.Commit(); err != nil {
			return dto.Order{}, err
		}
		return s.toDTOOrder(updated, nil), nil
	}

	txProductRepo := NewProductRepository(tx.Client())

	total, snapshots, err := s.resolveItems(ctx, txProductRepo, input.Items)
	if err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	updated, err := update.SetTotalAmount(total).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	if _, err := tx.OrderItem.Delete().Where(entOrderItem.HasOrderWith(entOrder.IDEQ(existingOrder.ID))).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	if err := s.createItems(ctx, tx, existingOrder.ID, snapshots); err != nil {
		_ = tx.Rollback()
		return dto.Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return dto.Order{}, err
	}

	return s.toDTOOrder(updated, snapshots), nil
}

// itemSnapshot holds resolved item data ready for persistence.
type itemSnapshot struct {
	product        dto.Product
	quantity       int
	unitPrice      float64
	customizations map[string]any
}

// resolveItems validates products, enriches customizations, and computes pricing.
func (s *OrderStore) resolveItems(ctx context.Context, productRepo productResolver, items []StoreOrderItemInput) (float64, []itemSnapshot, error) {
	total := 0.0
	snapshots := make([]itemSnapshot, 0, len(items))

	for _, item := range items {
		p, err := productRepo.FindByIDWithCategoryAndOptions(ctx, item.ProductID)
		if err != nil {
			return 0, nil, fmt.Errorf("%w: %s", apperror.ErrNotFound, item.ProductID)
		}
		if !p.IsAvailable {
			return 0, nil, fmt.Errorf("%w: %s", apperror.ErrConflict, p.Name)
		}

		enriched, addonExtra, err := s.enrichment.EnrichCustomizations(p, item.Customizations)
		if err != nil {
			return 0, nil, err
		}
		effectivePrice := p.Price + addonExtra

		total += effectivePrice * float64(item.Quantity)
		snapshots = append(snapshots, itemSnapshot{
			product:        p,
			quantity:       item.Quantity,
			unitPrice:      effectivePrice,
			customizations: enriched,
		})
	}

	return total, snapshots, nil
}

// createItems persists resolved item snapshots against an open transaction.
func (s *OrderStore) createItems(ctx context.Context, tx *ent.Tx, orderID uuid.UUID, snapshots []itemSnapshot) error {
	for _, item := range snapshots {
		create := tx.OrderItem.Create().
			SetOrderID(orderID).
			SetProductID(item.product.ID).
			SetSnapshotProductID(item.product.ID).
			SetProductName(item.product.Name).
			SetQuantity(item.quantity).
			SetUnitPrice(item.unitPrice).
			SetSubtotal(item.unitPrice * float64(item.quantity))
		if item.customizations != nil {
			create = create.SetCustomizations(item.customizations)
		}
		if _, err := create.Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

// toDTOOrder builds a dto.Order from a persisted ent.Order and resolved snapshots.
func (s *OrderStore) toDTOOrder(order *ent.Order, snapshots []itemSnapshot) dto.Order {
	if order == nil {
		return dto.Order{}
	}

	items := make([]dto.OrderItem, 0, len(snapshots))
	for _, snap := range snapshots {
		items = append(items, dto.OrderItem{
			OrderID:        order.ID,
			ProductID:      snap.product.ID,
			ProductName:    snap.product.Name,
			Quantity:       snap.quantity,
			UnitPrice:      snap.unitPrice,
			Subtotal:       snap.unitPrice * float64(snap.quantity),
			Customizations: snap.customizations,
			Product:        &snap.product,
		})
	}

	return dto.Order{
		ID:               order.ID,
		OrderNumber:      order.OrderNumber,
		CustomerName:     order.CustomerName,
		CustomerPhone:    order.CustomerPhone,
		CustomerEmail:    order.CustomerEmail,
		CustomerIP:       order.CustomerIP,
		TotalAmount:      order.TotalAmount,
		Status:           string(order.Status),
		PaymentMethod:    string(order.PaymentMethod),
		PaymentStatus:    string(order.PaymentStatus),
		OrderType:        string(order.OrderType),
		TableNumber:      order.TableNumber,
		Notes:            order.Notes,
		RejectionReason:  order.RejectionReason,
		ReviewedByUserID: order.ReviewedByUserID,
		ReviewedAt:       order.ReviewedAt,
		CreatedByUserID:  order.CreatedByUserID,
		IsArchived:       order.IsArchived,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
		StatusTimestamps: order.StatusTimestamps,
		Items:            items,
	}
}
