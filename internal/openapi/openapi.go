// Package openapi generates a living OpenAPI contract for the ishos API using
// Huma in docs-only mode. The Huma operations are registered on a separate Chi
// router used only for spec generation; the existing Chi handlers continue to
// serve all real traffic.
package openapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// NewAPI builds a Huma API instance backed by a fresh Chi router. The returned
// API is used to generate the OpenAPI spec; its routes are not mounted in the
// production server.
func NewAPI() huma.API {
	router := chi.NewMux()
	config := huma.DefaultConfig("ishos API", "1.0.0")
	config.Servers = []*huma.Server{
		{URL: "/api/v1"},
	}
	api := humachi.New(router, config)
	registerOperations(api)
	return api
}

// GenerateYAML returns the OpenAPI 3.0.3 YAML representation of the API.
func GenerateYAML() ([]byte, error) {
	api := NewAPI()
	return api.OpenAPI().DowngradeYAML()
}

// GenerateJSON returns the OpenAPI 3.1 JSON representation of the API.
func GenerateJSON() ([]byte, error) {
	api := NewAPI()
	return json.MarshalIndent(api.OpenAPI(), "", "  ")
}

func registerOperations(api huma.API) {
	// ── Public catalog ───────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "list-products",
		Method:      http.MethodGet,
		Path:        "/products",
		Summary:     "List products",
		Description: "Returns a paginated list of available products.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *ListProductsInput) (*PaginatedProductsOutput, error) {
		return &PaginatedProductsOutput{Body: PaginatedResponse[Product]{Data: []Product{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-product",
		Method:      http.MethodGet,
		Path:        "/products/{id}",
		Summary:     "Get product",
		Description: "Returns a single product by ID.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *ProductIDInput) (*ProductOutput, error) {
		return &ProductOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-categories",
		Method:      http.MethodGet,
		Path:        "/categories",
		Summary:     "List categories",
		Description: "Returns a paginated list of active categories.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *ListCategoriesInput) (*PaginatedCategoriesOutput, error) {
		return &PaginatedCategoriesOutput{Body: PaginatedResponse[Category]{Data: []Category{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-category-by-slug",
		Method:      http.MethodGet,
		Path:        "/categories/{slug}",
		Summary:     "Get category by slug",
		Description: "Returns a category and its products by URL slug.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *CategorySlugInput) (*CategoryDetailOutput, error) {
		return &CategoryDetailOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-flavors",
		Method:      http.MethodGet,
		Path:        "/flavors",
		Summary:     "List flavors",
		Description: "Returns a list of active flavors.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *struct {
		All bool `query:"all" default:"false" doc:"Return all flavors"`
	}) (*PaginatedFlavorsOutput, error) {
		return &PaginatedFlavorsOutput{Body: PaginatedResponse[Flavor]{Data: []Flavor{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-flavor",
		Method:      http.MethodGet,
		Path:        "/flavors/{id}",
		Summary:     "Get flavor",
		Description: "Returns a single flavor by ID.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *FlavorIDInput) (*FlavorOutput, error) {
		return &FlavorOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-addons",
		Method:      http.MethodGet,
		Path:        "/addons",
		Summary:     "List addons",
		Description: "Returns a list of active addons.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *struct {
		All bool `query:"all" default:"false" doc:"Return all addons"`
	}) (*PaginatedAddonsOutput, error) {
		return &PaginatedAddonsOutput{Body: PaginatedResponse[Addon]{Data: []Addon{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-addon",
		Method:      http.MethodGet,
		Path:        "/addons/{id}",
		Summary:     "Get addon",
		Description: "Returns a single addon by ID.",
		Tags:        []string{"Public catalog"},
	}, func(ctx context.Context, input *AddonIDInput) (*AddonOutput, error) {
		return &AddonOutput{}, nil
	})

	// ── Public orders ────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "create-order",
		Method:      http.MethodPost,
		Path:        "/orders",
		Summary:     "Create a public order",
		Description: "Creates a new order from the public storefront.",
		Tags:        []string{"Public orders"},
	}, func(ctx context.Context, input *CreateOrderInput) (*OrderOutput, error) {
		return &OrderOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "track-order",
		Method:      http.MethodGet,
		Path:        "/orders/track",
		Summary:     "Track an order",
		Description: "Returns the current status of an order using its tracking token.",
		Tags:        []string{"Public orders"},
	}, func(ctx context.Context, input *TrackOrderInput) (*OrderTrackingOutput, error) {
		return &OrderTrackingOutput{}, nil
	})

	// ── Auth ─────────────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/auth/login",
		Summary:     "Employee login",
		Description: "Authenticates an employee and sets an HttpOnly auth cookie.",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		return &LoginOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-session",
		Method:      http.MethodGet,
		Path:        "/auth/session",
		Summary:     "Get current session",
		Description: "Returns the currently authenticated employee session.",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct{}) (*SessionOutput, error) {
		return &SessionOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/auth/logout",
		Summary:     "Logout",
		Description: "Revokes the current session and clears the auth cookie.",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct{}) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Logged out successfully"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reset-login-lockout",
		Method:      http.MethodPost,
		Path:        "/auth/lockout/reset",
		Summary:     "Reset login lockout",
		Description: "Resets the login rate-limit/lockout state for an email.",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Email string `json:"email" example:"admin@example.com" doc:"Employee email"`
		}
	}) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Login lockout reset"}}, nil
	})

	// ── Settings ─────────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "get-public-store-status",
		Method:      http.MethodGet,
		Path:        "/settings/store/public",
		Summary:     "Get public store status",
		Description: "Returns whether public orders are enabled and active offers.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *struct{}) (*StoreStatusOutput, error) {
		return &StoreStatusOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-store-settings",
		Method:      http.MethodGet,
		Path:        "/settings/store",
		Summary:     "Get store settings",
		Description: "Returns the full store settings.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *struct{}) (*StoreSettingsOutput, error) {
		return &StoreSettingsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-store-settings",
		Method:      http.MethodPatch,
		Path:        "/settings/store",
		Summary:     "Update store settings",
		Description: "Updates the store orders-enabled flag and special offers.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *StoreSettingsOutput) (*StoreSettingsOutput, error) {
		return &StoreSettingsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-archive-settings",
		Method:      http.MethodGet,
		Path:        "/settings/archive",
		Summary:     "Get archive settings",
		Description: "Returns the order archive configuration.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *struct{}) (*ArchiveSettingsOutput, error) {
		return &ArchiveSettingsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-archive-settings",
		Method:      http.MethodPatch,
		Path:        "/settings/archive",
		Summary:     "Update archive settings",
		Description: "Updates the order archive configuration.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *ArchiveSettingsOutput) (*ArchiveSettingsOutput, error) {
		return &ArchiveSettingsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-rate-limit-settings",
		Method:      http.MethodGet,
		Path:        "/settings/rate-limits",
		Summary:     "Get rate-limit settings",
		Description: "Returns the public order rate-limit configuration.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *struct{}) (*RateLimitSettingsOutput, error) {
		return &RateLimitSettingsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-rate-limit-settings",
		Method:      http.MethodPatch,
		Path:        "/settings/rate-limits",
		Summary:     "Update rate-limit settings",
		Description: "Updates the public order rate-limit configuration.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *RateLimitSettingsOutput) (*RateLimitSettingsOutput, error) {
		return &RateLimitSettingsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-panel-config",
		Method:      http.MethodGet,
		Path:        "/settings/panel-config",
		Summary:     "Get panel configuration",
		Description: "Returns the panel/session token TTL configuration.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *struct{}) (*PanelConfigOutput, error) {
		return &PanelConfigOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-panel-config",
		Method:      http.MethodPatch,
		Path:        "/settings/panel-config",
		Summary:     "Update panel configuration",
		Description: "Updates the panel/session token TTL configuration.",
		Tags:        []string{"Settings"},
	}, func(ctx context.Context, input *PanelConfigOutput) (*PanelConfigOutput, error) {
		return &PanelConfigOutput{}, nil
	})

	// ── Products management (admin) ──────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "create-product",
		Method:      http.MethodPost,
		Path:        "/products",
		Summary:     "Create product",
		Description: "Creates a new product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *CreateProductInput) (*ProductOutput, error) {
		return &ProductOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-product",
		Method:      http.MethodPatch,
		Path:        "/products/{id}",
		Summary:     "Update product",
		Description: "Updates an existing product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *UpdateProductInput) (*ProductOutput, error) {
		return &ProductOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "toggle-product-availability",
		Method:      http.MethodPatch,
		Path:        "/products/{id}/availability",
		Summary:     "Toggle product availability",
		Description: "Flips the availability flag of a product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *UpdateProductAvailabilityInput) (*ProductOutput, error) {
		return &ProductOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-product",
		Method:      http.MethodDelete,
		Path:        "/products/{id}",
		Summary:     "Delete product",
		Description: "Removes a product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *ProductIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Product deleted"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "link-product-flavor",
		Method:      http.MethodPost,
		Path:        "/products/{id}/flavors/{flavorId}",
		Summary:     "Link flavor to product",
		Description: "Links a flavor to a product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *ProductFlavorLinkInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Flavor linked"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unlink-product-flavor",
		Method:      http.MethodDelete,
		Path:        "/products/{id}/flavors/{flavorId}",
		Summary:     "Unlink flavor from product",
		Description: "Unlinks a flavor from a product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *ProductFlavorLinkInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Flavor unlinked"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "link-product-addon",
		Method:      http.MethodPost,
		Path:        "/products/{id}/addons/{addonId}",
		Summary:     "Link addon to product",
		Description: "Links an addon to a product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *ProductAddonLinkInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Addon linked"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unlink-product-addon",
		Method:      http.MethodDelete,
		Path:        "/products/{id}/addons/{addonId}",
		Summary:     "Unlink addon from product",
		Description: "Unlinks an addon from a product. Admin only.",
		Tags:        []string{"Products"},
	}, func(ctx context.Context, input *ProductAddonLinkInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Addon unlinked"}}, nil
	})

	// ── Categories management (admin) ────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "create-category",
		Method:      http.MethodPost,
		Path:        "/categories",
		Summary:     "Create category",
		Description: "Creates a new category. Admin only.",
		Tags:        []string{"Categories"},
	}, func(ctx context.Context, input *CreateCategoryInput) (*CategoryOutput, error) {
		return &CategoryOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-category",
		Method:      http.MethodPatch,
		Path:        "/categories/{id}",
		Summary:     "Update category",
		Description: "Updates an existing category. Admin only.",
		Tags:        []string{"Categories"},
	}, func(ctx context.Context, input *UpdateCategoryInput) (*CategoryOutput, error) {
		return &CategoryOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-category",
		Method:      http.MethodDelete,
		Path:        "/categories/{id}",
		Summary:     "Delete category",
		Description: "Removes a category. Admin only.",
		Tags:        []string{"Categories"},
	}, func(ctx context.Context, input *CategoryIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Category deleted"}}, nil
	})

	// ── Flavors management (admin) ───────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "create-flavor",
		Method:      http.MethodPost,
		Path:        "/flavors",
		Summary:     "Create flavor",
		Description: "Creates a new flavor. Admin only.",
		Tags:        []string{"Flavors"},
	}, func(ctx context.Context, input *CreateFlavorInput) (*FlavorOutput, error) {
		return &FlavorOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-flavor",
		Method:      http.MethodPatch,
		Path:        "/flavors/{id}",
		Summary:     "Update flavor",
		Description: "Updates an existing flavor. Admin only.",
		Tags:        []string{"Flavors"},
	}, func(ctx context.Context, input *UpdateFlavorInput) (*FlavorOutput, error) {
		return &FlavorOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-flavor",
		Method:      http.MethodDelete,
		Path:        "/flavors/{id}",
		Summary:     "Delete flavor",
		Description: "Removes a flavor. Admin only.",
		Tags:        []string{"Flavors"},
	}, func(ctx context.Context, input *FlavorIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Flavor deleted"}}, nil
	})

	// ── Addons management (admin) ────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "create-addon",
		Method:      http.MethodPost,
		Path:        "/addons",
		Summary:     "Create addon",
		Description: "Creates a new addon. Admin only.",
		Tags:        []string{"Addons"},
	}, func(ctx context.Context, input *CreateAddonInput) (*AddonOutput, error) {
		return &AddonOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-addon",
		Method:      http.MethodPatch,
		Path:        "/addons/{id}",
		Summary:     "Update addon",
		Description: "Updates an existing addon. Admin only.",
		Tags:        []string{"Addons"},
	}, func(ctx context.Context, input *UpdateAddonInput) (*AddonOutput, error) {
		return &AddonOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-addon",
		Method:      http.MethodDelete,
		Path:        "/addons/{id}",
		Summary:     "Delete addon",
		Description: "Removes an addon. Admin only.",
		Tags:        []string{"Addons"},
	}, func(ctx context.Context, input *AddonIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Addon deleted"}}, nil
	})

	// ── Orders management (admin/employee) ───────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "create-internal-order",
		Method:      http.MethodPost,
		Path:        "/orders/internal",
		Summary:     "Create internal order",
		Description: "Creates a new order from the staff panel. Requires an authenticated employee or admin session and bypasses public storefront rate limits.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *CreateOrderInput) (*OrderOutput, error) {
		return &OrderOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-orders",
		Method:      http.MethodGet,
		Path:        "/orders",
		Summary:     "List orders",
		Description: "Returns a paginated list of orders.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *ListOrdersInput) (*PaginatedOrdersOutput, error) {
		return &PaginatedOrdersOutput{Body: PaginatedResponse[Order]{Data: []Order{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-order",
		Method:      http.MethodGet,
		Path:        "/orders/{id}",
		Summary:     "Get order",
		Description: "Returns a single order by ID.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *OrderIDInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-order",
		Method:      http.MethodPatch,
		Path:        "/orders/{id}",
		Summary:     "Update order",
		Description: "Updates an editable order.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *UpdateOrderInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "approve-order",
		Method:      http.MethodPost,
		Path:        "/orders/{id}/approve",
		Summary:     "Approve order",
		Description: "Approves an order for preparation.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *OrderIDInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reject-order",
		Method:      http.MethodPost,
		Path:        "/orders/{id}/reject",
		Summary:     "Reject order",
		Description: "Rejects an order with a reason.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *RejectOrderInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-order-status",
		Method:      http.MethodPatch,
		Path:        "/orders/{id}/status",
		Summary:     "Update order status",
		Description: "Transitions an order to a new status.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *UpdateOrderStatusInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-order-notes",
		Method:      http.MethodPatch,
		Path:        "/orders/{id}/notes",
		Summary:     "Update order notes",
		Description: "Updates the notes of an order.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *UpdateOrderNotesInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archive-order",
		Method:      http.MethodPatch,
		Path:        "/orders/{id}/archive",
		Summary:     "Archive order",
		Description: "Toggles the archive flag of an order. Admin only.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *ArchiveOrderInput) (*OrderOutputSingle, error) {
		return &OrderOutputSingle{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-order",
		Method:      http.MethodDelete,
		Path:        "/orders/{id}",
		Summary:     "Delete order",
		Description: "Deletes an editable order.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *OrderIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Order deleted"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "run-order-archive",
		Method:      http.MethodPost,
		Path:        "/admin/orders/archive/run",
		Summary:     "Run order archive",
		Description: "Triggers an immediate auto-archive pass. Admin only.",
		Tags:        []string{"Orders"},
	}, func(ctx context.Context, input *struct{}) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Archivo ejecutado"}}, nil
	})

	// ── Container types (admin) ──────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "list-container-types",
		Method:      http.MethodGet,
		Path:        "/container-types",
		Summary:     "List container types",
		Description: "Returns all container types.",
		Tags:        []string{"Container types"},
	}, func(ctx context.Context, input *struct{}) (*PaginatedContainerTypesOutput, error) {
		return &PaginatedContainerTypesOutput{Body: PaginatedResponse[ContainerType]{Data: []ContainerType{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-container-type",
		Method:      http.MethodPost,
		Path:        "/container-types",
		Summary:     "Create container type",
		Description: "Creates a new container type. Admin only.",
		Tags:        []string{"Container types"},
	}, func(ctx context.Context, input *CreateContainerTypeInput) (*ContainerTypeOutput, error) {
		return &ContainerTypeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-container-type",
		Method:      http.MethodGet,
		Path:        "/container-types/{id}",
		Summary:     "Get container type",
		Description: "Returns a single container type.",
		Tags:        []string{"Container types"},
	}, func(ctx context.Context, input *ContainerTypeIDInput) (*ContainerTypeOutput, error) {
		return &ContainerTypeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-container-type",
		Method:      http.MethodPatch,
		Path:        "/container-types/{id}",
		Summary:     "Update container type",
		Description: "Updates an existing container type. Admin only.",
		Tags:        []string{"Container types"},
	}, func(ctx context.Context, input *UpdateContainerTypeInput) (*ContainerTypeOutput, error) {
		return &ContainerTypeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-container-type",
		Method:      http.MethodDelete,
		Path:        "/container-types/{id}",
		Summary:     "Delete container type",
		Description: "Removes a container type. Admin only.",
		Tags:        []string{"Container types"},
	}, func(ctx context.Context, input *ContainerTypeIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Container type deleted"}}, nil
	})

	// ── Inventory (admin) ────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "list-inventory",
		Method:      http.MethodGet,
		Path:        "/inventory",
		Summary:     "List inventory",
		Description: "Returns all inventory items.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *struct{}) (*PaginatedInventoryOutput, error) {
		return &PaginatedInventoryOutput{Body: PaginatedResponse[InventoryItem]{Data: []InventoryItem{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-inventory-dashboard",
		Method:      http.MethodGet,
		Path:        "/inventory/dashboard",
		Summary:     "Inventory dashboard",
		Description: "Returns flavor inventory, unit inventory and stats in one response.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *struct{}) (*InventoryDashboardOutput, error) {
		return &InventoryDashboardOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-flavor-inventory",
		Method:      http.MethodGet,
		Path:        "/inventory/flavors",
		Summary:     "List flavor inventory",
		Description: "Returns ball-based inventory items linked to flavors.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *struct{}) (*PaginatedInventoryOutput, error) {
		return &PaginatedInventoryOutput{Body: PaginatedResponse[InventoryItem]{Data: []InventoryItem{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-unit-inventory",
		Method:      http.MethodGet,
		Path:        "/inventory/unit-products",
		Summary:     "List unit inventory",
		Description: "Returns unit-based inventory items linked to products.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *struct{}) (*PaginatedInventoryOutput, error) {
		return &PaginatedInventoryOutput{Body: PaginatedResponse[InventoryItem]{Data: []InventoryItem{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-unit-inventory-item",
		Method:      http.MethodPost,
		Path:        "/inventory/unit-products",
		Summary:     "Create unit inventory item",
		Description: "Creates a new unit-based inventory item.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *CreateUnitInventoryItemInput) (*InventoryItemOutput, error) {
		return &InventoryItemOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-inventory-stats",
		Method:      http.MethodGet,
		Path:        "/inventory/stats",
		Summary:     "Inventory stats",
		Description: "Returns aggregate inventory statistics.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *struct{}) (*struct{ Body InventoryStats }, error) {
		return &struct{ Body InventoryStats }{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-low-stock",
		Method:      http.MethodGet,
		Path:        "/inventory/low-stock",
		Summary:     "Low stock items",
		Description: "Returns inventory items below their low-stock threshold.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *struct{}) (*PaginatedInventoryOutput, error) {
		return &PaginatedInventoryOutput{Body: PaginatedResponse[InventoryItem]{Data: []InventoryItem{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-inventory-item",
		Method:      http.MethodGet,
		Path:        "/inventory/{id}",
		Summary:     "Get inventory item",
		Description: "Returns a single inventory item.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *InventoryItemIDInput) (*InventoryItemOutput, error) {
		return &InventoryItemOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-inventory-movements",
		Method:      http.MethodGet,
		Path:        "/inventory/{id}/movements",
		Summary:     "Get inventory movement history",
		Description: "Returns stock movements for an inventory item.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *InventoryItemIDInput) (*InventoryMovementsOutput, error) {
		return &InventoryMovementsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "record-inventory-entry",
		Method:      http.MethodPost,
		Path:        "/inventory/{id}/entries",
		Summary:     "Record inventory entry",
		Description: "Records a flavor stock entry.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *RecordEntryInput) (*struct{ Body StockMovement }, error) {
		return &struct{ Body StockMovement }{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "record-inventory-unit-entry",
		Method:      http.MethodPost,
		Path:        "/inventory/{id}/unit-entries",
		Summary:     "Record unit inventory entry",
		Description: "Records a unit-based stock entry.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *RecordUnitEntryInput) (*struct{ Body StockMovement }, error) {
		return &struct{ Body StockMovement }{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "record-inventory-adjustment",
		Method:      http.MethodPost,
		Path:        "/inventory/{id}/adjustments",
		Summary:     "Record inventory adjustment",
		Description: "Records a manual stock adjustment.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *RecordAdjustmentInput) (*struct{ Body StockMovement }, error) {
		return &struct{ Body StockMovement }{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "link-inventory-product",
		Method:      http.MethodPost,
		Path:        "/inventory/{id}/products/{productId}",
		Summary:     "Link inventory item to product",
		Description: "Links an inventory item to a product.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *InventoryProductLinkInput) (*InventoryItemOutput, error) {
		return &InventoryItemOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unlink-inventory-product",
		Method:      http.MethodDelete,
		Path:        "/inventory/{id}/products/{productId}",
		Summary:     "Unlink inventory item from product",
		Description: "Removes the product link from an inventory item.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *InventoryProductLinkInput) (*InventoryItemOutput, error) {
		return &InventoryItemOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-flavor-stock",
		Method:      http.MethodGet,
		Path:        "/inventory/flavors/{id}/stock",
		Summary:     "Get flavor stock",
		Description: "Returns stock status for a flavor.",
		Tags:        []string{"Inventory"},
	}, func(ctx context.Context, input *InventoryFlavorIDInput) (*FlavorStockOutput, error) {
		return &FlavorStockOutput{}, nil
	})

	// ── Users (admin/employee) ───────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/users",
		Summary:     "List users",
		Description: "Returns a paginated list of customers.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *ListUsersInput) (*PaginatedUsersOutput, error) {
		return &PaginatedUsersOutput{Body: PaginatedResponse[User]{Data: []User{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/users/{id}",
		Summary:     "Get user",
		Description: "Returns a single customer by ID.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *UserIDInput) (*UserOutput, error) {
		return &UserOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-user-orders",
		Method:      http.MethodGet,
		Path:        "/users/{id}/orders",
		Summary:     "Get user orders",
		Description: "Returns order history for a customer. Admin only.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *UserIDInput) (*UserOrdersOutput, error) {
		return &UserOrdersOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-user",
		Method:      http.MethodPost,
		Path:        "/users",
		Summary:     "Create user",
		Description: "Creates a new customer. Admin only.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *CreateUserInput) (*UserOutput, error) {
		return &UserOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "upsert-user",
		Method:      http.MethodPost,
		Path:        "/users/upsert",
		Summary:     "Upsert user",
		Description: "Creates or updates a customer by phone. Admin only.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *CreateUserInput) (*UserOutput, error) {
		return &UserOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPatch,
		Path:        "/users/{id}",
		Summary:     "Update user",
		Description: "Updates an existing customer. Admin only.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *UpdateUserInput) (*UserOutput, error) {
		return &UserOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/users/{id}",
		Summary:     "Delete user",
		Description: "Removes a customer. Admin only.",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *UserIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Usuario eliminado"}}, nil
	})

	// ── Employees (admin) ────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "list-employees",
		Method:      http.MethodGet,
		Path:        "/employees",
		Summary:     "List employees",
		Description: "Returns all employee/admin records.",
		Tags:        []string{"Employees"},
	}, func(ctx context.Context, input *struct{}) (*PaginatedEmployeesOutput, error) {
		return &PaginatedEmployeesOutput{Body: PaginatedResponse[Employee]{Data: []Employee{}}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-employee",
		Method:      http.MethodPost,
		Path:        "/employees",
		Summary:     "Create employee",
		Description: "Registers a new employee in the auth collection. Admin only.",
		Tags:        []string{"Employees"},
	}, func(ctx context.Context, input *CreateEmployeeInput) (*EmployeeOutput, error) {
		return &EmployeeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-employee",
		Method:      http.MethodPatch,
		Path:        "/employees/{id}",
		Summary:     "Update employee",
		Description: "Updates an employee account. Admin only.",
		Tags:        []string{"Employees"},
	}, func(ctx context.Context, input *UpdateEmployeeInput) (*EmployeeOutput, error) {
		return &EmployeeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "deactivate-employee",
		Method:      http.MethodPatch,
		Path:        "/employees/{id}/deactivate",
		Summary:     "Deactivate employee",
		Description: "Disables an employee account. Admin only.",
		Tags:        []string{"Employees"},
	}, func(ctx context.Context, input *EmployeeIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Employee deactivated"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-employee",
		Method:      http.MethodDelete,
		Path:        "/employees/{id}",
		Summary:     "Delete employee",
		Description: "Permanently removes an employee account. Admin only.",
		Tags:        []string{"Employees"},
	}, func(ctx context.Context, input *EmployeeIDInput) (*MessageOutput, error) {
		return &MessageOutput{Body: MessageResponse{Message: "Employee deleted"}}, nil
	})

	// ── Analytics (admin) ────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "analytics-overview",
		Method:      http.MethodGet,
		Path:        "/analytics/overview",
		Summary:     "Analytics overview",
		Description: "Returns aggregate order metrics for a period.",
		Tags:        []string{"Analytics"},
	}, func(ctx context.Context, input *AnalyticsOverviewInput) (*AnalyticsOverviewOutput, error) {
		return &AnalyticsOverviewOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "analytics-orders-over-time",
		Method:      http.MethodGet,
		Path:        "/analytics/orders-over-time",
		Summary:     "Orders over time",
		Description: "Returns order counts and revenue over a date range.",
		Tags:        []string{"Analytics"},
	}, func(ctx context.Context, input *AnalyticsOrdersOverTimeInput) (*AnalyticsOrdersOverTimeOutput, error) {
		return &AnalyticsOrdersOverTimeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "analytics-top-products",
		Method:      http.MethodGet,
		Path:        "/analytics/top-products",
		Summary:     "Top products",
		Description: "Returns the best-selling products for a date range.",
		Tags:        []string{"Analytics"},
	}, func(ctx context.Context, input *AnalyticsTopProductsInput) (*AnalyticsTopProductsOutput, error) {
		return &AnalyticsTopProductsOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "analytics-dashboard",
		Method:      http.MethodGet,
		Path:        "/analytics/dashboard",
		Summary:     "Analytics dashboard",
		Description: "Returns overview, orders-over-time, and top-products in a single response.",
		Tags:        []string{"Analytics"},
	}, func(ctx context.Context, input *AnalyticsDashboardInput) (*AnalyticsDashboardOutput, error) {
		return &AnalyticsDashboardOutput{}, nil
	})

	// ── Export (admin) ───────────────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "export-orders",
		Method:      http.MethodGet,
		Path:        "/export/orders",
		Summary:     "Export orders",
		Description: "Exports orders to CSV or JSON. Admin only.",
		Tags:        []string{"Export"},
	}, func(ctx context.Context, input *ExportOrdersInput) (*ExportOrdersOutput, error) {
		return &ExportOrdersOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "purge-orders",
		Method:      http.MethodDelete,
		Path:        "/export/orders",
		Summary:     "Purge orders",
		Description: "Deletes completed/cancelled orders in a date range. Admin only.",
		Tags:        []string{"Export"},
	}, func(ctx context.Context, input *PurgeOrdersInput) (*PurgeOrdersOutput, error) {
		return &PurgeOrdersOutput{}, nil
	})

	// ── Upload (admin/employee) ──────────────────────────────────────────
	huma.Register(api, huma.Operation{
		OperationID: "upload-image",
		Method:      http.MethodPost,
		Path:        "/upload/image",
		Summary:     "Upload image",
		Description: "Uploads an image to object storage.",
		Tags:        []string{"Upload"},
	}, func(ctx context.Context, input *UploadImageInput) (*UploadImageOutput, error) {
		return &UploadImageOutput{}, nil
	})
}
