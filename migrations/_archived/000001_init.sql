-- 000001_init.sql
-- Reference migration for IshosFactory backend.
-- Bun auto-migrates on startup, so this file is for documentation
-- and manual deployment scenarios only.
--
-- Tables: categories, products, orders, order_items, rate_limits

-- Enable UUID extension (Neon has this enabled by default)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Categories ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS categories (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(100) NOT NULL UNIQUE,
    slug         VARCHAR(100) NOT NULL UNIQUE,
    description  TEXT DEFAULT '',
    image_path   VARCHAR(500),
    display_order INTEGER NOT NULL DEFAULT 0,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories (slug);
CREATE INDEX IF NOT EXISTS idx_categories_is_active ON categories (is_active);

-- ─── Products ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS products (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(200) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    price        DOUBLE PRECISION NOT NULL CHECK (price > 0),
    image_path   VARCHAR(500),
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    stock_status VARCHAR(20) NOT NULL DEFAULT 'in_stock'
                 CHECK (stock_status IN ('in_stock', 'out_of_stock', 'discontinued')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Foreign key to categories
    category_products UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_products_availability ON products (is_available, stock_status);
CREATE INDEX IF NOT EXISTS idx_products_category ON products (category_products);

-- ─── Orders ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS orders (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_number        VARCHAR(255) NOT NULL UNIQUE,
    customer_name       VARCHAR(200) NOT NULL,
    customer_phone      VARCHAR(20) NOT NULL,
    customer_email      VARCHAR(255),
    customer_ip         VARCHAR(45),
    total_amount        DOUBLE PRECISION NOT NULL,
    status              VARCHAR(30) NOT NULL DEFAULT 'pendiente_revision'
                        CHECK (status IN (
                            'pendiente_revision',
                            'recibida',
                            'en_proceso',
                            'lista',
                            'entregada',
                            'cancelada'
                        )),
    payment_method      VARCHAR(20) NOT NULL DEFAULT 'efectivo'
                        CHECK (payment_method IN ('efectivo', 'tarjeta', 'transferencia', 'otro')),
    payment_status      VARCHAR(20) NOT NULL DEFAULT 'pendiente'
                        CHECK (payment_status IN ('pendiente', 'confirmado')),
    order_type          VARCHAR(20) NOT NULL DEFAULT 'para_llevar'
                        CHECK (order_type IN ('en_local', 'para_llevar')),
    table_number        INTEGER CHECK (table_number > 0),
    notes               TEXT,
    rejection_reason    TEXT,
    reviewed_by_ory_id  VARCHAR(255),  -- Historical name; now stores PocketBase employee record ID
    reviewed_at         TIMESTAMPTZ,
    created_by_ory_id   VARCHAR(255),  -- Historical name; now stores PocketBase employee record ID
    is_archived         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_customer_phone ON orders (customer_phone);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders (created_at);

-- ─── Order Items ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS order_items (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_name    VARCHAR(200) NOT NULL,
    quantity        INTEGER NOT NULL CHECK (quantity > 0),
    unit_price      DOUBLE PRECISION NOT NULL,
    subtotal        DOUBLE PRECISION NOT NULL,
    customizations  JSONB,

    -- Foreign key to orders
    order_items     UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    -- Foreign key to products
    product_order_items UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items (order_items);
CREATE INDEX IF NOT EXISTS idx_order_items_product ON order_items (product_order_items);

-- ─── Rate Limits ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS rate_limits (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ip_address    VARCHAR(45) NOT NULL,
    phone_number  VARCHAR(20),
    action_type   VARCHAR(50) NOT NULL DEFAULT 'order_creation',
    attempts      INTEGER NOT NULL DEFAULT 1,
    last_attempt  TIMESTAMPTZ NOT NULL DEFAULT now(),
    blocked_until TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_rate_limits_ip_action ON rate_limits (ip_address, action_type);
CREATE INDEX IF NOT EXISTS idx_rate_limits_phone_action ON rate_limits (phone_number, action_type);
