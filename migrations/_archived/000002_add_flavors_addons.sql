-- 000002_add_flavors_addons.sql
-- Add Flavor and Addon global management tables.
-- Create many-to-many relationships with products.
-- Remove stock_status column from products and create is_available.

-- ─── Flavors ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS flavors (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(100) NOT NULL UNIQUE,
    display_order INTEGER NOT NULL DEFAULT 0,
    is_seasonal  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_flavors_name ON flavors (name);
CREATE INDEX IF NOT EXISTS idx_flavors_is_active ON flavors (is_active);
CREATE INDEX IF NOT EXISTS idx_flavors_display_order ON flavors (display_order);

-- ─── Addons ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS addons (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(100) NOT NULL UNIQUE,
    price        DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (price >= 0),
    display_order INTEGER NOT NULL DEFAULT 0,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addons_name ON addons (name);
CREATE INDEX IF NOT EXISTS idx_addons_is_active ON addons (is_active);
CREATE INDEX IF NOT EXISTS idx_addons_display_order ON addons (display_order);

-- ─── Product-Flavor Junction Table ───────────────────────────────────────

CREATE TABLE IF NOT EXISTS product_flavors (
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    flavor_id  UUID NOT NULL REFERENCES flavors(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, flavor_id)
);

CREATE INDEX IF NOT EXISTS idx_product_flavors_flavor_id ON product_flavors (flavor_id);

-- ─── Product-Addon Junction Table ────────────────────────────────────────

CREATE TABLE IF NOT EXISTS product_addons (
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    addon_id   UUID NOT NULL REFERENCES addons(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, addon_id)
);

CREATE INDEX IF NOT EXISTS idx_product_addons_addon_id ON product_addons (addon_id);

-- ─── Update Products Table ───────────────────────────────────────────────

-- Remove the old index if it exists
DROP INDEX IF EXISTS idx_products_availability;

-- Drop stock_status column and ensure is_available is set
ALTER TABLE products 
DROP COLUMN IF EXISTS stock_status;

-- Ensure is_available column exists (it might already exist)
ALTER TABLE products 
ADD COLUMN IF NOT EXISTS is_available BOOLEAN NOT NULL DEFAULT TRUE;

-- Create new index
CREATE INDEX IF NOT EXISTS idx_products_availability ON products (is_available);

-- ─── Seed Initial Flavor ─────────────────────────────────────────────────

-- Insert "Guineo" flavor (optional, can be toggled later)
INSERT INTO flavors (name, display_order, is_seasonal, is_active)
VALUES ('Guineo', 0, FALSE, TRUE)
ON CONFLICT (name) DO NOTHING;
