package openapi

import (
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// PaginationMetadata mirrors pkg/response.PaginationMetadata.
type PaginationMetadata struct {
	Page       int `json:"page" example:"1" doc:"Current page number"`
	PerPage    int `json:"per_page" example:"20" doc:"Number of items per page"`
	Total      int `json:"total" example:"100" doc:"Total number of items"`
	TotalPages int `json:"total_pages" example:"5" doc:"Total number of pages"`
}

// PaginatedResponse mirrors pkg/response.Paginated.
type PaginatedResponse[T any] struct {
	Data       []T                `json:"data" doc:"Page items"`
	Pagination PaginationMetadata `json:"pagination" doc:"Pagination metadata"`
}

// Product is the public product shape.
type Product struct {
	ID                   string    `json:"id" format:"uuid" doc:"Product ID"`
	Name                 string    `json:"name" example:"Stracciatella" doc:"Product name"`
	Description          string    `json:"description" doc:"Product description"`
	Price                float64   `json:"price" example:"5.50" doc:"Product price"`
	CategoryID           *string   `json:"category_id,omitempty" format:"uuid" doc:"Category ID"`
	CategoryName         *string   `json:"category_name,omitempty" example:"Gelatos" doc:"Category name"`
	ImageURL             *string   `json:"image_url,omitempty" doc:"Public image URL"`
	ImagePath            *string   `json:"image_path,omitempty" doc:"Image object path"`
	IsAvailable          bool      `json:"is_available" example:"true" doc:"Whether the product is available"`
	ExcludeGlobalFlavors *bool     `json:"exclude_global_flavors,omitempty" example:"false" doc:"Whether global flavors are excluded"`
	ExcludeGlobalAddons  *bool     `json:"exclude_global_addons,omitempty" example:"false" doc:"Whether global addons are excluded"`
	BallBased            *bool     `json:"ball_based,omitempty" example:"false" doc:"Whether pricing is ball-based"`
	BallQuantity         *int      `json:"ball_quantity,omitempty" example:"0" doc:"Ball quantity"`
	AllowsMixedFlavors   *bool     `json:"allows_mixed_flavors,omitempty" example:"false" doc:"Whether mixed flavors are allowed"`
	StockStatus          *string   `json:"stock_status,omitempty" example:"in_stock" doc:"Stock status"`
	Flavors              []Flavor  `json:"flavors,omitempty" doc:"Product flavors"`
	Addons               []Addon   `json:"addons,omitempty" doc:"Product addons"`
	CreatedAt            time.Time `json:"created_at" doc:"Creation timestamp"`
}

// Flavor is a public flavor option.
type Flavor struct {
	ID           string    `json:"id" format:"uuid" doc:"Flavor ID"`
	Name         string    `json:"name" example:"Chocolate" doc:"Flavor name"`
	DisplayOrder int       `json:"display_order" example:"1" doc:"Display order"`
	IsSeasonal   bool      `json:"is_seasonal" example:"false" doc:"Whether the flavor is seasonal"`
	IsActive     bool      `json:"is_active" example:"true" doc:"Whether the flavor is active"`
	CreatedAt    time.Time `json:"created_at" doc:"Creation timestamp"`
}

// Addon is a public addon option.
type Addon struct {
	ID           string    `json:"id" format:"uuid" doc:"Addon ID"`
	Name         string    `json:"name" example:"Extra scoop" doc:"Addon name"`
	Price        float64   `json:"price" example:"1.00" doc:"Addon price"`
	GroupName    string    `json:"group_name" example:"Extras" doc:"Addon group name"`
	DisplayOrder int       `json:"display_order" example:"1" doc:"Display order"`
	IsActive     bool      `json:"is_active" example:"true" doc:"Whether the addon is active"`
	CreatedAt    time.Time `json:"created_at" doc:"Creation timestamp"`
}

// Category is the public category shape.
type Category struct {
	ID           string    `json:"id" format:"uuid" doc:"Category ID"`
	Name         string    `json:"name" example:"Gelatos" doc:"Category name"`
	Slug         string    `json:"slug" example:"gelatos" doc:"URL slug"`
	Description  string    `json:"description" doc:"Category description"`
	ImagePath    *string   `json:"image_path,omitempty" doc:"Image object path"`
	DisplayOrder int       `json:"display_order" example:"1" doc:"Display order"`
	IsActive     bool      `json:"is_active" example:"true" doc:"Whether the category is active"`
	CreatedAt    time.Time `json:"created_at" doc:"Creation timestamp"`
}

// CategoryDetail extends Category with its products.
type CategoryDetail struct {
	Category
	Products []ProductSummary `json:"products" doc:"Products in this category"`
}

// ProductSummary is a lightweight product shape embedded in category detail responses.
type ProductSummary struct {
	ID          string    `json:"id" format:"uuid" doc:"Product ID"`
	Name        string    `json:"name" example:"Stracciatella" doc:"Product name"`
	Description string    `json:"description" doc:"Product description"`
	Price       float64   `json:"price" example:"5.50" doc:"Product price"`
	ImagePath   *string   `json:"image_path,omitempty" doc:"Image object path"`
	IsAvailable bool      `json:"is_available" example:"true" doc:"Whether the product is available"`
	CreatedAt   time.Time `json:"created_at" doc:"Creation timestamp"`
}

// ListProductsInput describes query parameters for listing products.
type ListProductsInput struct {
	Category string `query:"category" doc:"Filter by category slug"`
	Page     int    `query:"page" default:"1" doc:"Page number"`
	PerPage  int    `query:"per_page" default:"20" doc:"Items per page"`
	All      bool   `query:"all" default:"false" doc:"Return all products (admin fallback)"`
}

// PaginatedProductsOutput is the response for listing products.
type PaginatedProductsOutput struct {
	Body PaginatedResponse[Product]
}

// ProductIDInput is the path parameter for single-product endpoints.
type ProductIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Product ID"`
}

// ProductOutput is the response for a single product.
type ProductOutput struct {
	Body Product
}

