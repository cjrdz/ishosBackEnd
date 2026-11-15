// locations_migrate.go — Schema steps for multiple locations (sucursales, schema v3).
package database

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
)

// defaultLocations seeds an empty locations table. Existing data (orders,
// stock) is moved to the first one. Addresses and maps can be edited later in
// the admin settings; Sucursal 3 has no Maps embed yet (only a pin link).
var defaultLocations = []struct {
	slug, name, address, mapsURL, embedURL string
}{
	{
		slug:     "sucursal-1",
		name:     "Isho's Sucursal 1",
		address:  "Ahuachapán, El Salvador",
		mapsURL:  "https://maps.google.com/?q=Isho%27s+Sucursal+1,+Ahuachap%C3%A1n,+El+Salvador",
		embedURL: "https://www.google.com/maps/embed?pb=!1m18!1m12!1m3!1d6847.623198808501!2d-89.8518696249066!3d13.92355228648646!2m3!1f0!2f0!3f0!3m2!1i1024!2i768!4f13.1!3m3!1m2!1s0x8f6297001f790639%3A0xa292396c664dfae0!2sIsho%E2%80%99s%20Sucursal%201!5e1!3m2!1ses!2ssv!4v1777132497595!5m2!1ses!2ssv",
	},
	{
		slug:     "sucursal-2",
		name:     "Isho's Sucursal 2",
		address:  "Ahuachapán, El Salvador",
		mapsURL:  "https://maps.app.goo.gl/bSYLeqagKbSW3wUh8",
		embedURL: "https://www.google.com/maps/embed?pb=!1m18!1m12!1m3!1d6847.705736039818!2d-89.8483311!3d13.920766299999997!2m3!1f0!2f0!3f0!3m2!1i1024!2i768!4f13.1!3m3!1m2!1s0x8f6297004861f67d%3A0xfed9bfb450de212f!2sIsho%E2%80%99s%20sucursal%202!5e1!3m2!1ses!2ssv!4v1777132555390!5m2!1ses!2ssv",
	},
	{
		slug:    "sucursal-3",
		name:    "Isho's Sucursal 3",
		address: "Colonia El Ivu, frente a Panadería Colombiana, Ahuachapán",
		mapsURL: "https://maps.app.goo.gl/3hYiapn9ED9fi2oX7",
	},
}

// Shared contact defaults: every location starts with the same number.
const (
	defaultLocationWhatsApp = "50376454347"
	defaultLocationPhone    = "7645-4347"
)

// locationBoundTables get a NOT NULL location_id (audit_logs keeps it nullable:
// catalog changes do not belong to a location).
var locationBoundTables = []string{"orders", "inventory_items", "stock_movements"}

// ensureLocations adds the location columns, seeds the default locations when
// none exist, assigns rows without a location to the first one, and adds the
// constraints and indexes. Every statement is idempotent.
func ensureLocations(ctx context.Context, db bun.IDB) error {
	for _, table := range append(append([]string{}, locationBoundTables...), "audit_logs") {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS location_id UUID`, table)); err != nil {
			return fmt.Errorf("add %s.location_id: %w", table, err)
		}
	}

	if err := seedDefaultLocations(ctx, db); err != nil {
		return err
	}

	var firstID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM locations ORDER BY sort_order, created_at LIMIT 1`).Scan(&firstID); err != nil {
		return fmt.Errorf("find first location: %w", err)
	}

	// Rows created before locations existed belong to the original shop.
	for _, table := range []string{"orders", "inventory_items"} {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET location_id = ? WHERE location_id IS NULL`, table), firstID); err != nil {
			return fmt.Errorf("backfill %s.location_id: %w", table, err)
		}
	}
	// A movement belongs to its inventory item's location.
	if _, err := db.ExecContext(ctx, `UPDATE stock_movements sm SET location_id = ii.location_id
		FROM inventory_items ii WHERE sm.inventory_item_id = ii.id AND sm.location_id IS NULL`); err != nil {
		return fmt.Errorf("backfill stock_movements.location_id: %w", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE stock_movements SET location_id = ? WHERE location_id IS NULL`, firstID); err != nil {
		return fmt.Errorf("backfill orphan stock_movements.location_id: %w", err)
	}

	for _, table := range locationBoundTables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN location_id SET NOT NULL`, table)); err != nil {
			return fmt.Errorf("set %s.location_id not null: %w", table, err)
		}
	}

	// Inventory names were unique store-wide; now each location has its own
	// row per flavour/product.
	if _, err := db.ExecContext(ctx, `ALTER TABLE inventory_items DROP CONSTRAINT IF EXISTS inventory_items_name_key`); err != nil {
		return fmt.Errorf("drop inventory_items name unique: %w", err)
	}

	fks := []struct{ name, sql string }{
		{"orders_locations_location", `ALTER TABLE orders ADD CONSTRAINT orders_locations_location FOREIGN KEY (location_id) REFERENCES locations(id)`},
		{"inventory_items_locations_location", `ALTER TABLE inventory_items ADD CONSTRAINT inventory_items_locations_location FOREIGN KEY (location_id) REFERENCES locations(id)`},
		{"stock_movements_locations_location", `ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_locations_location FOREIGN KEY (location_id) REFERENCES locations(id)`},
		{"audit_logs_locations_location", `ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_locations_location FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE SET NULL`},
		{"location_availability_locations_location", `ALTER TABLE location_availability ADD CONSTRAINT location_availability_locations_location FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE CASCADE`},
	}
	for _, fk := range fks {
		exists, err := constraintExists(ctx, db, fk.name)
		if err != nil {
			return fmt.Errorf("check FK %s: %w", fk.name, err)
		}
		if exists {
			continue
		}
		if _, err := db.ExecContext(ctx, fk.sql); err != nil {
			return fmt.Errorf("add FK %s: %w", fk.name, err)
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS order_location_created_at ON orders (location_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS order_location_status ON orders (location_id, status)`,
		`CREATE INDEX IF NOT EXISTS stockmovement_location_created_at ON stock_movements (location_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS auditlog_location_created_at ON audit_logs (location_id, created_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS inventoryitem_location_flavor ON inventory_items (location_id, flavor_id) WHERE flavor_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS inventoryitem_location_product ON inventory_items (location_id, product_id) WHERE product_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS inventoryitem_location_name ON inventory_items (location_id, name)`,
	}
	for _, stmt := range indexes {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create location index: %w", err)
		}
	}
	return nil
}

