CREATE TABLE IF NOT EXISTS order_tracking_tokens (
    token_hash TEXT PRIMARY KEY,
    order_number TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_order_tracking_tokens_order_number
    ON order_tracking_tokens (order_number);

CREATE INDEX IF NOT EXISTS idx_order_tracking_tokens_expires_at
    ON order_tracking_tokens (expires_at);

CREATE INDEX IF NOT EXISTS idx_order_tracking_tokens_revoked_at
    ON order_tracking_tokens (revoked_at);