// Package models contains Bun-tagged structs that mirror the existing database tables.
// These structs are intentionally boring: one file per table, explicit column names,
// and no business logic. They exist so Bun can create tables, scan rows, and build
// queries against the schema managed by database.EnsureBunSchema.
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Addon mirrors the addons table.
type Addon struct {
	bun.BaseModel `bun:"table:addons,alias:a"`
	ID            uuid.UUID `bun:",pk,type:uuid"`
	Name          string    `bun:",notnull,type:varchar(100),unique"`
	Price         float64   `bun:",notnull,default:0"`
	GroupName     string    `bun:",notnull,type:varchar(50),default:'extras'"`
	DisplayOrder  int       `bun:",notnull,default:0"`
	IsActive      bool      `bun:",notnull,default:true"`
	CreatedAt     time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:",notnull,default:current_timestamp"`
}

// AdminSetting mirrors the admin_settings table.
type AdminSetting struct {
	bun.BaseModel `bun:"table:admin_settings,alias:as"`
	ID            uuid.UUID      `bun:",pk,type:uuid"`
	Key           string         `bun:",notnull,type:varchar(120),unique"`
	Value         map[string]any `bun:",type:jsonb,default:'{}'"`
	ChangedBy     *string        `bun:",type:varchar(255)"`
	CreatedAt     time.Time      `bun:",notnull,default:current_timestamp"`
	UpdatedAt     time.Time      `bun:",notnull,default:current_timestamp"`
}

// AuditLog mirrors the audit_logs table.
type AuditLog struct {
	bun.BaseModel `bun:"table:audit_logs,alias:al"`
	ID            uuid.UUID      `bun:",pk,type:uuid"`
	Action        string         `bun:",notnull"`
	ActorID       *string        `bun:",type:varchar(255)"`
	Resource      string         `bun:",notnull"`
	Changes       map[string]any `bun:",type:jsonb"`
	IP            *string        `bun:",type:varchar(255)"`
	CreatedAt     time.Time      `bun:",notnull,default:current_timestamp"`
}

// Category mirrors the categories table.
type Category struct {
	bun.BaseModel `bun:"table:categories,alias:c"`
	ID            uuid.UUID `bun:",pk,type:uuid"`
	Name          string    `bun:",notnull,type:varchar(100),unique"`
	Slug          string    `bun:",notnull,type:varchar(100),unique"`
	Description   *string   `bun:",type:text,default:''"`
	ImagePath     *string   `bun:",type:varchar(255)"`
	DisplayOrder  int       `bun:",notnull,default:0"`
	IsActive      bool      `bun:",notnull,default:true"`
	CreatedAt     time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:",notnull,default:current_timestamp"`

	Products []Product `bun:"rel:has-many,join:id=category_products"`
}

// ContainerType mirrors the container_types table.
type ContainerType struct {
	bun.BaseModel     `bun:"table:container_types,alias:ct"`
	ID                uuid.UUID `bun:",pk,type:uuid"`
	Name              string    `bun:",notnull,type:varchar(100),unique"`
	BallsPerContainer int       `bun:"balls_per_container,notnull,default:0"`
	IsCustom          bool      `bun:",notnull,default:false"`
	IsActive          bool      `bun:",notnull,default:true"`
	CreatedAt         time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt         time.Time `bun:",notnull,default:current_timestamp"`
}

// CustomerUserType is the enum for customers.user_type.
type CustomerUserType string

const (
	CustomerUserTypeUser    CustomerUserType = "user"
	CustomerUserTypeCompany CustomerUserType = "company"
)

// CustomerStatus is the enum for customers.status.
type CustomerStatus string

const (
	CustomerStatusActive   CustomerStatus = "active"
	CustomerStatusInactive CustomerStatus = "inactive"
)

// Customer mirrors the customers table.
type Customer struct {
	bun.BaseModel `bun:"table:customers,alias:cust"`
	ID            uuid.UUID        `bun:",pk,type:uuid"`
	Name          string           `bun:",notnull,type:varchar(200)"`
	UserType      CustomerUserType `bun:"user_type,notnull,type:varchar(20),default:'user'"`
	Phone         string           `bun:",notnull,type:varchar(20),unique"`
	Email         *string          `bun:",type:varchar(255),unique"`
	Status        CustomerStatus   `bun:"status,notnull,type:varchar(20),default:'active'"`
	CreatedAt     time.Time        `bun:",notnull,default:current_timestamp"`
	UpdatedAt     time.Time        `bun:",notnull,default:current_timestamp"`
}

