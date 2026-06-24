// order_store.go — Transactional order persistence seam.
//
// OrderStore hides transaction orchestration, product validation, and
// customization enrichment behind a small interface so that services do not
// need to open transactions or import builders.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
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
	CustomerID    *uuid.UUID
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
	CustomerID    *uuid.UUID
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
	db          *bun.DB
	productRepo productResolver
	enrichment  EnrichmentService
}

// NewOrderStore creates a new transactional order store.
func NewOrderStore(db *bun.DB, productRepo productResolver, enrichment EnrichmentService) *OrderStore {
	return &OrderStore{db: db, productRepo: productRepo, enrichment: enrichment}
}

// CreateOrder creates a new order with its items inside a single transaction.
// It validates products, enriches customizations, and computes the total amount.
func (s *OrderStore) CreateOrder(ctx context.Context, input StoreOrderInput) (dto.Order, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return dto.Order{}, err
	}
	defer tx.Rollback()

	total, snapshots, err := s.resolveItems(ctx, s.productRepo, input.Items)
	if err != nil {
		return dto.Order{}, err
	}

	now := time.Now()
	order := models.Order{
		ID:            uuid.New(),
		OrderNumber:   input.OrderNumber,
		CustomerID:    input.CustomerID,
		CustomerName:  input.CustomerName,
		CustomerPhone: input.CustomerPhone,
		CustomerEmail: input.CustomerEmail,
		CustomerIP:    input.CustomerIP,
		TotalAmount:   total,
		Status:        models.OrderStatusPendienteRevision,
		PaymentMethod: models.OrderPaymentMethod(input.PaymentMethod),
		PaymentStatus: models.OrderPaymentStatusPendiente,
		OrderType:     models.OrderType(input.OrderType),
		TableNumber:   input.TableNumber,
		Notes:         input.Notes,
		CreatedByUserID: input.CreatedByID,
		StatusTimestamps: map[string]time.Time{
			string(models.OrderStatusPendienteRevision): now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := tx.NewInsert().Model(&order).Exec(ctx); err != nil {
		return dto.Order{}, err
	}

	if err := s.createItems(ctx, tx, order.ID, snapshots); err != nil {
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
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return dto.Order{}, err
	}
	defer tx.Rollback()

	var existingOrder models.Order
	if err := tx.NewSelect().Model(&existingOrder).Where("id = ?", id).For("UPDATE").Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}

	now := time.Now()
	existingOrder.CustomerName = input.CustomerName
	existingOrder.CustomerPhone = input.CustomerPhone
	existingOrder.CustomerEmail = input.CustomerEmail
	existingOrder.PaymentMethod = models.OrderPaymentMethod(input.PaymentMethod)
	existingOrder.OrderType = models.OrderType(input.OrderType)
	existingOrder.TableNumber = input.TableNumber
	existingOrder.Notes = input.Notes
	existingOrder.UpdatedAt = now
	if input.CustomerID != nil {
		existingOrder.CustomerID = input.CustomerID
	}

	if len(input.Items) == 0 {
		if _, err := tx.NewUpdate().Model(&existingOrder).WherePK().Exec(ctx); err != nil {
			return dto.Order{}, err
		}
		if err := tx.Commit(); err != nil {
			return dto.Order{}, err
		}
		return s.toDTOOrder(existingOrder, nil), nil
	}

	total, snapshots, err := s.resolveItems(ctx, s.productRepo, input.Items)
	if err != nil {
		return dto.Order{}, err
	}
	existingOrder.TotalAmount = total

	if _, err := tx.NewUpdate().Model(&existingOrder).WherePK().Exec(ctx); err != nil {
		return dto.Order{}, err
	}

	if _, err := tx.NewDelete().Model((*models.OrderItem)(nil)).Where("order_items = ?", existingOrder.ID).Exec(ctx); err != nil {
		return dto.Order{}, err
	}

	if err := s.createItems(ctx, tx, existingOrder.ID, snapshots); err != nil {
		return dto.Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return dto.Order{}, err
	}

	return s.toDTOOrder(existingOrder, snapshots), nil
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
func (s *OrderStore) createItems(ctx context.Context, tx bun.Tx, orderID uuid.UUID, snapshots []itemSnapshot) error {
	for _, item := range snapshots {
		model := models.OrderItem{
			ID:                uuid.New(),
			OrderID:           orderID,
			ProductID:         item.product.ID,
			SnapshotProductID: &item.product.ID,
			ProductName:       item.product.Name,
			Quantity:          item.quantity,
			UnitPrice:         item.unitPrice,
			Subtotal:          item.unitPrice * float64(item.quantity),
			Customizations:    item.customizations,
		}
		if _, err := tx.NewInsert().Model(&model).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// toDTOOrder builds a dto.Order from a persisted models.Order and resolved snapshots.
func (s *OrderStore) toDTOOrder(order models.Order, snapshots []itemSnapshot) dto.Order {
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
		CustomerID:       order.CustomerID,
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
