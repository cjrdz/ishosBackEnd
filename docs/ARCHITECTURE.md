# Architecture

This backend follows a layered Go architecture:

- `handlers`: HTTP transport layer (Chi handlers, request/response mapping)
- `services`: business logic and orchestration (transactions, side effects)
- `repository`: data access layer (Ent ORM queries/mutations)
- `middleware`: cross-cutting concerns (auth, roles, logging, recovery, rate-limit)
- `auth`: PocketBase integration for employee authentication

## Runtime Components

- **Chi HTTP server** (`cmd/api/main.go`, `internal/server/server.go`)
- **Ent ORM** with PostgreSQL for business data (via `database/sql` + `pgx` stdlib)
- **Neon pooler compatibility layer** in server bootstrap (`default_query_exec_mode=simple_protocol` auto-applied on `-pooler` hosts)
- **PocketBase embedded library** for auth and employee accounts
- **Optional PocketBase Admin UI server** (separate port, disabled by default; enabled via `ENABLE_PB_ADMIN_UI`)
- **Supabase Storage** for product image storage
- **WhatsApp service** for customer notifications

## Request Flow

1. Request enters Chi router.
2. Global middleware runs (`RequestID`, `RealIP`, `Recovery`, `Logger`, `CORS`).
3. Route-specific middleware runs (`RequireSession`, `RequireAdmin`, `OptionalAuth`, rate-limit).
4. Handler validates input and delegates to service.
5. Service executes business logic and calls repository.
6. Repository performs Ent database operations.
7. Handler returns JSON via `pkg/response`.

## Data Ownership

- PostgreSQL (Ent): categories, products, orders, order items, rate-limit entities.
- PocketBase SQLite (`pb_data`): employees, credentials, JWT token validation state.
- Supabase Storage: image binaries and paths.

## Design Principles

- Keep handlers thin and deterministic.
- Keep transactional and side-effect logic in services.
- Keep database specifics in repositories.
- Fail fast on validation errors.
- Prefer explicit middleware composition for route security.
