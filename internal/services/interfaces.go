// interfaces.go — Service interface contracts consumed by HTTP handlers.
package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

// Service interfaces let handlers depend on behavior contracts instead of concrete structs.
type OrderServiceInterface interface {
	IssueTrackingAccess(ctx context.Context, orderNumber string) (*TrackingAccess, error)
	ListPaginated(ctx context.Context, status string, page, limit int) ([]*ent.Order, int, error)
	GetByIDWithItems(ctx context.Context, id uuid.UUID) (*ent.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*ent.Order, error)
	GetByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (*ent.Order, error)
	ListRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]*ent.Order, error)
	Create(ctx context.Context, input CreateOrderInput) (*ent.Order, error)
	Approve(ctx context.Context, id uuid.UUID, reviewerID string, allowOverride bool) (*ent.Order, error)
	Reject(ctx context.Context, id uuid.UUID, reason, reviewerID string) (*ent.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*ent.Order, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateOrderInput) (*ent.Order, error)
	UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (*ent.Order, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProductServiceInterface interface {
	List(ctx context.Context, categorySlug string, includeAll bool) ([]*ent.Product, error)
	GetByIDWithCategory(ctx context.Context, id uuid.UUID) (*ent.Product, error)
	Create(ctx context.Context, input repository.CreateProductInput) (*ent.Product, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (*ent.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ToggleAvailability(ctx context.Context, id uuid.UUID) (*ent.Product, error)
	LinkFlavor(ctx context.Context, productID, flavorID uuid.UUID) error
	UnlinkFlavor(ctx context.Context, productID, flavorID uuid.UUID) error
	LinkAddon(ctx context.Context, productID, addonID uuid.UUID) error
	UnlinkAddon(ctx context.Context, productID, addonID uuid.UUID) error
}

type FlavorServiceInterface interface {
	List(ctx context.Context, onlyActive bool) ([]*ent.Flavor, error)
	GetByID(ctx context.Context, id uuid.UUID) (*ent.Flavor, error)
	Create(ctx context.Context, input repository.CreateFlavorInput) (*ent.Flavor, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (*ent.Flavor, error)
	Delete(ctx context.Context, id uuid.UUID) error
	LinkToProduct(ctx context.Context, flavorID, productID uuid.UUID) error
	UnlinkFromProduct(ctx context.Context, flavorID, productID uuid.UUID) error
}

type AddonServiceInterface interface {
	List(ctx context.Context, onlyActive bool) ([]*ent.Addon, error)
	GetByID(ctx context.Context, id uuid.UUID) (*ent.Addon, error)
	Create(ctx context.Context, input repository.CreateAddonInput) (*ent.Addon, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (*ent.Addon, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CustomerServiceInterface interface {
	List(ctx context.Context, status, search string) ([]*ent.Customer, error)
	ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]*ent.Customer, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*ent.Customer, error)
	Create(ctx context.Context, input repository.CreateCustomerInput) (*ent.Customer, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (*ent.Customer, error)
	Delete(ctx context.Context, id uuid.UUID) error
	OrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit int) ([]*ent.Order, error)
}