// CreateProductInput is the payload for creating a product.
type CreateProductInput struct {
	Body struct {
		Name                 string  `json:"name" example:"Stracciatella" doc:"Product name"`
		Description          string  `json:"description" doc:"Product description"`
		Price                float64 `json:"price" example:"5.50" doc:"Product price"`
		CategoryID           string  `json:"category_id" format:"uuid" doc:"Category ID"`
		ImagePath            *string `json:"image_path,omitempty" doc:"Image object path"`
		StockStatus          *string `json:"stock_status,omitempty" example:"in_stock" doc:"Stock status"`
		IsAvailable          *bool   `json:"is_available,omitempty" example:"true" doc:"Whether the product is available"`
		ExcludeGlobalFlavors *bool   `json:"exclude_global_flavors,omitempty" example:"false" doc:"Whether global flavors are excluded"`
		ExcludeGlobalAddons  *bool   `json:"exclude_global_addons,omitempty" example:"false" doc:"Whether global addons are excluded"`
		BallBased            *bool   `json:"ball_based,omitempty" example:"false" doc:"Whether pricing is ball-based"`
		BallQuantity         *int    `json:"ball_quantity,omitempty" example:"0" doc:"Ball quantity"`
		AllowsMixedFlavors   *bool   `json:"allows_mixed_flavors,omitempty" example:"false" doc:"Whether mixed flavors are allowed"`
	}
}

// UpdateProductInput is the payload for updating a product.
type UpdateProductInput struct {
	ProductIDInput
	Body struct {
		Name                 *string  `json:"name,omitempty" example:"Stracciatella" doc:"Product name"`
		Description          *string  `json:"description,omitempty" doc:"Product description"`
		Price                *float64 `json:"price,omitempty" example:"5.50" doc:"Product price"`
		CategoryID           *string  `json:"category_id,omitempty" format:"uuid" doc:"Category ID"`
		ImagePath            *string  `json:"image_path,omitempty" doc:"Image object path"`
		IsAvailable          *bool    `json:"is_available,omitempty" example:"true" doc:"Whether the product is available"`
		ExcludeGlobalFlavors *bool    `json:"exclude_global_flavors,omitempty" example:"false" doc:"Whether global flavors are excluded"`
		ExcludeGlobalAddons  *bool    `json:"exclude_global_addons,omitempty" example:"false" doc:"Whether global addons are excluded"`
		BallBased            *bool    `json:"ball_based,omitempty" example:"false" doc:"Whether pricing is ball-based"`
		BallQuantity         *int     `json:"ball_quantity,omitempty" example:"0" doc:"Ball quantity"`
		AllowsMixedFlavors   *bool    `json:"allows_mixed_flavors,omitempty" example:"false" doc:"Whether mixed flavors are allowed"`
	}
}

// UpdateProductAvailabilityInput is the payload for toggling product availability.
type UpdateProductAvailabilityInput struct {
	ProductIDInput
}

// ProductFlavorLinkInput describes the product-flavor link path parameters.
type ProductFlavorLinkInput struct {
	ID       string `path:"id" format:"uuid" doc:"Product ID"`
	FlavorID string `path:"flavorId" format:"uuid" doc:"Flavor ID"`
}

// ProductAddonLinkInput describes the product-addon link path parameters.
type ProductAddonLinkInput struct {
	ID      string `path:"id" format:"uuid" doc:"Product ID"`
	AddonID string `path:"addonId" format:"uuid" doc:"Addon ID"`
}

// ListCategoriesInput describes query parameters for listing categories.
type ListCategoriesInput struct {
	Page    int  `query:"page" default:"1" doc:"Page number"`
	PerPage int  `query:"per_page" default:"20" doc:"Items per page"`
	All     bool `query:"all" default:"false" doc:"Return all categories (admin fallback)"`
}

// PaginatedCategoriesOutput is the response for listing categories.
type PaginatedCategoriesOutput struct {
	Body PaginatedResponse[Category]
}

// CategorySlugInput is the path parameter for the category-by-slug endpoint.
type CategorySlugInput struct {
	Slug string `path:"slug" example:"gelatos" doc:"Category slug"`
}

// CategoryDetailOutput is the response for a category with products.
type CategoryDetailOutput struct {
	Body CategoryDetail
}

// CategoryIDInput is the path parameter for single-category endpoints.
type CategoryIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Category ID"`
}

// CreateCategoryInput is the payload for creating a category.
type CreateCategoryInput struct {
	Body struct {
		Name         string  `json:"name" example:"Gelatos" doc:"Category name"`
		Slug         string  `json:"slug" example:"gelatos" doc:"URL slug"`
		Description  string  `json:"description" doc:"Category description"`
		ImagePath    *string `json:"image_path,omitempty" doc:"Image object path"`
		DisplayOrder *int    `json:"display_order,omitempty" example:"1" doc:"Display order"`
	}
}

// UpdateCategoryInput is the payload for updating a category.
type UpdateCategoryInput struct {
	CategoryIDInput
	Body struct {
		Name         *string `json:"name,omitempty" example:"Gelatos" doc:"Category name"`
		Slug         *string `json:"slug,omitempty" example:"gelatos" doc:"URL slug"`
		Description  *string `json:"description,omitempty" doc:"Category description"`
		ImagePath    *string `json:"image_path,omitempty" doc:"Image object path"`
		DisplayOrder *int    `json:"display_order,omitempty" example:"1" doc:"Display order"`
		IsActive     *bool   `json:"is_active,omitempty" example:"true" doc:"Whether the category is active"`
	}
}

// CategoryOutput is the response for a single category.
type CategoryOutput struct {
	Body Category
}

// FlavorIDInput is the path parameter for single-flavor endpoints.
type FlavorIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Flavor ID"`
}

// CreateFlavorInput is the payload for creating a flavor.
type CreateFlavorInput struct {
	Body struct {
		Name         string `json:"name" example:"Chocolate" doc:"Flavor name"`
		DisplayOrder *int   `json:"display_order,omitempty" example:"1" doc:"Display order"`
		IsSeasonal   bool   `json:"is_seasonal" example:"false" doc:"Whether the flavor is seasonal"`
	}
}