// Flavor mirrors the flavors table.
type Flavor struct {
	bun.BaseModel `bun:"table:flavors,alias:f"`
	ID            uuid.UUID `bun:",pk,type:uuid"`
	Name          string    `bun:",notnull,type:varchar(100),unique"`
	DisplayOrder  int       `bun:",notnull,default:0"`
	IsSeasonal    bool      `bun:",notnull,default:false"`
	IsActive      bool      `bun:",notnull,default:true"`
	CreatedAt     time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:",notnull,default:current_timestamp"`
}

// InventoryItemType is the enum for inventory_items.type.
type InventoryItemType string

const (
	InventoryItemTypeBallBased InventoryItemType = "ball_based"
	InventoryItemTypeUnitBased InventoryItemType = "unit_based"
)

// InventoryItem mirrors the inventory_items table.
type InventoryItem struct {
	bun.BaseModel     `bun:"table:inventory_items,alias:ii"`
	ID                uuid.UUID         `bun:",pk,type:uuid"`
	Name              string            `bun:",notnull,type:varchar(200),unique"`
	Type              InventoryItemType `bun:"type,notnull,type:varchar(20),default:'ball_based'"`
	CurrentStock      int               `bun:"current_stock,notnull,default:0"`
	LowStockThreshold int               `bun:"low_stock_threshold,notnull,default:0"`
	CreatedAt         time.Time         `bun:",notnull,default:current_timestamp"`
	UpdatedAt         time.Time         `bun:",notnull,default:current_timestamp"`
	FlavorID          *uuid.UUID        `bun:"flavor_id,type:uuid"`
	ProductID         *uuid.UUID        `bun:"product_id,type:uuid"`
}

// OrderStatus is the enum for orders.status.
type OrderStatus string

const (
	OrderStatusPendienteRevision OrderStatus = "pendiente_revision"
	OrderStatusRecibida          OrderStatus = "recibida"
	OrderStatusEnProceso         OrderStatus = "en_proceso"
	OrderStatusLista             OrderStatus = "lista"
	OrderStatusEntregada         OrderStatus = "entregada"
	OrderStatusCancelada         OrderStatus = "cancelada"
)

// OrderPaymentMethod is the enum for orders.payment_method.
type OrderPaymentMethod string

const (
	OrderPaymentMethodEfectivo      OrderPaymentMethod = "efectivo"
	OrderPaymentMethodTarjeta       OrderPaymentMethod = "tarjeta"
	OrderPaymentMethodTransferencia OrderPaymentMethod = "transferencia"
	OrderPaymentMethodOtro          OrderPaymentMethod = "otro"
)

// OrderPaymentStatus is the enum for orders.payment_status.
type OrderPaymentStatus string

const (
	OrderPaymentStatusPendiente  OrderPaymentStatus = "pendiente"
	OrderPaymentStatusConfirmado OrderPaymentStatus = "confirmado"
)

// OrderType is the enum for orders.order_type.
type OrderType string

const (
	OrderTypeEnLocal    OrderType = "en_local"
	OrderTypeParaLlevar OrderType = "para_llevar"
)

// Order mirrors the orders table.
type Order struct {
	bun.BaseModel    `bun:"table:orders,alias:o"`
	ID               uuid.UUID            `bun:",pk,type:uuid"`
	OrderNumber      string               `bun:"order_number,notnull,unique"`
	CustomerID       *uuid.UUID           `bun:"customer_id,type:uuid"`
	CustomerName     string               `bun:"customer_name,notnull,type:varchar(200)"`
	CustomerPhone    string               `bun:"customer_phone,notnull,type:varchar(20)"`
	CustomerEmail    *string              `bun:"customer_email,type:varchar(255)"`
	CustomerIP       *string              `bun:"customer_ip,type:varchar(255)"`
	TotalAmount      float64              `bun:"total_amount,notnull"`
	AmountReceived   *float64             `bun:"amount_received"`
	Status           OrderStatus          "bun:\"status,notnull,type:varchar(40),default:\\\"pendiente_revision\\\"\""
	PaymentMethod    OrderPaymentMethod   "bun:\"payment_method,notnull,type:varchar(20),default:\\\"efectivo\\\"\""
	PaymentStatus    OrderPaymentStatus   "bun:\"payment_status,notnull,type:varchar(20),default:\\\"pendiente\\\"\""
	OrderType        OrderType            "bun:\"order_type,notnull,type:varchar(20),default:\\\"para_llevar\\\"\""
	TableNumber      *int                 `bun:"table_number"`
	Notes            *string              `bun:"notes,type:text"`
	RejectionReason  *string              `bun:"rejection_reason,type:text"`
	ReviewedByUserID *string              `bun:"reviewed_by_ory_id,type:varchar(255)"`
	ReviewedAt       *time.Time           `bun:"reviewed_at"`
	CreatedByUserID  *string              `bun:"created_by_ory_id,type:varchar(255)"`
	IsArchived       bool                 `bun:"is_archived,notnull,default:false"`
	StatusTimestamps map[string]time.Time `bun:"status_timestamps,type:jsonb"`
	CreatedAt        time.Time            `bun:",notnull,default:current_timestamp"`
	UpdatedAt        time.Time            `bun:",notnull,default:current_timestamp"`

	Items []OrderItem `bun:"rel:has-many,join:id=order_items"`
}

// OrderItem mirrors the order_items table.
type OrderItem struct {
	bun.BaseModel     `bun:"table:order_items,alias:oi"`
	ID                uuid.UUID      `bun:",pk,type:uuid"`
	ProductName       string         `bun:"product_name,notnull,type:varchar(200)"`
	SnapshotProductID *uuid.UUID     `bun:"snapshot_product_id,type:uuid"`
	Quantity          int            `bun:",notnull"`
	UnitPrice         float64        `bun:",notnull"`
	Subtotal          float64        `bun:",notnull"`
	Customizations    map[string]any `bun:",type:jsonb"`
	OrderID           uuid.UUID      `bun:"order_items,notnull,type:uuid"`
	ProductID         uuid.UUID      `bun:"product_order_items,notnull,type:uuid"`
}

// OrderTrackingToken mirrors the order_tracking_tokens table.
type OrderTrackingToken struct {
	bun.BaseModel `bun:"table:order_tracking_tokens,alias:ott"`
	ID            int64      `bun:",pk,autoincrement"`
	TokenHash     string     `bun:"token_hash,notnull,unique"`
	OrderNumber   string     `bun:"order_number,notnull"`
	ExpiresAt     time.Time  `bun:"expires_at,notnull"`
	IssuedAt      time.Time  `bun:"issued_at,notnull"`
	RevokedAt     *time.Time `bun:"revoked_at"`
}

// IdempotencyKey records a client-supplied key for an order creation
// request, allowing a retried or duplicated request to return the
// original order instead of creating a new one.
type IdempotencyKey struct {
	bun.BaseModel `bun:"table:idempotency_keys,alias:ik"`
	Key           uuid.UUID `bun:",pk,type:uuid"`
	OrderID       uuid.UUID `bun:"order_id,notnull,type:uuid"`
	CreatedAt     time.Time `bun:"created_at,notnull"`
	ExpiresAt     time.Time `bun:"expires_at,notnull"`
}

// Product mirrors the products table.
type Product struct {
	bun.BaseModel        `bun:"table:products,alias:p"`
	ID                   uuid.UUID `bun:",pk,type:uuid"`
	Name                 string    `bun:",notnull,type:varchar(200)"`
	Description          string    `bun:",notnull,type:text"`
	Price                float64   `bun:",notnull"`
	ImagePath            *string   `bun:",type:varchar(255)"`
	IsAvailable          bool      `bun:",notnull,default:true"`
	ExcludeGlobalFlavors bool      `bun:"exclude_global_flavors,notnull,default:false"`
	ExcludeGlobalAddons  bool      `bun:"exclude_global_addons,notnull,default:false"`
	BallBased            bool      `bun:"ball_based,notnull,default:false"`
	BallQuantity         int       `bun:"ball_quantity,notnull,default:0"`
	AllowsMixedFlavors   bool      `bun:"allows_mixed_flavors,notnull,default:false"`
	FreeToppings         int       `bun:"free_toppings,notnull,default:1"`
	CreatedAt            time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt            time.Time `bun:",notnull,default:current_timestamp"`
	CategoryID           uuid.UUID `bun:"category_products,notnull,type:uuid"`
}

// RevokedToken mirrors the revoked_tokens table.
type RevokedToken struct {
	bun.BaseModel `bun:"table:revoked_tokens,alias:rt"`
	ID            int64     `bun:",pk,autoincrement"`
	TokenHash     string    `bun:"token_hash,notnull,unique"`
	ExpiresAt     time.Time `bun:"expires_at,notnull"`
	RevokedAt     time.Time `bun:"revoked_at,notnull"`
}

// StockMovementType is the enum for stock_movements.type.
type StockMovementType string

const (
	StockMovementTypeEntry      StockMovementType = "entry"
	StockMovementTypeSale       StockMovementType = "sale"
	StockMovementTypeAdjustment StockMovementType = "adjustment"
)

// StockMovement mirrors the stock_movements table.
type StockMovement struct {
	bun.BaseModel      `bun:"table:stock_movements,alias:sm"`
	ID                 uuid.UUID         `bun:",pk,type:uuid"`
	Type               StockMovementType `bun:"type,notnull,type:varchar(20)"`
	Quantity           int               `bun:",notnull,default:0"`
	QuantityContainers *int              `bun:"quantity_containers"`
	InventoryItemID    uuid.UUID         `bun:"inventory_item_id,notnull,type:uuid"`
	ContainerTypeID    *uuid.UUID        `bun:"container_type_id,type:uuid"`
	FlavorID           *uuid.UUID        `bun:"flavor_id,type:uuid"`
	Reason             *string           `bun:"reason,type:text"`
	OrderID            *uuid.UUID        `bun:"order_id,type:uuid"`
	CreatedByUserID    *string           `bun:"created_by_user_id,type:varchar(255)"`
	CreatedAt          time.Time         `bun:",notnull,default:current_timestamp"`
}
