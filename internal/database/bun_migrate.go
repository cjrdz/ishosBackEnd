// bun_migrate.go — Bun-based schema creation and post-migration backfills.
// This mirrors the behavior of the old schema creation + raw-SQL backfills.
package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// EnsureBunSchema creates tables and indexes if they do not exist, then runs
// idempotent backfills for columns/data that the previous auto-migration historically
// missed. It is safe to run against an already-migrated database.
func EnsureBunSchema(ctx context.Context, db *bun.DB) error {
	if err := createTables(ctx, db); err != nil {
		return fmt.Errorf("create bun tables: %w", err)
	}
	if err := createIndexes(ctx, db); err != nil {
		return fmt.Errorf("create bun indexes: %w", err)
	}
	if err := runBackfills(ctx, db); err != nil {
		return fmt.Errorf("run backfills: %w", err)
	}
	return nil
}

func createTables(ctx context.Context, db *bun.DB) error {
	// Order matters only for tables that reference each other, but Postgres
	// foreign keys are checked at statement time. Creating tables first, then
	// adding FKs later, avoids ordering problems. Bun's NewCreateTable does not
	// emit FKs by default; we add them explicitly below.
	tables := []interface{}{
		(*models.Addon)(nil),
		(*models.AdminSetting)(nil),
		(*models.AuditLog)(nil),
		(*models.Category)(nil),
		(*models.ContainerType)(nil),
		(*models.Customer)(nil),
		(*models.Flavor)(nil),
		(*models.InventoryItem)(nil),
		(*models.Order)(nil),
		(*models.OrderItem)(nil),
		(*models.OrderTrackingToken)(nil),
		(*models.IdempotencyKey)(nil),
		(*models.Product)(nil),
		(*models.RevokedToken)(nil),
		(*models.StockMovement)(nil),
	}

	for _, m := range tables {
		_, err := db.NewCreateTable().Model(m).IfNotExists().Exec(ctx)
		if err != nil {
			return fmt.Errorf("create table for %T: %w", m, err)
		}
	}

	return createForeignKeys(ctx, db)
}

func createForeignKeys(ctx context.Context, db *bun.DB) error {
	// Bun's NewCreateTable does not emit foreign keys by default for Postgres.
	// Add them idempotently so relation queries and cascades behave like the legacy schema.
	fks := []struct {
		name string
		sql  string
	}{
		{
			name: "orders_customers_customer",
			sql:  `ALTER TABLE orders ADD CONSTRAINT orders_customers_customer FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE SET NULL`,
		},
		{
			name: "order_items_orders_items",
			sql:  `ALTER TABLE order_items ADD CONSTRAINT order_items_orders_items FOREIGN KEY (order_items) REFERENCES orders(id)`,
		},
		{
			name: "order_items_products_order_items",
			sql:  `ALTER TABLE order_items ADD CONSTRAINT order_items_products_order_items FOREIGN KEY (product_order_items) REFERENCES products(id)`,
		},
		{
			name: "products_categories_products",
			sql:  `ALTER TABLE products ADD CONSTRAINT products_categories_products FOREIGN KEY (category_products) REFERENCES categories(id)`,
		},
		{
			name: "inventory_items_flavors_inventory_item",
			sql:  `ALTER TABLE inventory_items ADD CONSTRAINT inventory_items_flavors_inventory_item FOREIGN KEY (flavor_id) REFERENCES flavors(id) ON DELETE SET NULL`,
		},
		{
			name: "inventory_items_products_inventory_item",
			sql:  `ALTER TABLE inventory_items ADD CONSTRAINT inventory_items_products_inventory_item FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE SET NULL`,
		},
		{
			name: "stock_movements_container_types_stock_movements",
			sql:  `ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_container_types_stock_movements FOREIGN KEY (container_type_id) REFERENCES container_types(id) ON DELETE SET NULL`,
		},
		{
			name: "stock_movements_flavors_flavor_stock_movements",
			sql:  `ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_flavors_flavor_stock_movements FOREIGN KEY (flavor_id) REFERENCES flavors(id) ON DELETE SET NULL`,
		},
		{
			name: "stock_movements_inventory_items_stock_movements",
			sql:  `ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_inventory_items_stock_movements FOREIGN KEY (inventory_item_id) REFERENCES inventory_items(id)`,
		},
	}

	for _, fk := range fks {
		_, err := db.ExecContext(ctx, fk.sql+` ON UPDATE NO ACTION NOT VALID`)
		if err != nil {
			// Ignore duplicate constraint errors so the migration is idempotent.
			if !isDuplicateConstraint(err) {
				return fmt.Errorf("add FK %s: %w", fk.name, err)
			}
		}
	}
	return nil
}

