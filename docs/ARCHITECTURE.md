# Architecture

The backend is a Chi HTTP service with explicit transport, service, repository, and middleware layers.

## Package roles

- `internal/server`: router wiring, infrastructure bootstrapping, process lifecycle
- `internal/handlers`: HTTP transport and request/response mapping
- `internal/services`: domain logic and orchestration
- `internal/repository`: Ent-backed persistence only
- `internal/middleware`: auth, role checks, rate limit, logging, recovery, security headers
- `internal/auth`: embedded PocketBase integration for employee auth
- `internal/validator`: request validation rules

## Runtime components

| Component | Responsibility |
| --- | --- |
| Chi | HTTP routing and middleware composition |
| PostgreSQL + Ent | Core business persistence |
| PocketBase embedded | Employee auth, password hashing, JWT issuance and validation |
| Cloudflare R2 | Media storage |
| Redis optional | Shared rate limiting and cache support |
| WhatsApp optional | Outbound customer notifications |

## Request flow

1. A request enters [internal/server/server.go](../internal/server/server.go).
2. Global middleware applies headers, logging, recovery, CORS, timeout, and request metadata.
3. Route-level middleware applies `OptionalSession`, `RequireSession`, role gates, and rate limits.
4. Handlers validate input and translate transport payloads.
5. Services enforce business rules and orchestrate repositories or side effects.
6. Repositories perform Ent reads and writes.
7. Responses are returned with `pkg/response` helpers.

## Route partitioning

The router is grouped into three main surfaces:

- Public routes: auth login/logout/session, public catalog, public order creation, public tracking, public store settings.
- Protected staff routes: settings, orders, catalog management, uploads, customer directory.
- Admin-only protected routes: analytics, export, employee management, lockout reset, destructive settings changes.

The settings subtree is intentionally isolated under `/api/v1/settings` so public store settings and protected admin settings can coexist without ambiguous router prefixes.

## Data ownership

| Store | Data |
| --- | --- |
| PostgreSQL via Ent | Categories, products, flavors, addons, orders, order items, customers, admin settings, rate limits, revoked tokens, tracking tokens |
| PocketBase SQLite | Employees and auth state |
| Cloudflare R2 | Image binaries |

Important distinction:

- `employees` are PocketBase auth records.
- `/api/v1/users` is the customer-directory API backed by the Ent `Customer` entity.
- orders still store customer contact snapshots needed for the actual transaction lifecycle.

## Tracking token model

Public tracking access uses `order_number + tracking_token`.

- Tokens are random high-entropy values.
- Only hashed token material is stored server-side.
- Tokens are issued through the order service.
- Tokens can be revoked per order.

## Security baseline

- `SecurityHeaders` middleware is global.
- JSON body parsing is capped at 1 MiB.
- analytics and export endpoints are admin-only.
- failed logins are rate-limited and lockout-aware.
- logout revokes the presented token server-side.
- request IP normalization does not trust raw forwarded headers in handlers.

## Design principles

- Keep handlers thin.
- Keep business rules in services.
- Keep repositories side-effect free.
- Prefer explicit route middleware over implicit behavior.
- Treat the docs and route wiring as part of the production contract.
