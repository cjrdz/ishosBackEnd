# Column-name audit for Bun model tags

This document records the exact table and column names used by Bun models, originally derived from the legacy ORM schema, so model tags match the existing database without migration.

Source: `internal/models/models.go`.

## Table names

| Entity            | Table name             |
|-------------------|------------------------|
| Addon             | addons                 |
| AdminSetting      | admin_settings         |
| AuditLog          | audit_logs             |
| Category          | categories             |
| ContainerType     | container_types        |
| Customer          | customers              |
| Flavor            | flavors                |
| InventoryItem     | inventory_items        |
| Order             | orders                 |
| OrderItem         | order_items            |
| OrderTrackingToken| order_tracking_tokens  |
| Product           | products               |
| RevokedToken      | revoked_tokens         |
| StockMovement     | stock_movements        |

## Notable column names (non-obvious FKs)

| Model             | Field in model            | Actual column in Postgres        |
|-------------------|---------------------------|----------------------------------|
| Order             | Customer (edge)           | customer_id                      |
| OrderItem         | Order (edge)              | order_items                      |
| OrderItem         | Product (edge)            | product_order_items              |
| Product           | Category (edge)           | category_products                |
| InventoryItem     | Flavor (edge)             | flavor_id                        |
| InventoryItem     | Product (edge)            | product_id                       |
| StockMovement     | ContainerType (edge)      | container_type_id                |
| StockMovement     | Flavor (edge)             | flavor_id                        |
| StockMovement     | InventoryItem (edge)      | inventory_item_id                |
| Order             | ReviewedByUserID          | reviewed_by_ory_id               |
| Order             | CreatedByUserID           | created_by_ory_id                |
| OrderItem         | SnapshotProductID         | snapshot_product_id              |

## Enum values

| Table/column                | Values                                                                |
|-----------------------------|-----------------------------------------------------------------------|
| customers.user_type         | user, company                                                         |
| customers.status            | active, inactive                                                      |
| inventory_items.type        | ball_based, unit_based                                                |
| orders.status               | pendiente_revision, recibida, en_proceso, lista, entregada, cancelada |
| orders.payment_method       | efectivo, tarjeta, transferencia, otro                                |
| orders.payment_status       | pendiente, confirmado                                                 |
| orders.order_type           | en_local, para_llevar                                                 |
| stock_movements.type        | entry, sale, adjustment                                               |

## Index names to recreate

Keep these index names in `EnsureBunSchema` so query plans stay stable:

- `addon_group_name_display_order_is_active`
- `auditlog_action_created_at`
- `auditlog_actor_id_created_at`
- `auditlog_resource`
- `auditlog_created_at`
- `containertype_is_active`
- `customer_status`
- `customer_name`
- `flavor_display_order_is_active`
- `inventoryitem_type`
- `inventoryitem_product_id`
- `inventoryitem_flavor_id`
- `order_status`
- `order_customer_phone`
- `order_customer_id`
- `order_created_at`
- `order_status_created_at`
- `ordertrackingtoken_order_number`
- `ordertrackingtoken_expires_at`
- `ordertrackingtoken_revoked_at`
- `product_is_available`
- `revokedtoken_token_hash_expires_at`
- `revokedtoken_expires_at`
- `stockmovement_type`
- `stockmovement_inventory_item_id_created_at`
- `stockmovement_order_id`