// UpdateFlavorInput is the payload for updating a flavor.
type UpdateFlavorInput struct {
	FlavorIDInput
	Body struct {
		Name         *string `json:"name,omitempty" example:"Chocolate" doc:"Flavor name"`
		DisplayOrder *int    `json:"display_order,omitempty" example:"1" doc:"Display order"`
		IsSeasonal   *bool   `json:"is_seasonal,omitempty" example:"false" doc:"Whether the flavor is seasonal"`
		IsActive     *bool   `json:"is_active,omitempty" example:"true" doc:"Whether the flavor is active"`
	}
}

// FlavorOutput is the response for a single flavor.
type FlavorOutput struct {
	Body Flavor
}

// PaginatedFlavorsOutput is the response for listing flavors.
type PaginatedFlavorsOutput struct {
	Body PaginatedResponse[Flavor]
}

// AddonIDInput is the path parameter for single-addon endpoints.
type AddonIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Addon ID"`
}

// CreateAddonInput is the payload for creating an addon.
type CreateAddonInput struct {
	Body struct {
		Name         string  `json:"name" example:"Extra scoop" doc:"Addon name"`
		Price        float64 `json:"price" example:"1.00" doc:"Addon price"`
		GroupName    string  `json:"group_name" example:"Extras" doc:"Addon group name"`
		DisplayOrder *int    `json:"display_order,omitempty" example:"1" doc:"Display order"`
	}
}

// UpdateAddonInput is the payload for updating an addon.
type UpdateAddonInput struct {
	AddonIDInput
	Body struct {
		Name         *string  `json:"name,omitempty" example:"Extra scoop" doc:"Addon name"`
		Price        *float64 `json:"price,omitempty" example:"1.00" doc:"Addon price"`
		GroupName    *string  `json:"group_name,omitempty" example:"Extras" doc:"Addon group name"`
		DisplayOrder *int     `json:"display_order,omitempty" example:"1" doc:"Display order"`
		IsActive     *bool    `json:"is_active,omitempty" example:"true" doc:"Whether the addon is active"`
	}
}

// AddonOutput is the response for a single addon.
type AddonOutput struct {
	Body Addon
}

// PaginatedAddonsOutput is the response for listing addons.
type PaginatedAddonsOutput struct {
	Body PaginatedResponse[Addon]
}

// OrderItem is a single item in a public order.
type OrderItem struct {
	ProductID      string                 `json:"product_id" format:"uuid" doc:"Product ID"`
	Quantity       int                    `json:"quantity" example:"2" doc:"Quantity"`
	Customizations map[string]interface{} `json:"customizations,omitempty" doc:"Item customizations"`
}

// Order is the full order shape returned by the admin endpoints.
type Order struct {
	ID               string            `json:"id" format:"uuid" doc:"Order ID"`
	OrderNumber      string            `json:"order_number" example:"ORD-0001" doc:"Order number"`
	CustomerID       *string           `json:"customer_id,omitempty" format:"uuid" doc:"Customer ID"`
	CustomerName     string            `json:"customer_name" example:"John Doe" doc:"Customer name"`
	CustomerPhone    string            `json:"customer_phone" example:"+50688881234" doc:"Customer phone"`
	CustomerEmail    *string           `json:"customer_email,omitempty" example:"john@example.com" doc:"Customer email"`
	TotalAmount      float64           `json:"total_amount" example:"11.00" doc:"Order total"`
	Status           string            `json:"status" example:"pendiente_revision" doc:"Order status"`
	PaymentMethod    string            `json:"payment_method" example:"efectivo" doc:"Payment method"`
	PaymentStatus    string            `json:"payment_status" example:"pending" doc:"Payment status"`
	OrderType        string            `json:"order_type" example:"para_llevar" doc:"Order type"`
	TableNumber      *int              `json:"table_number,omitempty" doc:"Table number"`
	Notes            *string           `json:"notes,omitempty" doc:"Order notes"`
	RejectionReason  *string           `json:"rejection_reason,omitempty" doc:"Rejection reason"`
	ReviewedByUserID *string           `json:"reviewed_by_user_id,omitempty" doc:"Reviewer employee ID"`
	ReviewedAt       *time.Time        `json:"reviewed_at,omitempty" doc:"Review timestamp"`
	CreatedByUserID  *string           `json:"created_by_user_id,omitempty" doc:"Creator employee ID"`
	IsArchived       bool              `json:"is_archived" example:"false" doc:"Whether the order is archived"`
	CreatedAt        time.Time         `json:"created_at" doc:"Creation timestamp"`
	UpdatedAt        time.Time         `json:"updated_at" doc:"Last update timestamp"`
	StatusTimestamps map[string]string `json:"status_timestamps,omitempty" doc:"Status change timestamps"`
	Items            []OrderItem       `json:"items,omitempty" doc:"Order items"`
}

// CreateOrderInput is the public order creation payload.
type CreateOrderInput struct {
	Body struct {
		CustomerName  string      `json:"customer_name" example:"John Doe" doc:"Customer name"`
		CustomerPhone string      `json:"customer_phone" example:"+50688881234" doc:"Customer phone"`
		CustomerEmail string      `json:"customer_email,omitempty" example:"john@example.com" doc:"Customer email"`
		PaymentMethod string      `json:"payment_method" enum:"efectivo,tarjeta,transferencia,otro" doc:"Payment method"`
		OrderType     string      `json:"order_type" enum:"en_local,para_llevar" doc:"Order type"`
		TableNumber   int         `json:"table_number,omitempty" doc:"Table number"`
		Notes         string      `json:"notes,omitempty" doc:"Order notes"`
		Items         []OrderItem `json:"items" doc:"Order items"`
	}
}

// OrderOutput is the response for creating an order.
type OrderOutput struct {
	Body struct {
		OrderNumber       string `json:"order_number" example:"ORD-0001" doc:"Order number"`
		Status            string `json:"status" example:"pendiente_revision" doc:"Order status"`
		Total             int    `json:"total" example:"11" doc:"Order total"`
		TrackingToken     string `json:"tracking_token" doc:"Tracking token"`
		TrackingExpiresAt string `json:"tracking_expires_at" doc:"Tracking token expiration"`
		Message           string `json:"message" example:"Order received" doc:"Confirmation message"`
	}
}