func seedDefaultLocations(ctx context.Context, db bun.IDB) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM locations`).Scan(&count); err != nil {
		return fmt.Errorf("count locations: %w", err)
	}
	if count > 0 {
		return nil
	}
	for i, l := range defaultLocations {
		if _, err := db.ExecContext(ctx, `INSERT INTO locations
			(id, slug, name, address, maps_url, maps_embed_url, whatsapp, phone, hours, orders_enabled, online_orders_enabled, is_active, sort_order, created_at, updated_at)
			VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, '{}'::jsonb, true, false, true, ?, NOW(), NOW())`,
			l.slug, l.name, l.address, l.mapsURL, l.embedURL, defaultLocationWhatsApp, defaultLocationPhone, i+1); err != nil {
			return fmt.Errorf("seed location %s: %w", l.slug, err)
		}
	}
	log.Info().Int("locations", len(defaultLocations)).Msg("seeded default locations")
	return nil
}

// migrateSingleStoreToLocations runs once (schema v3) on a database that was
// used as a single shop:
//   - the global hours and orders switch are copied to every location;
//   - every tracked flavour/product gets a zero-stock row at the other
//     locations, so their stock is tracked from day one.
//
// Online ordering starts off everywhere (staff-only orders); the old global
// keys stay in admin_settings, unused, for one release.
func migrateSingleStoreToLocations(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, `UPDATE locations SET hours = s.value->'days'
		FROM admin_settings s WHERE s.key = 'store.hours' AND jsonb_typeof(s.value->'days') = 'object'`); err != nil {
		return fmt.Errorf("copy store hours to locations: %w", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE locations SET orders_enabled = (s.value->>'enabled')::boolean
		FROM admin_settings s WHERE s.key = 'store.orders.enabled' AND jsonb_typeof(s.value->'enabled') = 'boolean'`); err != nil {
		return fmt.Errorf("copy orders switch to locations: %w", err)
	}

	res, err := db.ExecContext(ctx, `INSERT INTO inventory_items
		(id, location_id, name, type, current_stock, low_stock_threshold, flavor_id, product_id, created_at, updated_at)
		SELECT gen_random_uuid(), l.id, ii.name, ii.type, 0, ii.low_stock_threshold, ii.flavor_id, ii.product_id, NOW(), NOW()
		FROM inventory_items ii
		CROSS JOIN locations l
		WHERE ii.location_id = (SELECT id FROM locations ORDER BY sort_order, created_at LIMIT 1)
		  AND l.id <> ii.location_id
		  AND (ii.flavor_id IS NOT NULL OR ii.product_id IS NOT NULL)
		  AND NOT EXISTS (
			SELECT 1 FROM inventory_items x
			WHERE x.location_id = l.id AND (x.name = ii.name
				OR x.flavor_id = ii.flavor_id OR x.product_id = ii.product_id))`)
	if err != nil {
		return fmt.Errorf("create inventory rows for new locations: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Info().Int64("items", n).Msg("created zero-stock inventory rows for the new locations")
	}
	return nil
}
