// bun_migrate.go — Bun-based schema creation and post-migration backfills.
// This mirrors the behavior of the old schema creation + raw-SQL backfills.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// schemaVersion MUST be bumped whenever a step is added to migrateSchema (or an
// existing step changes). A database whose recorded version is already
// >= schemaVersion skips migration entirely, so a new step added without a bump
// never runs on existing databases.
const schemaVersion = 1

// Migration timeouts. lock_timeout makes a DDL statement that is blocked by a
// live transaction (e.g. the previous instance during a rolling deploy) fail
// fast instead of queueing every order write behind it; the boot then fails
// and is retried, which is safer than hanging.
const (
	migrationDeadline         = 3 * time.Minute
	migrationLockTimeout      = "15s"
	migrationStatementTimeout = "120s"
)

// schemaMigrationLockName keys the transaction-scoped advisory lock that
// serialises concurrent migrators (e.g. two replicas booting at once).
const schemaMigrationLockName = "ishos:schema-migration"

// EnsureBunSchema creates tables, foreign keys and indexes if they do not
// exist, then runs idempotent backfills. It is safe to run against an
// already-migrated database.
//
// Every step runs inside a single transaction guarded by an advisory lock, and
// the schema_version marker is written in that same transaction. A crash or
// timeout part-way through therefore rolls back cleanly instead of leaving a
// half-built schema that later boots would mistake for a complete one.
//
// On a warm database the version check short-circuits with a single query, so
// server restarts do not pay a Neon round-trip per DDL statement.
func EnsureBunSchema(ctx context.Context, db *bun.DB) error {
	current, err := readSchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current >= schemaVersion {
		log.Debug().Int("version", current).Msg("schema up to date; skipping migration")
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, migrationDeadline)
	defer cancel()

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SELECT set_config('lock_timeout', ?, true), set_config('statement_timeout', ?, true)",
			migrationLockTimeout, migrationStatementTimeout); err != nil {
			return fmt.Errorf("set migration timeouts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", AdvisoryLockKey(schemaMigrationLockName)); err != nil {
			return fmt.Errorf("acquire schema migration lock: %w", err)
		}

		// Another migrator may have finished while we waited for the lock.
		current, err := readSchemaVersion(ctx, tx)
		if err != nil {
			return fmt.Errorf("re-read schema version: %w", err)
		}
		if current >= schemaVersion {
			return nil
		}

		log.Info().Int("from", current).Int("to", schemaVersion).Msg("migrating database schema")
		if err := migrateSchema(ctx, tx); err != nil {
			return err
		}
		return writeSchemaVersion(ctx, tx, schemaVersion)
	})
}

// migrateSchema runs every idempotent schema step. Each step must be safe to
// re-run against a database that already has it applied, and must not raise an
// error for already-present objects: an error aborts the enclosing transaction.
func migrateSchema(ctx context.Context, db bun.IDB) error {
	if err := createTables(ctx, db); err != nil {
		return fmt.Errorf("create bun tables: %w", err)
	}
	if err := createIndexes(ctx, db); err != nil {
		return fmt.Errorf("create bun indexes: %w", err)
	}
	if err := ensureOrderArchiveIndex(ctx, db); err != nil {
		return err
	}
	if err := ensureIdempotencyKeyExpiresAt(ctx, db); err != nil {
		return err
	}
	if err := ensureAmountReceivedColumn(ctx, db); err != nil {
		return err
	}
	if err := ensureFreeToppingsColumn(ctx, db); err != nil {
		return err
	}
	if err := runBackfills(ctx, db); err != nil {
		return fmt.Errorf("run backfills: %w", err)
	}
	return nil
}

// readSchemaVersion returns the recorded schema version, or 0 when the
// schema_version table does not exist yet (fresh database).
func readSchemaVersion(ctx context.Context, db bun.IDB) (int, error) {
	const existsQuery = `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
	  AND table_name = 'schema_version'
)`
	var exists bool
	if err := db.QueryRowContext(ctx, existsQuery).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}

	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func writeSchemaVersion(ctx context.Context, db bun.IDB, version int) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_version (
	version    INTEGER PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?) ON CONFLICT (version) DO NOTHING`, version); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	return nil
}

func createTables(ctx context.Context, db bun.IDB) error {
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

func createForeignKeys(ctx context.Context, db bun.IDB) error {
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
		// Check for the constraint up front rather than swallowing "already
		// exists" errors: inside a transaction any error aborts the whole tx.
		exists, err := constraintExists(ctx, db, fk.name)
		if err != nil {
			return fmt.Errorf("check FK %s: %w", fk.name, err)
		}
		if exists {
			continue
		}
		if _, err := db.ExecContext(ctx, fk.sql+` ON UPDATE NO ACTION NOT VALID`); err != nil {
			return fmt.Errorf("add FK %s: %w", fk.name, err)
		}
	}
	return nil
}

// constraintExists reports whether a constraint with the given name exists in
// the current schema.
func constraintExists(ctx context.Context, db bun.IDB, name string) (bool, error) {
	const query = `
SELECT EXISTS (
	SELECT 1
	FROM pg_constraint c
	JOIN pg_namespace n ON n.oid = c.connamespace
	WHERE n.nspname = current_schema()
	  AND c.conname = ?
)`
	var exists bool
	if err := db.QueryRowContext(ctx, query, name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func createIndexes(ctx context.Context, db bun.IDB) error {
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

func ensureOrderArchiveIndex(ctx context.Context, db bun.IDB) error {
	_, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_orders_is_archived_created_at ON orders (is_archived, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("create order archive index: %w", err)
	}
	return nil
}

func ensureAmountReceivedColumn(ctx context.Context, db bun.IDB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE orders ADD COLUMN IF NOT EXISTS amount_received DOUBLE PRECISION`)
	if err != nil {
		return fmt.Errorf("add orders.amount_received column: %w", err)
	}
	return nil
}

func ensureFreeToppingsColumn(ctx context.Context, db bun.IDB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE products ADD COLUMN IF NOT EXISTS free_toppings INTEGER NOT NULL DEFAULT 1`)
	if err != nil {
		return fmt.Errorf("add products.free_toppings column: %w", err)
	}
	_, err = db.ExecContext(ctx, `UPDATE products SET free_toppings = 1 WHERE free_toppings IS NULL OR free_toppings < 0`)
	if err != nil {
		return fmt.Errorf("backfill products.free_toppings: %w", err)
	}
	return nil
}

func ensureIdempotencyKeyExpiresAt(ctx context.Context, db bun.IDB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ`)
	if err != nil {
		return fmt.Errorf("add idempotency_keys.expires_at column: %w", err)
	}
	_, err = db.ExecContext(ctx, `UPDATE idempotency_keys SET expires_at = created_at + INTERVAL '30 days' WHERE expires_at IS NULL`)
	if err != nil {
		return fmt.Errorf("backfill idempotency_keys.expires_at: %w", err)
	}
	_, err = db.ExecContext(ctx, `ALTER TABLE idempotency_keys ALTER COLUMN expires_at SET NOT NULL`)
	if err != nil {
		return fmt.Errorf("set idempotency_keys.expires_at not null: %w", err)
	}
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idempotencykey_expires_at ON idempotency_keys (expires_at)`)
	if err != nil {
		return fmt.Errorf("create idempotency_keys expires_at index: %w", err)
	}
	return nil
}

func runBackfills(ctx context.Context, db bun.IDB) error {
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