// TrackOrderInput describes query parameters for tracking an order.
type TrackOrderInput struct {
	OrderNumber   string `query:"order_number" doc:"Order number"`
	TrackingToken string `query:"tracking_token" doc:"Tracking token"`
}

// OrderTrackingOutput is the response for tracking an order.
type OrderTrackingOutput struct {
	Body struct {
		OrderNumber string  `json:"order_number" doc:"Order number"`
		Status      string  `json:"status" doc:"Order status"`
		TotalAmount float64 `json:"total_amount" doc:"Order total"`
	}
}

// ListOrdersInput describes query parameters for listing orders.
type ListOrdersInput struct {
	Status   string `query:"status" doc:"Filter by status"`
	Lite     bool   `query:"lite" default:"false" doc:"Return lightweight order rows"`
	Archived bool   `query:"archived" default:"false" doc:"Include archived orders"`
	Page     int    `query:"page" default:"1" doc:"Page number"`
	PerPage  int    `query:"per_page" default:"20" doc:"Items per page"`
}

// PaginatedOrdersOutput is the response for listing orders.
type PaginatedOrdersOutput struct {
	Body PaginatedResponse[Order]
}

// OrderIDInput is the path parameter for single-order endpoints.
type OrderIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Order ID"`
}

// OrderOutputSingle is the response for a single order.
type OrderOutputSingle struct {
	Body Order
}

// UpdateOrderInput is the payload for updating an order.
type UpdateOrderInput struct {
	OrderIDInput
	Body struct {
		CustomerName  string      `json:"customer_name" example:"John Doe" doc:"Customer name"`
		CustomerPhone string      `json:"customer_phone" example:"+50688881234" doc:"Customer phone"`
		CustomerEmail string      `json:"customer_email,omitempty" example:"john@example.com" doc:"Customer email"`
		PaymentMethod string      `json:"payment_method" enum:"efectivo,tarjeta,transferencia,otro" doc:"Payment method"`
		OrderType     string      `json:"order_type" enum:"en_local,para_llevar" doc:"Order type"`
		TableNumber   int         `json:"table_number,omitempty" doc:"Table number"`
		Notes         string      `json:"notes" doc:"Order notes"`
		Items         []OrderItem `json:"items" doc:"Order items"`
	}
}

// RejectOrderInput is the payload for rejecting an order.
type RejectOrderInput struct {
	OrderIDInput
	Body struct {
		Reason string `json:"reason" example:"Out of stock" doc:"Rejection reason"`
	}
}

// UpdateOrderStatusInput is the payload for updating an order status.
type UpdateOrderStatusInput struct {
	OrderIDInput
	Body struct {
		Status string `json:"status" example:"en_preparacion" doc:"New order status"`
	}
}

// UpdateOrderNotesInput is the payload for updating order notes.
type UpdateOrderNotesInput struct {
	OrderIDInput
	Body struct {
		Notes *string `json:"notes" example:"Call on arrival" doc:"Order notes"`
	}
}

// ArchiveOrderInput is the payload for archiving/unarchiving an order.
type ArchiveOrderInput struct {
	OrderIDInput
	Body struct {
		Archived bool `json:"archived" example:"true" doc:"Archive flag"`
	}
}

// ContainerType is the shape of an ice-cream container type.
type ContainerType struct {
	ID                string    `json:"id" format:"uuid" doc:"Container type ID"`
	Name              string    `json:"name" example:"Small cup" doc:"Container type name"`
	BallsPerContainer int       `json:"balls_per_container" example:"1" doc:"Number of balls per container"`
	IsCustom          bool      `json:"is_custom" example:"false" doc:"Whether the container size is custom"`
	IsActive          bool      `json:"is_active" example:"true" doc:"Whether the container type is active"`
	CreatedAt         time.Time `json:"created_at" doc:"Creation timestamp"`
}

// PaginatedContainerTypesOutput is the response for listing container types.
type PaginatedContainerTypesOutput struct {
	Body PaginatedResponse[ContainerType]
}

// ContainerTypeIDInput is the path parameter for single container-type endpoints.
type ContainerTypeIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Container type ID"`
}

// CreateContainerTypeInput is the payload for creating a container type.
type CreateContainerTypeInput struct {
	Body struct {
		Name              string `json:"name" example:"Small cup" doc:"Container type name"`
		BallsPerContainer int    `json:"balls_per_container" example:"1" doc:"Number of balls per container"`
		IsCustom          bool   `json:"is_custom" example:"false" doc:"Whether the container size is custom"`
	}
}

// UpdateContainerTypeInput is the payload for updating a container type.
type UpdateContainerTypeInput struct {
	ContainerTypeIDInput
	Body struct {
		Name              *string `json:"name,omitempty" example:"Small cup" doc:"Container type name"`
		BallsPerContainer *int    `json:"balls_per_container,omitempty" example:"1" doc:"Number of balls per container"`
		IsCustom          *bool   `json:"is_custom,omitempty" example:"false" doc:"Whether the container size is custom"`
		IsActive          *bool   `json:"is_active,omitempty" example:"true" doc:"Whether the container type is active"`
	}
}

// ContainerTypeOutput is the response for a single container type.
type ContainerTypeOutput struct {
	Body ContainerType
}

// InventoryItem is an inventory stock-keeping item.
type InventoryItem struct {
	ID                string    `json:"id" format:"uuid" doc:"Inventory item ID"`
	Name              string    `json:"name" example:"Chocolate" doc:"Item name"`
	Type              string    `json:"type" example:"flavor" doc:"Item type"`
	CurrentStock      int       `json:"current_stock" example:"10" doc:"Current stock quantity"`
	LowStockThreshold int       `json:"low_stock_threshold" example:"5" doc:"Low stock alert threshold"`
	ProductID         *string   `json:"product_id,omitempty" format:"uuid" doc:"Linked product ID"`
	FlavorID          *string   `json:"flavor_id,omitempty" format:"uuid" doc:"Linked flavor ID"`
	CreatedAt         time.Time `json:"created_at" doc:"Creation timestamp"`
}

