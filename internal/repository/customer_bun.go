// customer_bun.go — Bun-backed repository for customer CRUD and order lookup by phone.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// CustomerBunRepository implements services.CustomerRepository using Bun.
type CustomerBunRepository struct {
	db bun.IDB
}

// NewCustomerBunRepository creates a new Bun-backed customer repository.
func NewCustomerBunRepository(db bun.IDB) *CustomerBunRepository {
	return &CustomerBunRepository{db: db}
}

// List returns customers filtered by status and/or search term.
func (r *CustomerBunRepository) List(ctx context.Context, status, search string) ([]dto.Customer, error) {
	var rows []models.Customer
	q := r.db.NewSelect().Model(&rows).Order("created_at DESC")

	if status != "" {
		q = q.Where("status = ?", status)
	}

	if term := strings.TrimSpace(search); term != "" {
		pattern := "%" + term + "%"
		q = q.Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ?", pattern, pattern, pattern)
	}

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	return toCustomerDTOsFromModels(rows), nil
}

// ListPaginated returns a paginated list of customers and the total count.
func (r *CustomerBunRepository) ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error) {
	q := r.db.NewSelect().Model((*models.Customer)(nil))

	if status != "" {
		q = q.Where("status = ?", status)
	}
	if term := strings.TrimSpace(search); term != "" {
		pattern := "%" + term + "%"
		q = q.Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ?", pattern, pattern, pattern)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var rows []models.Customer
	if err := q.Model(&rows).Order("created_at DESC").Offset(offset).Limit(limit).Scan(ctx); err != nil {
		return nil, 0, err
	}
	return toCustomerDTOsFromModels(rows), total, nil
}

// FindByID returns a customer by ID.
func (r *CustomerBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Customer, error) {
	var row models.Customer
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Customer{}, &apperror.NotFoundError{Resource: "customer", Cause: err}
		}
		return dto.Customer{}, err
	}
	return toCustomerDTOFromModel(row), nil
}

// FindByPhone returns a customer by phone number.
func (r *CustomerBunRepository) FindByPhone(ctx context.Context, phone string) (dto.Customer, error) {
	var row models.Customer
	if err := r.db.NewSelect().Model(&row).Where("phone = ?", phone).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Customer{}, &apperror.NotFoundError{Resource: "customer", Cause: err}
		}
		return dto.Customer{}, err
	}
	return toCustomerDTOFromModel(row), nil
}

// Create creates a new customer.
func (r *CustomerBunRepository) Create(ctx context.Context, input CreateCustomerInput) (dto.Customer, error) {
	now := time.Now()
	row := models.Customer{
		ID:        uuid.New(),
		Name:      input.Name,
		UserType:  models.CustomerUserType(input.UserType),
		Phone:     input.Phone,
		Email:     input.Email,
		Status:    models.CustomerStatus(input.Status),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := r.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return dto.Customer{}, err
	}
	return toCustomerDTOFromModel(row), nil
}

// Update updates an existing customer.
func (r *CustomerBunRepository) Update(ctx context.Context, id uuid.UUID, input UpdateCustomerInput) (dto.Customer, error) {
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return dto.Customer{}, err
	}

	row := models.Customer{
		ID:        existing.ID,
		Name:      existing.Name,
		UserType:  models.CustomerUserType(existing.UserType),
		Phone:     existing.Phone,
		Email:     existing.Email,
		Status:    models.CustomerStatus(existing.Status),
		CreatedAt: existing.CreatedAt,
		UpdatedAt: time.Now(),
	}

	if input.Name != nil {
		row.Name = *input.Name
	}
	if input.UserType != nil {
		row.UserType = models.CustomerUserType(*input.UserType)
	}
	if input.Phone != nil {
		row.Phone = *input.Phone
	}
	if input.Email != nil {
		if strings.TrimSpace(*input.Email) == "" {
			row.Email = nil
		} else {
			row.Email = input.Email
		}
	}
	if input.Status != nil {
		row.Status = models.CustomerStatus(*input.Status)
	}

	res, err := r.db.NewUpdate().Model(&row).WherePK().Exec(ctx)
	if err != nil {
		return dto.Customer{}, err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return dto.Customer{}, &apperror.NotFoundError{Resource: "customer"}
	}

	return toCustomerDTOFromModel(row), nil
}

