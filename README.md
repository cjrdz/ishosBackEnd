# IshosFactory Backend

Go + Chi + Ent + PocketBase backend for IshosFactory. The service owns catalog, orders, employee auth, customer directory records, admin settings, analytics, exports, and media uploads.

## Architecture

The codebase still uses layered packages, but the boundaries are explicit:

- Transport: `internal/handlers` and `internal/server`
- Business logic: `internal/services`
- Data access: `internal/repository`
- Validation: `internal/validator`
- Cross-cutting middleware: `internal/middleware`
- Auth integration: `internal/auth`

Request lifecycle:

1. The request enters the Chi router in [internal/server/server.go](internal/server/server.go).
2. Global middleware applies security headers, CORS, logging, recovery, and request metadata.
3. Route middleware enforces session, role, timeout, and rate-limit policy.
4. Handlers validate payloads and delegate to services.
5. Services orchestrate business rules, repositories, and side effects.
6. Repositories persist through Ent and PostgreSQL.

## Runtime components

| Component | Purpose |
| --- | --- |
| PostgreSQL + Ent | Business data: categories, products, orders, customers, admin settings, tracking tokens, revoked tokens |
| Embedded PocketBase | Employee accounts, password hashing, JWT issuance and validation |
| Cloudflare R2 | Product and gallery image storage |
| Redis optional | Distributed rate-limit and cache support when configured |
| WhatsApp integration optional | Customer notifications for order lifecycle events |

## Data ownership

There are three important identity concepts in this backend:

- Employees: stored in PocketBase and used for staff/admin authentication.
- Customers: stored in PostgreSQL as the `Customer` Ent entity, exposed by `/api/v1/users` for the admin customer directory.
- Order contact snapshot: each order still stores the customer name, phone, email, and IP needed for the transaction itself.

That means `/api/v1/users` is a customer-directory API backed by the `Customer` entity, not a PocketBase auth collection.

## Route surface

All business endpoints live under `/api/v1`.

### Public routes

- `POST /auth/login`
- `POST /auth/logout`
- `GET /auth/session` using optional-session middleware
- `GET /products`, `GET /products/{id}`
- `GET /categories`, `GET /categories/{slug}`
- `GET /flavors`, `GET /flavors/{id}`
- `GET /addons`, `GET /addons/{id}`
- `POST /orders` with optional staff session capture
- `GET /orders/track`
- `GET /settings/store/public`

### Protected routes

- Settings: `/settings/tabs`, `/settings/panel-config`, `/settings/store`
- Employees: `/employees/*`
- Customer directory: `/users/*`
- Catalog management: `/categories/*`, `/products/*`, `/flavors/*`, `/addons/*`
- Orders: `/orders/*`
- Admin-only analytics: `/analytics/*`
- Admin-only export: `/export/orders`
- Uploads: `/upload/image`, `/upload/images`
- Admin lockout reset: `POST /auth/lockout/reset`

See [docs/API.md](docs/API.md) for the canonical endpoint table.

## Security baseline

- Global `SecurityHeaders` middleware applies `nosniff`, `DENY` frame policy, strict referrer policy, and a restrictive CSP.
- JSON decoding is capped at 1 MiB.
- Analytics and export routes are admin-only.
- Login attempts are rate-limited and can trigger temporary lockout.
- Logout revokes the presented token server-side.
- Public order tracking uses order-number plus high-entropy tracking token verification.
- Rate limiting can use Redis or in-memory fallback.

## Quick start

### Prerequisites

- Go 1.23+
- PostgreSQL database
- Cloudflare R2 bucket credentials
- Optional WhatsApp credentials

### Local setup

```sh
cp .env.example .env
go mod tidy
go generate ./ent
make migrate
go run ./cmd/api
```

### Bootstrap the first admin

```sh
bash ./scripts/bootstrap_admin.sh
# or
make bootstrap-admin
# or
go run ./cmd/bootstrap -email admin@yourdomain.com -password "StrongPassword123!" -name "Admin"
```

### Health checks

```sh
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/products
curl http://localhost:8080/api/v1/categories
```

## Project layout

```text
cmd/                  # Entrypoints and utilities
docs/                 # API, architecture, development, deployment, PocketBase docs
ent/                  # Ent schemas and generated code
internal/
├── auth/             # PocketBase wrapper and auth helpers
├── handlers/         # HTTP handlers
├── middleware/       # Session, role, rate-limit, logging, recovery, headers
├── repository/       # Ent-backed persistence
├── server/           # Router wiring and process lifecycle
├── services/         # Domain logic and side effects
└── validator/        # Validation rules
pkg/response/         # JSON response helpers
scripts/              # Bootstrap, seed, backup, test, deploy helpers
tests/                # Integration scaffolding and fixtures
pb_data/              # PocketBase SQLite state, must be persistent in prod
```

## Documentation

- [docs/API.md](docs/API.md): endpoint reference
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): runtime and layering details
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): local workflow and coding rules
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md): environment and release guidance
- [docs/POCKETBASE.md](docs/POCKETBASE.md): employee auth and operational notes

## License

Private - IshosFactory