// StockMovement is a single inventory movement record.
type StockMovement struct {
	ID                 string    `json:"id" format:"uuid" doc:"Movement ID"`
	Type               string    `json:"type" example:"entry" doc:"Movement type"`
	Quantity           int       `json:"quantity" example:"5" doc:"Quantity changed"`
	QuantityContainers *int      `json:"quantity_containers,omitempty" example:"2" doc:"Number of containers"`
	InventoryItemID    string    `json:"inventory_item_id" format:"uuid" doc:"Inventory item ID"`
	ContainerTypeID    *string   `json:"container_type_id,omitempty" format:"uuid" doc:"Container type ID"`
	FlavorID           *string   `json:"flavor_id,omitempty" format:"uuid" doc:"Flavor ID"`
	OrderID            *string   `json:"order_id,omitempty" format:"uuid" doc:"Order ID"`
	Reason             *string   `json:"reason,omitempty" example:"Initial stock" doc:"Movement reason"`
	CreatedByUserID    *string   `json:"created_by_user_id,omitempty" doc:"Creator employee ID"`
	CreatedAt          time.Time `json:"created_at" doc:"Creation timestamp"`
}

// PaginatedInventoryOutput is the response for listing inventory items.
type PaginatedInventoryOutput struct {
	Body PaginatedResponse[InventoryItem]
}

// InventoryDashboardOutput is the aggregated inventory dashboard response.
type InventoryDashboardOutput struct {
	Body struct {
		FlavorInventory []InventoryItem `json:"flavor_inventory" doc:"Flavor inventory items"`
		UnitInventory   []InventoryItem `json:"unit_inventory" doc:"Unit inventory items"`
		Stats           InventoryStats  `json:"stats" doc:"Inventory statistics"`
	}
}

// InventoryStats is the aggregate inventory statistics shape.
type InventoryStats struct {
	TotalItems      int `json:"total_items" example:"25" doc:"Total inventory items"`
	LowStockItems   int `json:"low_stock_items" example:"3" doc:"Items below low-stock threshold"`
	OutOfStockItems int `json:"out_of_stock_items" example:"1" doc:"Items with zero stock"`
}

// InventoryItemIDInput is the path parameter for single-inventory endpoints.
type InventoryItemIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Inventory item ID"`
}

// InventoryItemOutput is the response for a single inventory item.
type InventoryItemOutput struct {
	Body InventoryItem
}

// InventoryMovementsOutput is the response for inventory movement history.
type InventoryMovementsOutput struct {
	Body struct {
		Data       []StockMovement `json:"data" doc:"Movement records"`
		Total      int             `json:"total" example:"100" doc:"Total records"`
		Limit      int             `json:"limit" example:"50" doc:"Page limit"`
		Offset     int             `json:"offset" example:"0" doc:"Page offset"`
		TotalPages int             `json:"total_pages" example:"2" doc:"Total pages"`
	}
}

// CreateUnitInventoryItemInput is the payload for creating a unit-based inventory item.
type CreateUnitInventoryItemInput struct {
	Body struct {
		Name              string `json:"name" example:"Cones" doc:"Item name"`
		ProductID         string `json:"product_id" format:"uuid" doc:"Linked product ID"`
		LowStockThreshold int    `json:"low_stock_threshold" example:"10" doc:"Low stock alert threshold"`
	}
}

// RecordEntryInput is the payload for recording a flavor stock entry.
type RecordEntryInput struct {
	InventoryItemIDInput
	Body struct {
		ContainerTypeID    string `json:"container_type_id" format:"uuid" doc:"Container type ID"`
		QuantityContainers int    `json:"quantity_containers" example:"2" doc:"Number of containers"`
		BallsPerContainer  int    `json:"balls_per_container,omitempty" example:"1" doc:"Balls per container (for custom containers)"`
	}
}

// RecordUnitEntryInput is the payload for recording a unit-based stock entry.
type RecordUnitEntryInput struct {
	InventoryItemIDInput
	Body struct {
		Quantity int `json:"quantity" example:"20" doc:"Quantity added"`
	}
}

// RecordAdjustmentInput is the payload for recording a manual stock adjustment.
type RecordAdjustmentInput struct {
	InventoryItemIDInput
	Body struct {
		Quantity int    `json:"quantity" example:"-2" doc:"Quantity adjustment"`
		Reason   string `json:"reason" example:"Spoilage" doc:"Adjustment reason"`
	}
}

// InventoryProductLinkInput describes the inventory-product link path parameters.
type InventoryProductLinkInput struct {
	ID        string `path:"id" format:"uuid" doc:"Inventory item ID"`
	ProductID string `path:"productId" format:"uuid" doc:"Product ID"`
}

// FlavorStockOutput is the response for flavor stock status.
type FlavorStockOutput struct {
	Body struct {
		FlavorID     string `json:"flavor_id" format:"uuid" doc:"Flavor ID"`
		CurrentStock int    `json:"current_stock" example:"10" doc:"Current stock"`
		LowStock     bool   `json:"low_stock" example:"false" doc:"Whether stock is low"`
	}
}

// InventoryFlavorIDInput is the path parameter for flavor inventory endpoints.
type InventoryFlavorIDInput struct {
	ID string `path:"id" format:"uuid" doc:"Flavor ID"`
}

// User is the customer directory shape.
type User struct {
	ID        string    `json:"id" format:"uuid" doc:"User ID"`
	Name      string    `json:"name" example:"John Doe" doc:"User name"`
	UserType  string    `json:"user_type" example:"regular" doc:"User type"`
	Phone     string    `json:"phone" example:"+50688881234" doc:"User phone"`
	Email     *string   `json:"email,omitempty" example:"john@example.com" doc:"User email"`
	Status    string    `json:"status" example:"active" doc:"User status"`
	CreatedAt time.Time `json:"created_at" doc:"Creation timestamp"`
	UpdatedAt time.Time `json:"updated_at" doc:"Last update timestamp"`
}

