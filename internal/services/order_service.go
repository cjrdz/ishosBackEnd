package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosFactory-backend/ent"
	entOrder "github.com/cjrdz/ishosFactory-backend/ent/order"
	entOrderItem "github.com/cjrdz/ishosFactory-backend/ent/orderitem"
	"github.com/cjrdz/ishosFactory-backend/internal/repository"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidOrderState  = errors.New("invalid order state")
	ErrProductNotFound    = errors.New("product not found")
	ErrProductUnavailable = errors.New("product unavailable")
)

type OrderService struct {
	db          *ent.Client
	orderRepo   *repository.OrderRepository
	productRepo *repository.ProductRepository
	whatsapp    *WhatsAppService
}

type CreateOrderInput struct {
	CustomerName  string
	CustomerPhone string
	CustomerEmail *string
	CustomerIP    *string
	PaymentMethod string
	OrderType     string
	TableNumber   *int
	Notes         *string
	Items         []OrderItemInput
	CreatedByID   *string
}

type OrderItemInput struct {
	ProductID      uuid.UUID
	Quantity       int
	Customizations map[string]interface{}
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

func NewOrderService(db *ent.Client, orderRepo *repository.OrderRepository, productRepo *repository.ProductRepository, whatsapp *WhatsAppService) *OrderService {
	return &OrderService{
		db:          db,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		whatsapp:    whatsapp,
	}
}

func (s *OrderService) ListPaginated(ctx context.Context, status string, page, limit int) ([]*ent.Order, int, error) {
	return s.orderRepo.ListPaginated(ctx, status, page, limit)
}

func (s *OrderService) GetByIDWithItems(ctx context.Context, id uuid.UUID) (*ent.Order, error) {
	return s.orderRepo.FindByIDWithItems(ctx, id)
}

func (s *OrderService) GetByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByOrderNumberAndPhone(ctx, orderNumber, customerPhone)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return order, nil
}

func (s *OrderService) Create(ctx context.Context, input CreateOrderInput) (*ent.Order, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	txProductRepo := repository.NewProductRepository(tx.Client())
	total := 0.0

	type snapshot struct {
		productID      uuid.UUID
		productName    string
		quantity       int
		unitPrice      float64
		customizations map[string]interface{}
	}
	items := make([]snapshot, 0, len(input.Items))

	for _, item := range input.Items {
		p, getErr := txProductRepo.FindByID(ctx, item.ProductID)
		if getErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, item.ProductID)
		}
		if !p.IsAvailable {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductUnavailable, p.Name)
		}

		total += p.Price * float64(item.Quantity)
		items = append(items, snapshot{
			productID:      item.ProductID,
			productName:    p.Name,
			quantity:       item.Quantity,
			unitPrice:      p.Price,
			customizations: item.Customizations,
		})
	}

	order, err := tx.Order.Create().
		SetOrderNumber(generateOrderNumber()).
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
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	for _, item := range items {
		create := tx.OrderItem.Create().
			SetOrderID(order.ID).
			SetProductID(item.productID).
			SetProductName(item.productName).
			SetQuantity(item.quantity).
			SetUnitPrice(item.unitPrice).
			SetSubtotal(item.unitPrice * float64(item.quantity))
		if item.customizations != nil {
			create = create.SetCustomizations(item.customizations)
		}
		if _, itemErr := create.Save(ctx); itemErr != nil {
			_ = tx.Rollback()
			return nil, itemErr
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	go s.whatsapp.SendOrderCreated(input.CustomerPhone, order.OrderNumber, total)
	return order, nil
}

func (s *OrderService) Approve(ctx context.Context, id uuid.UUID, reviewerID string, allowOverride bool) (*ent.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if order.Status != entOrder.StatusPendienteRevision && !(allowOverride && order.Status == entOrder.StatusCancelada) {
		return nil, ErrInvalidOrderState
	}

	updated, err := s.orderRepo.Approve(ctx, id, &reviewerID)
	if err != nil {
		return nil, err
	}

	go s.whatsapp.SendOrderApproved(order.CustomerPhone, order.OrderNumber)
	return updated, nil
}

func (s *OrderService) Reject(ctx context.Context, id uuid.UUID, reason, reviewerID string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	updated, err := s.orderRepo.Reject(ctx, id, reason, &reviewerID)
	if err != nil {
		return nil, err
	}

	go s.whatsapp.SendOrderRejected(order.CustomerPhone, order.OrderNumber, reason)
	return updated, nil
}

func (s *OrderService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	updated, err := s.orderRepo.UpdateStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}

	if status == "lista" {
		go s.whatsapp.SendOrderReady(order.CustomerPhone, order.OrderNumber)
	}

	return updated, nil
}

func (s *OrderService) Update(ctx context.Context, id uuid.UUID, input UpdateOrderInput) (*ent.Order, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	order, err := tx.Order.Get(ctx, id)
	if err != nil {
		_ = tx.Rollback()
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
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
		if _, err := update.Save(ctx); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.orderRepo.FindByIDWithItems(ctx, id)
	}

	txProductRepo := repository.NewProductRepository(tx.Client())
	total := 0.0

	type snapshot struct {
		productID      uuid.UUID
		productName    string
		quantity       int
		unitPrice      float64
		customizations map[string]interface{}
	}
	items := make([]snapshot, 0, len(input.Items))

	for _, item := range input.Items {
		p, getErr := txProductRepo.FindByID(ctx, item.ProductID)
		if getErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, item.ProductID)
		}
		if !p.IsAvailable {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductUnavailable, p.Name)
		}

		total += p.Price * float64(item.Quantity)
		items = append(items, snapshot{
			productID:      item.ProductID,
			productName:    p.Name,
			quantity:       item.Quantity,
			unitPrice:      p.Price,
			customizations: item.Customizations,
		})
	}

	if _, err := update.SetTotalAmount(total).Save(ctx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if _, err := tx.OrderItem.Delete().Where(entOrderItem.HasOrderWith(entOrder.IDEQ(order.ID))).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	for _, item := range items {
		create := tx.OrderItem.Create().
			SetOrderID(order.ID).
			SetProductID(item.productID).
			SetProductName(item.productName).
			SetQuantity(item.quantity).
			SetUnitPrice(item.unitPrice).
			SetSubtotal(item.unitPrice * float64(item.quantity))
		if item.customizations != nil {
			create = create.SetCustomizations(item.customizations)
		}
		if _, itemErr := create.Save(ctx); itemErr != nil {
			_ = tx.Rollback()
			return nil, itemErr
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.orderRepo.FindByIDWithItems(ctx, id)
}

func (s *OrderService) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (*ent.Order, error) {
	if _, err := s.orderRepo.FindByID(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	return s.orderRepo.UpdateNotes(ctx, id, notes)
}

func (s *OrderService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.orderRepo.FindByID(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return ErrOrderNotFound
		}
		return err
	}

	return s.orderRepo.Delete(ctx, id)
}

func generateOrderNumber() string {
	return fmt.Sprintf("ORD-%s-%04d", time.Now().Format("20060102"), rand.Intn(9000)+1000)
}
