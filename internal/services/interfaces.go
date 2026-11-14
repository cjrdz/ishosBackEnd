// interfaces.go — Service interface contracts consumed by HTTP handlers.
package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

// Service interfaces let handlers depend on behavior contracts instead of concrete structs.
type OrderServiceInterface interface {
	IssueTrackingAccess(ctx context.Context, orderNumber string) (*TrackingAccess, error)
	ListPaginated(ctx context.Context, loc scope.Location, status string, archived bool, page, limit int) ([]dto.Order, int, error)
	GetByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error)
	GetByIDWithItemsAndProducts(ctx context.Context, id uuid.UUID) (dto.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error)
	GetByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error)
	ListRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error)
	Create(ctx context.Context, input CreateOrderInput) (dto.Order, error)
	Approve(ctx context.Context, id uuid.UUID, reviewerID string, allowOverride bool) (dto.Order, error)
	Reject(ctx context.Context, id uuid.UUID, reason, reviewerID string) (dto.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (dto.Order, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateOrderInput) (dto.Order, error)
	UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error)
	Delete(ctx context.Context, id uuid.UUID) error
	IsOrderEditable(order dto.Order) bool
}

type ProductServiceInterface interface {
	List(ctx context.Context, categorySlug string, includeAll bool) ([]dto.Product, error)
	ListPaginated(ctx context.Context, categorySlug string, includeAll bool, page, perPage int) ([]dto.Product, int, error)
	GetByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error)
	Create(ctx context.Context, input repository.CreateProductInput) (dto.Product, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (dto.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ToggleAvailability(ctx context.Context, id uuid.UUID) (dto.Product, error)
}

type FlavorServiceInterface interface {
	List(ctx context.Context, onlyActive bool) ([]dto.Flavor, error)
	GetByID(ctx context.Context, id uuid.UUID) (dto.Flavor, error)
	Create(ctx context.Context, input repository.CreateFlavorInput) (dto.Flavor, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (dto.Flavor, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AddonServiceInterface interface {
	List(ctx context.Context, onlyActive bool) ([]dto.Addon, error)
	GetByID(ctx context.Context, id uuid.UUID) (dto.Addon, error)
	Create(ctx context.Context, input repository.CreateAddonInput) (dto.Addon, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (dto.Addon, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CategoryServiceInterface interface {
	List(ctx context.Context, showAll bool) ([]dto.Category, error)
	ListPaginated(ctx context.Context, showAll bool, page, perPage int) ([]dto.Category, int, error)
	GetBySlug(ctx context.Context, slug string) (dto.Category, error)
	Create(ctx context.Context, input repository.CreateCategoryInput) (dto.Category, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateCategoryInput) (dto.Category, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CustomerServiceInterface interface {
	List(ctx context.Context, status, search string) ([]dto.Customer, error)
	ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (dto.Customer, error)
	FindByPhone(ctx context.Context, phone string) (dto.Customer, error)
	Create(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error)
	UpsertByPhone(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (dto.Customer, error)
	Delete(ctx context.Context, id uuid.UUID) error
	OrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error)
}

type ContainerTypeServiceInterface interface {
	List(ctx context.Context) ([]dto.ContainerType, error)
	GetByID(ctx context.Context, id uuid.UUID) (dto.ContainerType, error)
	Create(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error)
	Update(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
