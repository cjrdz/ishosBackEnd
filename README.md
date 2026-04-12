# IshosFactory Backend

Go + Chi + PocketBase + Ent backend for the IshosFactory gelato shop, integrating **Neon PostgreSQL**, **Cloudflare R2**, **PocketBase** authentication, and **WhatsApp Business API** notifications.

## Architecture

The backend is organized in clear layers to keep HTTP, business logic, and data access separated:

- **Transport layer**: `internal/handlers` + `internal/server` (routing, HTTP contracts)
- **Middleware layer**: `internal/middleware` (auth, optional auth, rate-limit, logger, recovery, roles)
- **Validation layer**: `internal/validator` (request validation rules)
- **Business layer**: `internal/services` (order workflow, upload orchestration, WhatsApp side effects)
- **Data layer**: `internal/repository` (Ent queries/mutations only)
- **Persistence/integration**:
  - Neon PostgreSQL via Ent ORM for business data
  - PocketBase embedded library for employee auth and JWT validation
  - Cloudflare R2 for image objects

### Request Lifecycle

1. Request enters Chi router (`internal/server/server.go`).
2. Global middleware runs: request metadata, security headers, recovery, logger, CORS.
3. Route middleware runs: `RequireSession` / `OptionalAuth`, roles, rate limit.
4. Handler decodes + validates payload with `internal/validator`.
5. Service executes business rules and side effects.
6. Repository performs database operations through Ent.
7. Response is returned via `pkg/response` helpers.

## How the Services Connect

| Service              | What it stores                                       | How Go connects                              |
| -------------------- | ---------------------------------------------------- | -------------------------------------------- |
| **Neon PostgreSQL**  | Categories, Products, Orders, OrderItems, RateLimits | Ent ORM via `database/sql` + `pgx` stdlib driver (pooler-safe simple protocol auto-applied) |
| **Cloudflare R2**    | Product images (JPEG, PNG, WebP)                     | AWS SDK v2 S3 client with R2 credentials     |
| **PocketBase**       | Admin & employee accounts, JWT tokens                | Embedded Go library via `PBAuth` wrapper     |

## Auth Flow

1. **Login**: Frontend sends `POST /api/v1/auth/login` with `{ "email": "...", "password": "..." }`.
2. **Token**: The Chi handler validates credentials via PocketBase's Go API and returns a JWT token + employee data.
3. **Requests**: Frontend sends `Authorization: Bearer <token>` on protected routes.
4. **Validation**: Chi middleware (`PBAuth.RequireSession`) validates the token via PocketBase's Go API (no HTTP calls) and stores the employee record and role in the request context.
5. **Role check**: Role middleware reads the role from context for admin/employee access control.

> PocketBase auth runs embedded via Go API. By default, all app requests go through Chi on a single port; an optional PocketBase Admin UI server can be enabled for development via `ENABLE_PB_ADMIN_UI=true`.

## Prerequisites