// UserOrder is a lightweight order shape embedded in user history.
type UserOrder struct {
	ID            string    `json:"id" format:"uuid" doc:"Order ID"`
	OrderNumber   string    `json:"order_number" example:"ORD-0001" doc:"Order number"`
	CustomerName  string    `json:"customer_name" example:"John Doe" doc:"Customer name"`
	CustomerPhone string    `json:"customer_phone" example:"+50688881234" doc:"Customer phone"`
	TotalAmount   float64   `json:"total_amount" example:"11.00" doc:"Order total"`
	Status        string    `json:"status" example:"entregada" doc:"Order status"`
	CreatedAt     time.Time `json:"created_at" doc:"Creation timestamp"`
}

// ListUsersInput describes query parameters for listing users.
type ListUsersInput struct {
	Status  string `query:"status" doc:"Filter by status"`
	Search  string `query:"search" doc:"Search by name or phone"`
	Page    int    `query:"page" default:"1" doc:"Page number"`
	PerPage int    `query:"per_page" default:"20" doc:"Items per page"`
}

// PaginatedUsersOutput is the response for listing users.
type PaginatedUsersOutput struct {
	Body PaginatedResponse[User]
}

// UserIDInput is the path parameter for single-user endpoints.
type UserIDInput struct {
	ID string `path:"id" format:"uuid" doc:"User ID"`
}

// UserOutput is the response for a single user.
type UserOutput struct {
	Body User
}

// CreateUserInput is the payload for creating a user.
type CreateUserInput struct {
	Body struct {
		Name     string `json:"name" example:"John Doe" doc:"User name"`
		UserType string `json:"user_type" example:"regular" doc:"User type"`
		Phone    string `json:"phone" example:"+50688881234" doc:"User phone"`
		Email    string `json:"email,omitempty" example:"john@example.com" doc:"User email"`
		Status   string `json:"status" example:"active" doc:"User status"`
	}
}

// UpdateUserInput is the payload for updating a user.
type UpdateUserInput struct {
	UserIDInput
	Body struct {
		Name     *string `json:"name,omitempty" example:"John Doe" doc:"User name"`
		UserType *string `json:"user_type,omitempty" example:"regular" doc:"User type"`
		Phone    *string `json:"phone,omitempty" example:"+50688881234" doc:"User phone"`
		Email    *string `json:"email,omitempty" example:"john@example.com" doc:"User email"`
		Status   *string `json:"status,omitempty" example:"active" doc:"User status"`
	}
}

// UserOrdersOutput is the response for a user's order history.
type UserOrdersOutput struct {
	Body struct {
		Orders []UserOrder `json:"orders" doc:"User orders"`
	}
}

// Employee is the employee directory shape.
type Employee struct {
	ID     string `json:"id" doc:"Employee ID"`
	Email  string `json:"email" example:"admin@example.com" doc:"Employee email"`
	Name   string `json:"name" example:"Admin" doc:"Employee name"`
	Phone  string `json:"phone" example:"+50688881234" doc:"Employee phone"`
	Role   string `json:"role" example:"admin" doc:"Employee role"`
	State  string `json:"state" example:"active" doc:"Employee state"`
	Active bool   `json:"active" example:"true" doc:"Whether the employee is active"`
}

// PaginatedEmployeesOutput is the response for listing employees.
type PaginatedEmployeesOutput struct {
	Body PaginatedResponse[Employee]
}

// EmployeeIDInput is the path parameter for single-employee endpoints.
type EmployeeIDInput struct {
	ID string `path:"id" doc:"Employee ID"`
}

// CreateEmployeeInput is the payload for creating an employee.
type CreateEmployeeInput struct {
	Body struct {
		Email    string `json:"email" example:"admin@example.com" doc:"Employee email"`
		Password string `json:"password" example:"secret" doc:"Employee password"`
		Name     string `json:"name" example:"Admin" doc:"Employee name"`
		Phone    string `json:"phone" example:"+50688881234" doc:"Employee phone"`
		Role     string `json:"role" example:"employee" doc:"Employee role"`
		State    string `json:"state" example:"active" doc:"Employee state"`
	}
}

// UpdateEmployeeInput is the payload for updating an employee.
type UpdateEmployeeInput struct {
	EmployeeIDInput
	Body struct {
		Email    *string `json:"email,omitempty" example:"admin@example.com" doc:"Employee email"`
		Password *string `json:"password,omitempty" example:"secret" doc:"Employee password"`
		Name     *string `json:"name,omitempty" example:"Admin" doc:"Employee name"`
		Phone    *string `json:"phone,omitempty" example:"+50688881234" doc:"Employee phone"`
		Role     *string `json:"role,omitempty" example:"employee" doc:"Employee role"`
		State    *string `json:"state,omitempty" example:"active" doc:"Employee state"`
	}
}

// EmployeeOutput is the response for a single employee.
type EmployeeOutput struct {
	Body Employee
}

// OfferItem is a single special offer in store settings.
type OfferItem struct {
	ProductID     string   `json:"product_id" format:"uuid" doc:"Product ID"`
	Label         string   `json:"label" example:"2x1" doc:"Offer label"`
	Note          string   `json:"note,omitempty" example:"All week" doc:"Offer note"`
	DiscountPrice float64  `json:"discount_price,omitempty" example:"4.00" doc:"Discount price"`
	FlavorID      string   `json:"flavor_id,omitempty" format:"uuid" doc:"Flavor ID"`
	FlavorIDs     []string `json:"flavor_ids,omitempty" doc:"Flavor IDs"`
	ExpiresAt     string   `json:"expires_at" example:"2026-12-31T23:59:59Z" doc:"Offer expiration (RFC 3339)"`
}

// StoreSettings is the persisted store configuration shape.
type StoreSettings struct {
	OrdersEnabled bool        `json:"orders_enabled" example:"true" doc:"Whether public orders are enabled"`
	Offers        []OfferItem `json:"offers" doc:"Active offers"`
}

// StoreSettingsOutput is the response for store settings.
type StoreSettingsOutput struct {
	Body StoreSettings
}

