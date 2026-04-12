# Architecture

This backend follows a layered Go architecture:

- `handlers`: HTTP transport layer (Chi handlers, request/response mapping)
- `services`: business logic and orchestration (transactions, side effects)
- `repository`: data access layer (Ent ORM queries/mutations)
- `middleware`: cross-cutting concerns (auth, roles, logging, recovery, rate-limit, security headers)
- `auth`: PocketBase integration for employee authentication

## Runtime Components

- **Chi HTTP server** (`cmd/api/main.go`, `internal/server/server.go`)
- **Ent ORM** with PostgreSQL for business data (via `database/sql` + `pgx` stdlib)
  - Neon direct endpoint recommended — avoid `-pooler` hosts for stable query execution
- **PocketBase embedded library** for employee auth and JWT token validation
  - Optional PocketBase Admin UI server (separate port, disabled by default; enabled via `ENABLE_PB_ADMIN_UI`)
- **Cloudflare R2** for product image storage (AWS SDK v2, S3-compatible)
- **WhatsApp Business API** (Meta Graph API) for customer order notifications
- **Redis** (optional) for distributed rate limiting; falls back to in-memory sliding-window counter when `REDIS_URL` is unset
- **PostgreSQL `auth_revoked_tokens` table** for server-side JWT revocation (initialized at startup via `auth.InitTokenRevocationStore`)
- **PostgreSQL `order_tracking_tokens` table** for high-entropy tracking token storage (HMAC-hashed, verified by `order_number + token`)

## Request Flow

1. Request enters Chi router.
2. Global middleware runs: `RequestID`, `RealIP`, `SecurityHeaders`, `Logger`, `Recovery`, `CORS`, `Timeout`.
3. Route-specific middleware runs: `RequireSession` / `OptionalSession`, rate-limit, role checks where applicable.
4. Handler validates input with `internal/validator` and delegates to service layer.
5. Service executes business logic and calls repository or external services (R2, WhatsApp).
6. Repository performs Ent database operations.
7. Handler returns JSON via `pkg/response`.

## Data Ownership

| Store | Data |
| --- | --- |
| PostgreSQL via Ent | Categories, Products (with Flavor/Addon edges), Orders, OrderItems, Flavors, Addons, RateLimits, AdminSettings, Users, `auth_revoked_tokens`, `order_tracking_tokens` |
| PocketBase SQLite (`pb_data`) | Employees, password hashes, JWT token validation state |
| Cloudflare R2 | Image binaries; object keys stored in PostgreSQL rows |

## Ent Schema Entities

| Entity | Description |
| --- | --- |
| `Category` | Menu top-level grouping |
| `Product` | Menu item; edges to Flavor (M2M) and Addon (M2M) |
| `Flavor` | Available gelato flavors linkable to products |
| `Addon` | Toppings/extras with `group_name`; linkable to products |
| `Order` | Customer order (stores customer contact as fields, not FK) |
| `OrderItem` | Per-order product snapshot (name, price, qty, customizations) |
| `RateLimit` | Sliding-window rate limit record per IP or phone |
| `AdminSetting` | Persisted panel configuration (tab order, token TTL overrides) |

> **Note on Customers:** There is no persistent `Customer` entity. Customer contact data (name, phone, email, IP) is stored directly on the `Order` row. The `User` entity (available via `/api/v1/users`) represents a separately managed customer directory record.

## Order Tracking Token Design

Public order tracking does not use phone-based or stateless HMAC payloads. Tokens are:

1. Random high-entropy values generated at order creation.
2. Stored as HMAC-signed material only in the `order_tracking_tokens` table.
3. Verified by presenting `order_number + token` together.
4. Per-order revocable by admin.
5. Issued through `OrderService.IssueTrackingAccess`; consumed via the WhatsApp notification link or the storefront tracking page.

## Security Baseline

- Global `SecurityHeaders` middleware applies: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`.
- JSON request bodies are capped at **1 MiB** by the shared `decodeJSON` helper.
- Analytics and export routes are **admin-only** via the `RequireAdmin` role middleware.
- Failed login attempts are IP-rate-limited and subject to temporary lockout.
- Server-side token revocation is stored in PostgreSQL so revoked tokens are rejected before JWT expiry.
- Order customer IP capture uses normalized `RemoteAddr` parsing, not raw `X-Forwarded-For`.

## Design Principles

- Keep handlers thin and deterministic.
- Keep transactional and side-effect logic in services.
- Keep database specifics in repositories.
- Fail fast on validation errors.
- Prefer explicit middleware composition for route security.
