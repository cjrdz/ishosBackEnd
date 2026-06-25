# IshosFactory Backend

Go + Chi + Bun + PocketBase service. Manages orders, products, employees, customers, inventory, and settings.

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
- **[API.md](docs/API.md)** — All endpoints
- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** — Production setup

---

## Key Commands

```bash
go run ./cmd/api              # Run server
go test ./...                 # Run all tests
make migrate                  # Database migration
make bootstrap-admin          # Create first admin
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

## Security

✅ Bun ORM queries (no SQL injection)
✅ Audit logging (all sensitive ops)
✅ Token revocation (secure logout)
✅ Rate limiting (login + endpoints)
✅ HttpOnly cookie auth (no JS-accessible tokens)
✅ SameSite=Lax CSRF protection
✅ Atomic stock transactions (status + inventory committed together)

---

## Rules

- ✅ Use Bun ORM for all queries (isolated in `internal/repository`)
- ✅ Business logic in services
- ✅ Keep handlers < 50 LOC
- ✅ Repository methods map model types → DTOs
- ✅ Test domain logic without database
- ✅ Stock-affecting transitions use `OrderStore.UpdateStatusWithStockAdjustment` (atomic)
- ❌ No raw SQL
- ❌ No hardcoded secrets
- ❌ No `internal/models` import in `internal/services`, `internal/handlers`, or `internal/domain`

---

## Need Help?

- **"How do I add an endpoint?"** → See [GUIDE.md](docs/GUIDE.md)
- **"Where's the API reference?"** → See [API.md](docs/API.md)
- **"How do I deploy?"** → See [DEPLOYMENT.md](docs/DEPLOYMENT.md)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for PR workflow and code review checklist.

---
