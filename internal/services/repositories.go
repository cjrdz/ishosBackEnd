// repositories.go — Repository interface contracts consumed by services.
//
// These interfaces define the data access contracts that services depend on.
// Defining interfaces at the consumer (service) level, not the provider (repository)
// level, allows services to be easily tested with mock implementations.
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
)

// ProductRepository defines the product data access contract.
type ProductRepository interface {
	FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]dto.Product, error)
	FindAllPaginated(ctx context.Context, categorySlug string, onlyAvailable bool, page, perPage int) ([]dto.Product, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Product, error)
	FindByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error)
	Create(ctx context.Context, input repository.CreateProductInput) (dto.Product, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (dto.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ToggleAvailability(ctx context.Context, id uuid.UUID) (dto.Product, error)
}

// OrderRepository defines the order data access contract.
type OrderRepository interface {
	ListPaginated(ctx context.Context, loc scope.Location, status string, archived bool, page, limit int) ([]dto.Order, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Order, error)
	FindByIDWithItems(ctx context.Context, id uuid.UUID) (dto.Order, error)
	FindByIDWithItemsAndProducts(ctx context.Context, id uuid.UUID) (dto.Order, error)
	FindByOrderNumber(ctx context.Context, orderNumber string) (dto.Order, error)
	FindByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (dto.Order, error)
	FindRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]dto.Order, error)
	Create(ctx context.Context, input repository.CreateOrderInput) (dto.Order, error)
	// Approve, Reject and UpdateStatus take the status the caller validated the
	// transition against; the repository re-checks it under a row lock and
	// returns apperror.ErrConflict if the order changed concurrently.
	// Reject also returns any stock the order still holds.
	Approve(ctx context.Context, id uuid.UUID, reviewerID *string, expectedStatus string) (dto.Order, error)
	Reject(ctx context.Context, id uuid.UUID, reason string, reviewerID *string, expectedStatus string) (dto.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, expectedStatus string) (dto.Order, error)
	UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (dto.Order, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (dto.Order, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Reporting and bulk operations
	FindByDateRange(ctx context.Context, loc scope.Location, start, end time.Time, statuses []string) ([]dto.Order, error)
	FindByDateRangeWithItems(ctx context.Context, loc scope.Location, start, end time.Time, statuses []string) ([]dto.Order, error)
	FindOrderItemsByOrderDateRange(ctx context.Context, loc scope.Location, start, end time.Time, excludeStatuses []string) ([]dto.OrderItem, error)
	Purge(ctx context.Context, loc scope.Location, start, end time.Time, statuses []string) (int, int, error)

	// SQL-aggregated analytics (avoid loading full date ranges into memory).
	AnalyticsOverview(ctx context.Context, loc scope.Location, start, end time.Time) (dto.AnalyticsOverview, error)
	AnalyticsOrdersOverTime(ctx context.Context, loc scope.Location, start, end time.Time, groupBy string) ([]dto.AnalyticsTimelinePoint, error)
	AnalyticsTopProducts(ctx context.Context, loc scope.Location, start, end time.Time, limit int) ([]dto.AnalyticsTopProduct, error)

	// Streaming export rows (CSV) without loading the full date range into memory.
	StreamOrderExportRows(ctx context.Context, loc scope.Location, start, end time.Time, statuses []string, batchSize int, cb func(dto.ExportOrderRow) error) error

	// Archiving
	FindEligibleForArchive(ctx context.Context, terminalStatuses []string, cutoff time.Time, limit int) ([]dto.Order, error)
	ArchiveOrders(ctx context.Context, ids []uuid.UUID) (int, error)
}

// FlavorRepository defines the flavor data access contract.
type FlavorRepository interface {
	FindAll(ctx context.Context, onlyActive bool) ([]dto.Flavor, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Flavor, error)
	Create(ctx context.Context, input repository.CreateFlavorInput) (dto.Flavor, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (dto.Flavor, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// AddonRepository defines the addon data access contract.
type AddonRepository interface {
	FindAll(ctx context.Context, onlyActive bool) ([]dto.Addon, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Addon, error)
	Create(ctx context.Context, input repository.CreateAddonInput) (dto.Addon, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (dto.Addon, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// CategoryRepository defines the category data access contract.
type CategoryRepository interface {
	FindAll(ctx context.Context) ([]dto.Category, error)
	FindActive(ctx context.Context) ([]dto.Category, error)
	FindAllPaginated(ctx context.Context, showAll bool, page, perPage int) ([]dto.Category, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Category, error)
	FindBySlug(ctx context.Context, slug string) (dto.Category, error)
	FindBySlugWithProducts(ctx context.Context, slug string) (dto.Category, error)
	Create(ctx context.Context, input repository.CreateCategoryInput) (dto.Category, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateCategoryInput) (dto.Category, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// CustomerRepository defines the customer data access contract.
type CustomerRepository interface {
	List(ctx context.Context, status, search string) ([]dto.Customer, error)
	ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Customer, error)
	FindByPhone(ctx context.Context, phone string) (dto.Customer, error)
	Create(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (dto.Customer, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListOrdersByCustomerID(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error)
}

// AdminSettingRepository defines the admin settings data access contract.
type AdminSettingRepository interface {
	GetByKey(ctx context.Context, key string) (dto.AdminSetting, error)
	UpsertByKeyWithAudit(ctx context.Context, key string, value map[string]any, changedBy string) (dto.AdminSetting, error)
}

// OrderStore encapsulates the transactional persistence of orders and their items.
// It hides Bun transaction orchestration, product validation, and enrichment behind a small interface.
type OrderStore interface {
	CreateOrder(ctx context.Context, input repository.StoreOrderInput) (dto.Order, error)
	FindOrderByIdempotencyKey(ctx context.Context, key uuid.UUID) (dto.Order, error)
	UpdateOrder(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput) (dto.Order, error)
	// UpdateOrderWithStockAdjustment updates an order and replaces its items, then
	// applies an inventory adjustment inside the same transaction. When restore is
	// set, the order's outstanding stock is returned first; deductions are then
	// applied. Set input.ExpectedStatus to guard against concurrent changes.
	UpdateOrderWithStockAdjustment(ctx context.Context, id uuid.UUID, input repository.StoreUpdateOrderInput, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, expectedStatus string) (dto.Order, error)
	// UpdateStatusWithStockAdjustment updates an order's status and applies an
	// inventory adjustment inside the same transaction: restore first (if set),
	// then deductions. expectedStatus behaves as in OrderRepository.UpdateStatus.
	UpdateStatusWithStockAdjustment(ctx context.Context, id uuid.UUID, status, expectedStatus string, deductions []repository.StockDeductionInput, restore bool) (dto.Order, error)
}

// ContainerTypeRepository defines the container type data access contract.
type ContainerTypeRepository interface {
	List(ctx context.Context) ([]dto.ContainerType, error)
	GetByID(ctx context.Context, id uuid.UUID) (dto.ContainerType, error)
	Create(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error)
	Update(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// TrackingTokenStore defines the persistence contract for order tracking tokens.
type TrackingTokenStore interface {
	Store(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error
	Lookup(ctx context.Context, orderNumber, tokenHash string) (*dto.TrackingTokenRecord, error)
	RevokeAllByOrder(ctx context.Context, orderNumber string) error
}

// InventoryRepository defines the inventory data access contract.
type InventoryRepository interface {
	List(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error)
	ListByType(ctx context.Context, loc scope.Location, itemType string) ([]dto.InventoryItem, error)
	ListLinked(ctx context.Context, loc scope.Location) ([]dto.InventoryItem, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.InventoryItem, error)
	FindByFlavorID(ctx context.Context, locationID, flavorID uuid.UUID) (dto.InventoryItem, error)
	FindByProductID(ctx context.Context, locationID, productID uuid.UUID) (dto.InventoryItem, error)
	Create(ctx context.Context, input repository.CreateInventoryItemInput) (dto.InventoryItem, error)
	Update(ctx context.Context, id uuid.UUID, input repository.UpdateInventoryItemInput) (dto.InventoryItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
	LinkToProduct(ctx context.Context, inventoryItemID, productID uuid.UUID) (dto.InventoryItem, error)
	UnlinkFromProduct(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error)
	LinkToFlavor(ctx context.Context, inventoryItemID, flavorID uuid.UUID) (dto.InventoryItem, error)
	UnlinkFromFlavor(ctx context.Context, inventoryItemID uuid.UUID) (dto.InventoryItem, error)

	FindStockMovementsByOrder(ctx context.Context, orderID uuid.UUID) ([]dto.StockMovement, error)
	FindStockMovementsByInventoryItem(ctx context.Context, inventoryItemID uuid.UUID, limit, offset int) ([]dto.StockMovement, int, error)

	DeductStockForOrder(ctx context.Context, orderID uuid.UUID, deductions []repository.StockDeductionInput, createdByID string) ([]dto.StockMovement, error)
	RestoreStockForOrder(ctx context.Context, orderID uuid.UUID, createdByID string) ([]dto.StockMovement, error)
	RecordEntry(ctx context.Context, input repository.RecordEntryInput) (dto.StockMovement, error)
	RecordAdjustment(ctx context.Context, input repository.RecordAdjustmentInput) (dto.StockMovement, error)
	RecordUnitEntry(ctx context.Context, input repository.RecordUnitEntryInput) (dto.StockMovement, error)
}
