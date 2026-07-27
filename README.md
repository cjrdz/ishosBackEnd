# IshosFactory Backend

Go + Chi + Bun + PocketBase service. Manages orders, products, employees, customers, inventory, and settings.

**Current version:** `0.0.1` (see [`VERSION`](VERSION) and [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) for release flow).

---

## Quick Start

```bash
cp .env.example .env
go mod tidy
make migrate
go run ./cmd/api
```

Check health: `curl http://localhost:8080/health`

---

## Architecture

Clean 4-layer model with Bun isolated behind the repository boundary:

```
Handler   → Parse request, delegate to service
Service   → Business logic, call repository interface
Repository → Bun ORM queries, map to DTOs
Database  → PostgreSQL
```

**Authentication** is cookie-based: login sets an HttpOnly, Secure, SameSite=Lax `auth_token` cookie. The JWT is never returned in the response body. A `Bearer` header is accepted as a fallback for non-browser clients, but no endpoint issues bearer tokens.

---

## Documentation

- **[GUIDE.md](docs/GUIDE.md)** ← Start here! Patterns, common tasks, troubleshooting
- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** — Production setup and environment variables
- **[API contract](api/openapi.yaml)** — Authoritative OpenAPI 3.0 spec generated from source
  - Also served live at `/openapi.yaml` and `/openapi.json`
  - Interactive UI (Scalar) at **`/docs`** on a running server
  - Generate locally: `go run ./cmd/api openapi > api/openapi.yaml`
- **[API.md](docs/API.md)** — Points to `/docs` and the OpenAPI spec

---

## Key Commands

```bash
go run ./cmd/api              # Run server
go test ./...                 # Run all tests
make migrate                  # Database migration
make bootstrap-admin          # Create first admin
go run ./cmd/api openapi      # Print OpenAPI YAML to stdout
```

> Repository integration tests require PostgreSQL. Use `make docker-up` to start a local Postgres, or run them via CI (which provisions a Postgres service container automatically).

---

## Structure

```
internal/
├── handlers/          # HTTP layer
│   └── requests/      # Request-parsing DTOs (handler-local)
├── services/          # Business logic
├── repository/        # Database access (Bun ORM)
├── models/            # Bun-tagged database models
├── database/          # Connection pool and migrations
├── domain/            # Pure business rules
├── dto/               # Response DTOs (shared across layers)
└── ...

docs/                  # Documentation
```

---

## Security & Operations

✅ Bun ORM queries (no SQL injection)
✅ Transactional audit logging for order and catalog mutations
✅ Token revocation (secure logout validates the token first)
✅ Rate limiting (login + public orders + tracking) with Redis-backed distributed state
✅ HttpOnly cookie auth (no JS-accessible tokens)
✅ SameSite=Lax CSRF protection
✅ Atomic stock transactions (status + inventory committed together)
✅ Idempotent order creation (client-supplied key prevents duplicate orders)
✅ Global 1 MiB JSON body-size limit
✅ Hardened CORS (wildcard origins rejected when credentials are enabled)
✅ Hardened image uploads (dimensions, extension/content-type match, folder allowlist)
✅ Sanitized fallback error messages (no internal details leaked)
✅ Configurable database connection pool
✅ Bounded in-memory cache with optional LRU eviction
✅ Read-only transactions for analytics and export queries
✅ Graceful shutdown of background jobs, event dispatcher, and WhatsApp worker

---

## Rules

- ✅ Use Bun ORM for all queries (isolated in `internal/repository`)
- ✅ Business logic in services
- ✅ Keep handlers < 50 LOC
- ✅ Repository methods map model types → DTOs
- ✅ Test domain logic without database
- ❌ No raw SQL
- ❌ No hardcoded secrets
- ❌ No `internal/models` import in `internal/services`, `internal/handlers`, or `internal/domain`

---

## Need Help?

- **"How do I add an endpoint?"** → See [GUIDE.md](docs/GUIDE.md)
- **"Where's the API reference?"** → See the interactive docs at `/docs` or the OpenAPI spec at `/openapi.yaml`
- **"How do I deploy?"** → See [DEPLOYMENT.md](docs/DEPLOYMENT.md)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for PR workflow and code review checklist.

---