- [Go 1.23+](https://go.dev/dl/)
- [Neon](https://neon.tech) PostgreSQL database
- [Cloudflare R2](https://developers.cloudflare.com/r2/) bucket with S3 API credentials
- (Optional) [WhatsApp Business API](https://developers.facebook.com/docs/whatsapp) credentials

## Quick Start

### 1. Clone and configure

```sh
git clone <repo-url>
cd ishosBackend
cp .env.example .env
# Edit .env with your actual credentials
```

### 2. Install dependencies and generate Ent code

```sh
go mod tidy
go generate ./ent
```

If you need to migrate the database without starting the API server:

```sh
make migrate
```

### 3. Run locally

```sh
# Option A: Direct (requires Neon DB URL in .env)
go run ./cmd/api

# Option B: Docker Compose (local Postgres for development)
docker compose up --build
```

API startup also applies Ent schema migrations automatically and verifies that `addons.group_name` exists before serving requests.

### 4. Create the first admin employee

On first launch, the `employees` auth collection is created automatically in PocketBase's embedded SQLite with the following custom fields:
- `name` (text)
- `phone` (text)
- `role` (select: `admin` | `employee`)

Create the first admin with one of these options:

```sh
# Option A: helper script (supports env file or interactive prompt)
bash ./scripts/bootstrap_admin.sh

# Option B: Make target
make bootstrap-admin

# Option C: direct Go command
go run ./cmd/bootstrap -email admin@yourdomain.com -password "StrongPassword123!" -name "Admin"
```

If you want to avoid typing credentials every time, add these variables to your `.env` file:

```sh
BOOTSTRAP_ADMIN_EMAIL=admin@yourdomain.com
BOOTSTRAP_ADMIN_PASSWORD=StrongPassword123!
BOOTSTRAP_ADMIN_NAME=Admin
BOOTSTRAP_ADMIN_PHONE=+50688881234
```

When these values are missing in `.env`, the script prompts for them in the terminal.

### 5. Login and get a token

```sh
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@yourdomain.com",
    "password": "securePassword123"
  }'
```

Response:

```json
{
  "token": "<your-jwt-token>",
  "employee": {
    "id": "pb-record-id",
    "email": "admin@yourdomain.com",
    "name": "Admin",
    "phone": "+50688881234",
    "role": "admin"
  }
}
```

Use the returned `token` in subsequent requests:

```sh
curl http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer <TOKEN>"
```

### 6. Verify public endpoints

```sh
curl http://localhost:8080/api/v1/products
curl http://localhost:8080/api/v1/categories
curl http://localhost:8080/health
```

## Project Structure

```
ishosBackend/
├── cmd/
│   ├── api/
│   │   └── main.go             # Entrypoint: Chi HTTP server + graceful shutdown
│   ├── bootstrap/
│   │   └── main.go             # Create first admin employee
│   ├── migrate/
│   │   └── main.go             # Apply schema migrations and verify addon grouping column
│   ├── seed/
│   │   └── main.go             # Seed categories/products from JSON
├── ent/
│   ├── schema/
│   │   ├── category.go         # Category entity
│   │   ├── product.go          # Product entity (edges: flavor, addon)
│   │   ├── order.go            # Order entity
│   │   ├── orderitem.go        # OrderItem entity
│   │   ├── flavor.go           # Flavor entity
│   │   ├── addon.go            # Addon entity
│   │   ├── adminsetting.go     # AdminSetting entity (panel config, tab order)
│   │   └── ratelimit.go        # RateLimit entity
│   └── generate.go             # go:generate directive
├── internal/
│   ├── auth/
│   │   └── pocketbase.go       # PBAuth wrapper (login, tokens, employee CRUD)
│   ├── config/
│   │   └── config.go           # Env var loading
│   ├── handlers/
│   │   ├── auth.go             # Login, session, logout
│   │   ├── categories.go       # Category CRUD
│   │   ├── employees.go        # Employee management
│   │   ├── flavors.go          # Flavor CRUD
│   │   ├── addons.go           # Addon CRUD
│   │   ├── helpers.go          # Shared handler utilities
│   │   ├── orders.go           # Order lifecycle
│   │   ├── products.go         # Product CRUD + flavor/addon linking
│   │   ├── users.go            # User (customer directory) CRUD
│   │   ├── analytics.go        # Analytics endpoints
│   │   ├── export.go           # Order export and purge
│   │   ├── admin_settings.go   # Admin panel settings
│   │   └── upload.go           # R2 image upload/delete
│   ├── middleware/
│   │   ├── auth.go             # PBAuth session validation (Chi middleware)
│   │   ├── optional_auth.go    # Optional auth (public + staff mixed routes)
│   │   ├── logger.go           # Request logging middleware
│   │   ├── recovery.go         # Panic recovery middleware
│   │   ├── security_headers.go # Security response headers middleware
│   │   ├── ratelimit.go        # IP/phone rate limiting (Redis + in-memory fallback)
│   │   └── roles.go            # Admin/employee role checks
│   ├── repository/
│   │   ├── category_repo.go
│   │   ├── product_repo.go
│   │   ├── order_repo.go
│   │   ├── ratelimit_repo.go
│   │   └── adminsetting_repo.go
│   ├── server/
│   │   └── server.go           # Chi router, DB wiring, route registration
│   ├── services/
│   │   ├── order_service.go
│   │   ├── product_service.go
│   │   ├── upload_service.go
│   │   ├── adminsetting_service.go
│   │   ├── tracking_token_service.go  # Tracking token mint/verify/revoke
│   │   └── whatsapp_service.go # WhatsApp Business API
│   └── validator/
│       ├── validator.go
│       └── rules.go
├── pkg/response/
│   └── response.go             # JSON response helpers
├── tests/
│   ├── integration/
│   └── fixtures/
├── scripts/
│   ├── seed_all.sh
│   ├── clean_seeds.sh
│   ├── backup_pb.sh
│   ├── deploy.sh
│   └── test.sh
├── docs/
│   ├── API.md
│   ├── ARCHITECTURE.md
│   ├── DEPLOYMENT.md
│   ├── POCKETBASE.md
│   └── DEVELOPMENT.md
├── .editorconfig
├── .golangci.yml
├── Makefile
├── pb_data/                    # PocketBase SQLite data (gitignored)
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

## Documentation

- `docs/API.md` — full API reference
- `docs/ARCHITECTURE.md` — system architecture and layer responsibilities
- `docs/DEPLOYMENT.md` — deployment checklist and platform notes
- `docs/POCKETBASE.md` — embedded PocketBase auth model and operations
- `docs/DEVELOPMENT.md` — local workflow, testing, and coding conventions

## Tooling

- `Makefile` targets for local dev, test, build, backup, and deploy
- `.editorconfig` for consistent editor behavior
- `.golangci.yml` for lint policy and checks
- `.github/workflows/security.yml` for automated `govulncheck`, `gosec`, `gitleaks`, and Trivy image scanning
- Integration test scaffolding under `tests/integration` (gated by `RUN_INTEGRATION=1`)

## Security Hardening (Current Baseline)

- Security response headers are applied globally:
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`
- Analytics endpoints are now **admin-only** via role middleware.
- JSON request decoding enforces a shared **1 MiB** max body size limit.
- Login rate limiting panic path was removed by making per-IP sliding window compaction nil-safe.
- Failed-login lockout is enforced per IP after configurable repeated failures in a rolling window.
- Logout now triggers server-side token revocation (denylist) so revoked tokens are rejected before JWT expiry.
- Browser auth cookie TTL is reduced to **24h** by default (`AUTH_COOKIE_TTL_HOURS`) to reduce long-lived session risk.
- Order customer IP capture now uses normalized `RemoteAddr` parsing instead of directly trusting `X-Forwarded-For` in handlers.

These controls are intentionally conservative and can be relaxed per endpoint if future requirements need broader CSP or larger JSON payloads.

## Scripts

- `scripts/bootstrap_admin.sh` — creates the first admin user; reads credentials from `.env` (`BOOTSTRAP_ADMIN_*`) or prompts interactively.
- `scripts/seed_all.sh` — seeds categories, products, flavors, and addons from JSON files into the configured database.
- `scripts/clean_seeds.sh` — removes seeded data from database tables (supports confirmation and dry-run mode).
- `scripts/test.sh` — runs Go tests with race detector and generates `coverage.html`.
- `scripts/backup_pb.sh` — creates a compressed backup of `pb_data` into `pb_data/backups`.
- `scripts/deploy.sh` — builds production binary (`bin/server`) for deployment.

## API Reference

### Authentication

| Method | Path                     | Auth    | Description                    |
| ------ | ------------------------ | ------- | ------------------------------ |
| POST   | `/api/v1/auth/login`     | Public  | Login → returns JWT token      |
| GET    | `/api/v1/auth/session`   | Bearer  | Current user session info      |
| POST   | `/api/v1/auth/logout`    | Public  | Logout helper (discard token)  |

### Categories

| Method | Path                        | Auth   | Description                |
| ------ | --------------------------- | ------ | -------------------------- |
| GET    | `/api/v1/categories`        | Public | List all active categories |
| GET    | `/api/v1/categories/{slug}` | Public | Get category by slug       |
| POST   | `/api/v1/categories`        | Admin  | Create a category          |
| PATCH  | `/api/v1/categories/{slug}` | Admin  | Update a category          |
| DELETE | `/api/v1/categories/{slug}` | Admin  | Delete a category          |

### Products

| Method | Path                                 | Auth   | Description         |
| ------ | ------------------------------------ | ------ | ------------------- |
| GET    | `/api/v1/products`                   | Public | List products       |
| GET    | `/api/v1/products?category={slug}`   | Public | Filter by category  |
| GET    | `/api/v1/products/{id}`              | Public | Get single product  |
| POST   | `/api/v1/products`                   | Admin  | Create product      |
| PATCH  | `/api/v1/products/{id}`              | Admin  | Update product      |
| DELETE | `/api/v1/products/{id}`              | Admin  | Delete product      |
| PATCH  | `/api/v1/products/{id}/availability` | Admin  | Toggle availability |

### Orders

| Method | Path                                           | Auth    | Description                                |
| ------ | ---------------------------------------------- | ------- | ------------------------------------------ |
| POST   | `/api/v1/orders`                               | Public* | Create order (* rate limited)              |
| GET    | `/api/v1/orders/track?order_number=...&tracking_token=...` | Public  | Public order tracking by number + token    |
| GET    | `/api/v1/orders`                               | Staff   | List orders (supports `status`, `page`, `limit`) |
| GET    | `/api/v1/orders/{id}`                          | Staff   | Get single order                           |
| PATCH  | `/api/v1/orders/{id}`                          | Staff   | Update order details and items             |
| POST   | `/api/v1/orders/{id}/approve`                  | Staff   | Approve (pendiente → recibida)             |
| POST   | `/api/v1/orders/{id}/reject`                   | Staff   | Reject with reason (→ cancelada)           |
| PATCH  | `/api/v1/orders/{id}/status`                   | Staff   | Advance status                             |
| PATCH  | `/api/v1/orders/{id}/notes`                    | Staff   | Update internal notes only                 |
| DELETE | `/api/v1/orders/{id}`                          | Admin   | Delete order                               |

### Flavors

| Method | Path                                       | Auth   | Description                         |
| ------ | ------------------------------------------ | ------ | ----------------------------------- |
| GET    | `/api/v1/flavors`                          | Public | List all active flavors             |
| GET    | `/api/v1/flavors/{id}`                     | Public | Get flavor by ID                    |
| POST   | `/api/v1/flavors`                          | Staff  | Create flavor                       |
| PATCH  | `/api/v1/flavors/{id}`                     | Staff  | Update flavor                       |
| DELETE | `/api/v1/flavors/{id}`                     | Staff  | Delete flavor                       |
| POST   | `/api/v1/products/{id}/flavors/{flavorId}` | Staff  | Link flavor to product              |
| DELETE | `/api/v1/products/{id}/flavors/{flavorId}` | Staff  | Unlink flavor from product          |

### Addons

| Method | Path                                       | Auth   | Description                         |
| ------ | ------------------------------------------ | ------ | ----------------------------------- |
| GET    | `/api/v1/addons`                           | Public | List all active addons              |
| GET    | `/api/v1/addons/{id}`                      | Public | Get addon by ID                     |
| POST   | `/api/v1/addons`                           | Staff  | Create addon                        |
| PATCH  | `/api/v1/addons/{id}`                      | Staff  | Update addon                        |
| DELETE | `/api/v1/addons/{id}`                      | Staff  | Delete addon                        |
| POST   | `/api/v1/products/{id}/addons/{addonId}`   | Staff  | Link addon to product               |
| DELETE | `/api/v1/products/{id}/addons/{addonId}`   | Staff  | Unlink addon from product           |

### Analytics

| Method | Path                                  | Auth  | Description                    |
| ------ | ------------------------------------- | ----- | ------------------------------ |
| GET    | `/api/v1/analytics/overview`          | Admin | KPI summary metrics            |
| GET    | `/api/v1/analytics/orders-over-time`  | Admin | Time-series order counts       |
| GET    | `/api/v1/analytics/top-products`      | Admin | Most sold products breakdown   |

### Image Upload

| Method | Path                     | Auth  | Description            |
| ------ | ------------------------ | ----- | ---------------------- |
| POST   | `/api/v1/upload/image`   | Admin | Upload image (max 5MB) |
| DELETE | `/api/v1/upload/image`   | Admin | Delete image by path   |
| GET    | `/api/v1/upload/images`  | Admin | List media library     |

### Employees

| Method | Path                               | Auth  | Description                     |
| ------ | ---------------------------------- | ----- | ------------------------------- |
| GET    | `/api/v1/employees`                | Admin | List all staff accounts         |
| POST   | `/api/v1/employees`                | Admin | Create employee                 |
| PATCH  | `/api/v1/employees/{id}`           | Admin | Update employee fields/state    |
| PATCH  | `/api/v1/employees/{id}/deactivate`| Admin | Deactivate employee             |
| DELETE | `/api/v1/employees/{id}`           | Admin | Permanently delete employee     |

### Users (Customer Directory)

| Method | Path                         | Auth  | Description                               |
| ------ | ---------------------------- | ----- | ----------------------------------------- |
| GET    | `/api/v1/users`              | Staff | List all users                            |
| GET    | `/api/v1/users/{id}`         | Staff | Get user by ID                            |
| GET    | `/api/v1/users/{id}/orders`  | Admin | Get orders associated with this user      |
| POST   | `/api/v1/users`              | Admin | Create user                               |
| PATCH  | `/api/v1/users/{id}`         | Admin | Update user                               |
| DELETE | `/api/v1/users/{id}`         | Admin | Delete user                               |

### Admin Settings

| Method | Path                             | Auth  | Description                                  |
| ------ | -------------------------------- | ----- | -------------------------------------------- |
| GET    | `/api/v1/settings/tabs`          | Staff | Get saved dashboard tab order                |
| PATCH  | `/api/v1/settings/tabs`          | Admin | Update dashboard tab order                   |
| GET    | `/api/v1/settings/panel-config`  | Staff | Get panel configuration (token TTLs, etc.)   |
| PATCH  | `/api/v1/settings/panel-config`  | Admin | Update panel configuration                   |

### Export / Purge

| Method | Path                      | Auth  | Description                       |
| ------ | ------------------------- | ----- | --------------------------------- |
| GET    | `/api/v1/export/orders`   | Admin | Export orders as CSV              |
| DELETE | `/api/v1/export/orders`   | Admin | Purge old orders from the database|

### Health

| Method | Path      | Auth   | Description        |
| ------ | --------- | ------ | ------------------ |
| GET    | `/health` | Public | Server health check|

> **Staff** = employee or admin role. **Admin** = admin role only.

## Order Status Workflow

```
Customer submits order
        │
        ▼
 pendiente_revision ──▸ cancelada (rejected with reason)
        │
        ▼ (approved)
     recibida
        │
        ▼
    en_proceso
        │
        ▼
      lista ──▸ WhatsApp: "¡Tu orden está lista!"
        │
        ▼
    entregada
```

## Environment Variables

| Variable            | Required | Default                 | Description                           |
| ------------------- | -------- | ----------------------- | ------------------------------------- |
| `PORT`              | No       | `8080`                  | HTTP server port                      |
| `ENV`               | No       | `development`           | `development` or `production`         |
| `DATABASE_URL`      | Yes      | —                       | Neon PostgreSQL connection string     |
| `PB_DATA_DIR`       | No       | `./pb_data`             | PocketBase SQLite data directory      |
| `ENABLE_PB_ADMIN_UI`| No       | `false`                 | Enables PocketBase admin UI server    |
| `PB_ADMIN_UI_HOST`  | No       | `127.0.0.1`             | Bind host for PocketBase admin UI     |
| `PB_ADMIN_UI_PORT`  | No       | `8090`                  | Port for PocketBase admin UI          |
| `PB_ADMIN_UI_ALLOWED_ORIGINS` | No | `http://localhost:8090` | Allowed origins for PocketBase admin UI |
| `ALLOW_PB_ADMIN_UI_IN_PRODUCTION` | No | `false`          | Explicit override to allow PB admin UI in production |
| `R2_ACCESS_KEY_ID`  | Yes      | —                       | Cloudflare R2 S3 access key ID        |
| `R2_SECRET_ACCESS_KEY` | Yes   | —                       | Cloudflare R2 S3 secret access key    |
| `R2_BUCKET_NAME`    | No       | `product-images`        | R2 bucket name                        |
| `R2_REGION`         | No       | `auto`                  | R2 region for the S3 SDK              |
| `R2_ENDPOINT`       | Cond.    | derived from account ID | R2 S3 endpoint                        |
| `R2_ACCOUNT_ID`     | Cond.    | —                       | Used to derive `R2_ENDPOINT` if unset |
| `R2_PUBLIC_BASE_URL`| Yes      | —                       | Public bucket URL or custom domain    |
| `WHATSAPP_PHONE_ID` | No       | —                       | Meta WhatsApp phone number ID         |
| `WHATSAPP_TOKEN`    | No       | —                       | Meta WhatsApp API token               |
| `WHATSAPP_NUMBER`   | No       | —                       | Business phone displayed to customers |
| `PUBLIC_STORE_URL`  | No       | `http://localhost:4321` | Storefront base URL used in customer links |
| `ORDERS_PER_HOUR`   | No       | `5`                     | Max orders per IP per hour            |
| `ORDERS_PER_DAY`    | No       | `20`                    | Max orders per phone per day          |
| `TRACKING_REQUESTS_PER_HOUR` | No | `60`                | Max public tracking requests per IP per hour |
| `TRACKING_TOKEN_SECRET` | No   | `DATABASE_URL` fallback | Signing key for public tracking tokens (set explicitly in production) |
| `TRACKING_TOKEN_TTL_HOURS` | No | `720`               | Public tracking token lifetime in hours |
| `REDIS_URL`             | No       | —                       | Redis connection URL for distributed rate limiting (falls back to in-memory) |
| `REDIS_PREFIX`          | No       | `ishos:`                | Key prefix for Redis rate limit entries |
| `AUTH_COOKIE_TTL_HOURS` | No       | `24`                    | `auth_token` cookie lifetime in hours  |
| `AUTH_TOKEN_TTL_HOURS`  | No       | `168`                   | JWT token validity in hours (7 days)   |
| `LOGIN_ATTEMPTS_PER_MINUTE` | No   | `5`                     | Max failed login attempts per IP per minute |
| `ALLOWED_ORIGINS`       | No       | `http://localhost:4321` | Comma-separated CORS allowed origins   |
| `LOGIN_ATTEMPTS_PER_MINUTE` | No | `10`                  | Max login attempts per IP per minute  |
| `LOGIN_LOCKOUT_MAX_FAILURES` | No | `5`                  | Failed logins before lockout is applied |
| `LOGIN_LOCKOUT_WINDOW_MINUTES` | No | `15`               | Rolling window used to count failed logins |
| `LOGIN_LOCKOUT_DURATION_MINUTES` | No | `15`             | Lockout duration once threshold is reached |
| `AUTH_COOKIE_TTL_HOURS` | No | `24`                    | `auth_token` cookie lifetime in browser |
| `AUTH_TOKEN_TTL_HOURS` | No | `168`                   | PocketBase employee JWT lifetime in hours |
| `ALLOWED_ORIGINS`   | No       | `http://localhost:4321` | Comma-separated CORS allowed origins  |

## Deployment

### Production Notes

- **PocketBase data persistence**: PocketBase stores auth data in SQLite at `pb_data/`. This directory **must** be on a persistent volume in production.
- **Single app port**: Chi serves all API routes on one port.
- **PocketBase Admin UI**: keep `ENABLE_PB_ADMIN_UI=false` in production unless a short-lived operational need requires it.
- **No external auth service**: PocketBase runs as an embedded Go library — no separate service, no tunnel, no external API calls for auth.

## License

Private — IshosFactory