func createIndexes(ctx context.Context, db *bun.DB) error {
	indexes := []struct {
		name  string
		table string
		cols  []string
	}{
		{"addon_group_name_display_order_is_active", "addons", []string{"group_name", "display_order", "is_active"}},
		{"auditlog_action_created_at", "audit_logs", []string{"action", "created_at"}},
		{"auditlog_actor_id_created_at", "audit_logs", []string{"actor_id", "created_at"}},
		{"auditlog_resource", "audit_logs", []string{"resource"}},
		{"auditlog_created_at", "audit_logs", []string{"created_at"}},
		{"containertype_is_active", "container_types", []string{"is_active"}},
		{"customer_status", "customers", []string{"status"}},
		{"customer_name", "customers", []string{"name"}},
		{"flavor_display_order_is_active", "flavors", []string{"display_order", "is_active"}},
		{"inventoryitem_type", "inventory_items", []string{"type"}},
		{"inventoryitem_product_id", "inventory_items", []string{"product_id"}},
		{"inventoryitem_flavor_id", "inventory_items", []string{"flavor_id"}},
		{"order_status", "orders", []string{"status"}},
		{"order_customer_phone", "orders", []string{"customer_phone"}},
		{"order_customer_id", "orders", []string{"customer_id"}},
		{"order_created_at", "orders", []string{"created_at"}},
		{"order_status_created_at", "orders", []string{"status", "created_at"}},
		{"ordertrackingtoken_order_number", "order_tracking_tokens", []string{"order_number"}},
		{"ordertrackingtoken_expires_at", "order_tracking_tokens", []string{"expires_at"}},
		{"ordertrackingtoken_revoked_at", "order_tracking_tokens", []string{"revoked_at"}},
		{"idempotencykey_created_at", "idempotency_keys", []string{"created_at"}},
		{"product_is_available", "products", []string{"is_available"}},
		{"revokedtoken_token_hash_expires_at", "revoked_tokens", []string{"token_hash", "expires_at"}},
		{"revokedtoken_expires_at", "revoked_tokens", []string{"expires_at"}},
		{"stockmovement_type", "stock_movements", []string{"type"}},
		{"stockmovement_inventory_item_id_created_at", "stock_movements", []string{"inventory_item_id", "created_at"}},
		{"stockmovement_order_id", "stock_movements", []string{"order_id"}},
	}

	for _, idx := range indexes {
		_, err := db.NewCreateIndex().
			Table(idx.table).
			Index(idx.name).
			Column(idx.cols...).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index %s: %w", idx.name, err)
		}
	}
	return nil
}

func runBackfills(ctx context.Context, db *bun.DB) error {
	// These backfills were previously run by EnsureSchema after schema creation.
	// They remain idempotent so Bun auto-migration can safely rerun them.
	if err := verifyAddonGroupColumn(ctx, db); err != nil {
		return err
	}
	if err := verifyProductGlobalExclusionColumns(ctx, db); err != nil {
		return err
	}
	if err := backfillOrderItemSnapshotProductID(ctx, db); err != nil {
		return err
	}
	if err := ensureOrderItemIndexes(ctx, db); err != nil {
		return err
	}
	if err := backfillOrderCustomerID(ctx, db); err != nil {
		return err
	}
	return nil
}