// ActiveOffer is a public active offer.
type ActiveOffer struct {
	ProductID     string   `json:"product_id" doc:"Product ID"`
	Label         string   `json:"label" doc:"Offer label"`
	Note          string   `json:"note,omitempty" doc:"Offer note"`
	DiscountPrice float64  `json:"discount_price,omitempty" doc:"Discount price"`
	FlavorID      string   `json:"flavor_id,omitempty" doc:"Flavor ID"`
	FlavorIDs     []string `json:"flavor_ids,omitempty" doc:"Flavor IDs"`
	ExpiresAt     string   `json:"expires_at" doc:"Offer expiration (RFC 3339)"`
}

// StoreStatusOutput is the response for public store status.
type StoreStatusOutput struct {
	Body struct {
		OrdersEnabled bool          `json:"orders_enabled" doc:"Whether public orders are enabled"`
		Offers        []ActiveOffer `json:"offers" doc:"Active offers"`
	}
}

// ArchiveConfig is the persisted order archive configuration.
type ArchiveConfig struct {
	Enabled         bool `json:"enabled" example:"true" doc:"Whether auto-archiving is enabled"`
	AgeDays         int  `json:"age_days" example:"30" doc:"Archive orders older than this many days"`
	IntervalMinutes int  `json:"interval_minutes" example:"60" doc:"Archive check interval in minutes"`
}

// ArchiveSettingsOutput is the response for archive settings.
type ArchiveSettingsOutput struct {
	Body ArchiveConfig
}

// PublicOrderRateLimitConfig is the persisted public order rate-limit configuration.
type PublicOrderRateLimitConfig struct {
	PhoneBurst         int `json:"phone_burst" example:"5" doc:"Max orders per phone burst"`
	PhoneWindowMinutes int `json:"phone_window_minutes" example:"60" doc:"Phone rate-limit window in minutes"`
	PhoneBlockMinutes  int `json:"phone_block_minutes" example:"60" doc:"Phone block duration in minutes"`
	IPBurst            int `json:"ip_burst" example:"10" doc:"Max orders per IP burst"`
	IPWindowMinutes    int `json:"ip_window_minutes" example:"60" doc:"IP rate-limit window in minutes"`
	IPBlockMinutes     int `json:"ip_block_minutes" example:"60" doc:"IP block duration in minutes"`
	StaffBurst         int `json:"staff_burst" example:"100" doc:"Max staff orders per burst"`
	StaffWindowMinutes int `json:"staff_window_minutes" example:"60" doc:"Staff rate-limit window in minutes"`
}

// RateLimitSettingsOutput is the response for rate-limit settings.
type RateLimitSettingsOutput struct {
	Body PublicOrderRateLimitConfig
}

// TokenTTLConfig is the persisted panel/session token TTL configuration.
type TokenTTLConfig struct {
	AuthCookieTTLHours      int `json:"auth_cookie_ttl_hours" example:"168" doc:"Auth cookie TTL in hours"`
	AuthTokenTTLHours       int `json:"auth_token_ttl_hours" example:"24" doc:"Auth token TTL in hours"`
	TrackingTokenTTLHours   int `json:"tracking_token_ttl_hours" example:"72" doc:"Tracking token TTL in hours"`
	InactivityLogoutSeconds int `json:"inactivity_logout_seconds" example:"1800" doc:"Inactivity logout timeout in seconds"`
}

// PanelConfigOutput is the response for panel configuration.
type PanelConfigOutput struct {
	Body TokenTTLConfig
}

// AnalyticsOverview is the aggregate result for the analytics overview endpoint.
type AnalyticsOverview struct {
	Period          string         `json:"period" example:"month" doc:"Report period"`
	TotalOrders     int            `json:"total_orders" example:"150" doc:"Total orders"`
	TotalRevenue    float64        `json:"total_revenue" example:"1250.50" doc:"Total revenue"`
	AvgOrderValue   float64        `json:"avg_order_value" example:"8.34" doc:"Average order value"`
	StatusBreakdown map[string]int `json:"status_breakdown" doc:"Orders grouped by status"`
}

// AnalyticsOverviewOutput is the response for the analytics overview endpoint.
type AnalyticsOverviewOutput struct {
	Body AnalyticsOverview
}

// AnalyticsOverviewInput describes query parameters for the overview report.
type AnalyticsOverviewInput struct {
	Period string `query:"period" default:"month" doc:"Report period"`
}

// AnalyticsTimelinePoint is one bucket for orders-over-time.
type AnalyticsTimelinePoint struct {
	Date    string  `json:"date" example:"2026-07-01" doc:"Bucket date"`
	Count   int     `json:"count" example:"10" doc:"Order count"`
	Revenue float64 `json:"revenue" example:"100.00" doc:"Revenue"`
}

// AnalyticsOrdersOverTimeInput describes query parameters for the orders-over-time report.
type AnalyticsOrdersOverTimeInput struct {
	Start   string `query:"start" example:"2026-06-01" doc:"Start date (YYYY-MM-DD)"`
	End     string `query:"end" example:"2026-07-01" doc:"End date (YYYY-MM-DD)"`
	GroupBy string `query:"groupBy" default:"day" enum:"day,week,month" doc:"Bucket granularity"`
}

// AnalyticsOrdersOverTimeOutput is the response for the orders-over-time report.
type AnalyticsOrdersOverTimeOutput struct {
	Body []AnalyticsTimelinePoint
}

// AnalyticsTopProduct is one row of the top-products report.
type AnalyticsTopProduct struct {
	Name       string  `json:"name" example:"Stracciatella" doc:"Product name"`
	TotalSold  int     `json:"total_sold" example:"50" doc:"Total units sold"`
	TotalValue float64 `json:"total_value" example:"275.00" doc:"Total sold value"`
}

// AnalyticsTopProductsInput describes query parameters for the top-products report.
type AnalyticsTopProductsInput struct {
	Start string `query:"start" example:"2026-06-01" doc:"Start date (YYYY-MM-DD)"`
	End   string `query:"end" example:"2026-07-01" doc:"End date (YYYY-MM-DD)"`
	Limit int    `query:"limit" default:"10" doc:"Number of products to return"`
}

// AnalyticsTopProductsOutput is the response for the top-products report.
type AnalyticsTopProductsOutput struct {
	Body []AnalyticsTopProduct
}