// Delete deletes a customer by ID.
func (r *CustomerBunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	row := models.Customer{ID: id}
	res, err := r.db.NewDelete().Model(&row).WherePK().Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &apperror.NotFoundError{Resource: "customer", Cause: err}
		}
		return err
	}
	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return &apperror.NotFoundError{Resource: "customer"}
	}
	return nil
}

// ListOrdersByCustomerID returns the most recent orders for a customer.
func (r *CustomerBunRepository) ListOrdersByCustomerID(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error) {
	var orders []models.Order
	q := r.db.NewSelect().Model(&orders).Where("customer_id = ?", customerID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	orderIDs := make([]uuid.UUID, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}

	itemsByOrder := make(map[uuid.UUID][]models.OrderItem)
	if len(orderIDs) > 0 {
		var items []models.OrderItem
		if err := r.db.NewSelect().Model(&items).Where("order_items IN (?)", bun.In(orderIDs)).Scan(ctx); err != nil {
			return nil, err
		}
		for _, item := range items {
			itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
		}
	}

	result := make([]dto.Order, len(orders))
	for i, o := range orders {
		itemDTOs := make([]dto.OrderItem, len(itemsByOrder[o.ID]))
		for j, item := range itemsByOrder[o.ID] {
			itemDTOs[j] = toOrderItemDTOFromModel(item, nil)
		}
		result[i] = toOrderDTOFromModel(o, itemDTOs)
	}
	return result, nil
}

func toCustomerDTOsFromModels(rows []models.Customer) []dto.Customer {
	result := make([]dto.Customer, len(rows))
	for i, row := range rows {
		result[i] = toCustomerDTOFromModel(row)
	}
	return result
}

func toCustomerDTOFromModel(c models.Customer) dto.Customer {
	return dto.Customer{
		ID:        c.ID,
		Name:      c.Name,
		UserType:  string(c.UserType),
		Phone:     c.Phone,
		Email:     c.Email,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func toOrderDTOFromModel(o models.Order, items []dto.OrderItem) dto.Order {
	order := dto.Order{
		ID:               o.ID,
		OrderNumber:      o.OrderNumber,
		CustomerID:       o.CustomerID,
		CustomerName:     o.CustomerName,
		CustomerPhone:    o.CustomerPhone,
		CustomerEmail:    o.CustomerEmail,
		CustomerIP:       o.CustomerIP,
		TotalAmount:      o.TotalAmount,
		Status:           string(o.Status),
		PaymentMethod:    string(o.PaymentMethod),
		PaymentStatus:    string(o.PaymentStatus),
		OrderType:        string(o.OrderType),
		TableNumber:      o.TableNumber,
		Notes:            o.Notes,
		RejectionReason:  o.RejectionReason,
		ReviewedByUserID: o.ReviewedByUserID,
		ReviewedAt:       o.ReviewedAt,
		CreatedByUserID:  o.CreatedByUserID,
		IsArchived:       o.IsArchived,
		StatusTimestamps: o.StatusTimestamps,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
		Items:            items,
	}
	return order
}

func toOrderItemDTOFromModel(item models.OrderItem, product *dto.Product) dto.OrderItem {
	dtoItem := dto.OrderItem{
		ID:             item.ID,
		OrderID:        item.OrderID,
		ProductName:    item.ProductName,
		Quantity:       item.Quantity,
		UnitPrice:      item.UnitPrice,
		Subtotal:       item.Subtotal,
		Customizations: item.Customizations,
	}
	if item.SnapshotProductID != nil && *item.SnapshotProductID != uuid.Nil {
		dtoItem.ProductID = *item.SnapshotProductID
	} else {
		dtoItem.ProductID = item.ProductID
	}
	if product != nil {
		dtoItem.Product = product
	}
	return dtoItem
}