// AnalyticsDashboardInput describes query parameters for the combined dashboard.
type AnalyticsDashboardInput struct {
	Start   string `query:"start" example:"2026-06-01" doc:"Start date (YYYY-MM-DD)"`
	End     string `query:"end" example:"2026-07-01" doc:"End date (YYYY-MM-DD)"`
	GroupBy string `query:"groupBy" default:"day" enum:"day,week,month" doc:"Bucket granularity"`
	Limit   int    `query:"limit" default:"10" doc:"Number of products to return"`
}

// AnalyticsDashboardOutput is the combined analytics dashboard response.
type AnalyticsDashboardOutput struct {
	Body struct {
		Overview       AnalyticsOverview        `json:"overview" doc:"Aggregate metrics for the date range"`
		OrdersOverTime []AnalyticsTimelinePoint `json:"orders_over_time" doc:"Order counts and revenue over the date range"`
		TopProducts    []AnalyticsTopProduct    `json:"top_products" doc:"Best-selling products for the date range"`
	}
}

// ExportOrdersInput describes query parameters for exporting orders.
type ExportOrdersInput struct {
	Start    string `query:"start" example:"2026-06-01" doc:"Start date (YYYY-MM-DD)"`
	End      string `query:"end" example:"2026-07-01" doc:"End date (YYYY-MM-DD)"`
	Format   string `query:"format" default:"csv" enum:"csv,json" doc:"Export format"`
	Statuses string `query:"statuses" doc:"Comma-separated order statuses"`
}

// ExportOrdersOutput is the response for a JSON order export.
type ExportOrdersOutput struct {
	Body struct {
		Start  string           `json:"start" doc:"Start date"`
		End    string           `json:"end" doc:"End date"`
		Count  int              `json:"count" example:"100" doc:"Exported order count"`
		Orders []ExportOrderRow `json:"orders" doc:"Exported orders"`
	}
}

// ExportOrderRow is a flattened order export row.
type ExportOrderRow struct {
	ID            string    `json:"id" format:"uuid" doc:"Order ID"`
	OrderNumber   string    `json:"order_number" example:"ORD-0001" doc:"Order number"`
	CustomerName  string    `json:"customer_name" example:"John Doe" doc:"Customer name"`
	CustomerPhone string    `json:"customer_phone" example:"+50688881234" doc:"Customer phone"`
	Status        string    `json:"status" example:"entregada" doc:"Order status"`
	PaymentMethod string    `json:"payment_method" example:"efectivo" doc:"Payment method"`
	PaymentStatus string    `json:"payment_status" example:"paid" doc:"Payment status"`
	OrderType     string    `json:"order_type" example:"para_llevar" doc:"Order type"`
	TableNumber   *int      `json:"table_number,omitempty" doc:"Table number"`
	TotalAmount   float64   `json:"total_amount" example:"11.00" doc:"Order total"`
	ItemsCount    int       `json:"items_count" example:"2" doc:"Number of items"`
	ItemsSummary  string    `json:"items_summary" example:"2x Stracciatella" doc:"Items summary"`
	Notes         *string   `json:"notes,omitempty" doc:"Order notes"`
	CreatedAt     time.Time `json:"created_at" doc:"Creation timestamp"`
	UpdatedAt     time.Time `json:"updated_at" doc:"Last update timestamp"`
}

// PurgeOrdersInput describes query parameters for purging orders.
type PurgeOrdersInput struct {
	Start    string `query:"start" example:"2026-01-01" doc:"Start date (YYYY-MM-DD)"`
	End      string `query:"end" example:"2026-06-01" doc:"End date (YYYY-MM-DD)"`
	Statuses string `query:"statuses" doc:"Comma-separated statuses to purge"`
}

// PurgeOrdersOutput is the response for the purge endpoint.
type PurgeOrdersOutput struct {
	Body struct {
		DeletedOrders int    `json:"deleted_orders" example:"10" doc:"Number of deleted orders"`
		DeletedItems  int    `json:"deleted_items" example:"25" doc:"Number of deleted items"`
		Message       string `json:"message" example:"Orders purged successfully" doc:"Result message"`
	}
}

// UploadImageInput is the payload for uploading an image.
type UploadImageInput struct {
	Body struct {
		Image huma.FormFile `form:"image" doc:"Image file (JPEG, PNG, WebP)"`
	}
}

// UploadImageOutput is the response for a successful image upload.
type UploadImageOutput struct {
	Body struct {
		Path    string `json:"path" example:"products/abc123.png" doc:"Uploaded image path"`
		Message string `json:"message" example:"Image uploaded successfully" doc:"Result message"`
	}
}

// LoginInput is the employee login payload.
type LoginInput struct {
	Body struct {
		Email    string `json:"email" example:"admin@example.com" doc:"Employee email"`
		Password string `json:"password" example:"secret" doc:"Employee password"`
	}
}

// LoginOutput is the response for a successful login.
type LoginOutput struct {
	Body struct {
		Employee struct {
			ID     string `json:"id" doc:"Employee ID"`
			Email  string `json:"email" doc:"Employee email"`
			Name   string `json:"name" doc:"Employee name"`
			Phone  string `json:"phone" doc:"Employee phone"`
			Role   string `json:"role" doc:"Employee role"`
			Active bool   `json:"active" doc:"Whether the employee is active"`
		} `json:"employee" doc:"Employee record"`
	}
}

// SessionOutput is the response for the current session.
type SessionOutput struct {
	Body struct {
		ID     string `json:"id" doc:"Employee ID"`
		Email  string `json:"email" doc:"Employee email"`
		Name   string `json:"name" doc:"Employee name"`
		Phone  string `json:"phone" doc:"Employee phone"`
		Role   string `json:"role" doc:"Employee role"`
		Active bool   `json:"active" doc:"Whether the employee is active"`
	}
}

// MessageResponse is a simple message response.
type MessageResponse struct {
	Message string `json:"message" doc:"Response message"`
}

// MessageOutput is a simple message response output.
type MessageOutput struct {
	Body MessageResponse
}
