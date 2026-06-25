# Guide — IshosFactory Backend

Simple patterns for working with this codebase.

---

## Architecture

**Request flow:**
```
Handler (parse input, delegate)
    ↓
Service (business logic)
    ↓
Repository (Bun ORM queries → DTO)
    ↓
PostgreSQL
```

**Rules:** Handlers and services must never import `internal/models` or database drivers. Repositories must map Bun model types to DTOs before returning.

**Authentication** is cookie-based. Login sets an HttpOnly, Secure, SameSite=Lax `auth_token` cookie. The JWT is never returned in the response body. A `Bearer` header is accepted as a fallback for non-browser clients, but no endpoint issues bearer tokens. The frontend relies on the browser cookie jar; no manual token management is needed.

---

## Patterns

### Handler
**Location:** `internal/handlers/`  
**Do:** Parse input, validate, call service  
**Don't:** Business logic, database queries

Request types live in `internal/handlers/requests/` (e.g. `requests.CreateOrderRequest`). Response types live in `internal/dto/`. This split keeps request-parsing types local to handlers while sharing response shapes across layers.

```go
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body requests.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	// ... call service
}
```

### Service
**Location:** `internal/services/`  
**Do:** Business rules, call repositories, publish events  
**Don't:** HTTP concerns, direct database access

```go
func (s *OrderService) GetByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	return s.query.GetByIDWithItems(ctx, id)
}
```

#### Order service facade pattern

The order domain is large enough that a single `OrderService` would be unwieldy. It is split into a **facade** (`OrderService`) that delegates to four focused sub-services:

| Sub-service | File | Responsibility |
| --- | --- | --- |
| `OrderCommandService` | `order_command_service.go` | Writes: create, approve, reject, update status, update items, delete |
| `OrderQueryService` | `order_query_service.go` | Reads: list, get by ID/order number/phone |
| `OrderTrackingService` | `order_tracking_service.go` | Tracking token issuance |
| `OrderEnrichmentService` | `order_enrichment_service.go` | Customization enrichment and pricing |

The facade keeps constructor call sites simple (one `NewOrderService` call) while each sub-service stays small and testable. **This is the documented exception** to the "one service per domain" pattern. Other domains (products, flavors, addons, categories) use a single service.

### Repository
**Location:** `internal/repository/`  
**Do:** Bun ORM queries ONLY, map model types to DTOs  
**Don't:** Business logic, raw SQL, expose model types to callers

```go
func (r *OrderBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Order{}, translateOrderError(err)
	}
	return toOrderDTOFromModel(order, nil), nil
}
```

#### Transactional stores

Some operations span multiple tables or domains and must be atomic. `OrderStore` (`internal/repository/order_store.go`) encapsulates these transactions:

| Method | When to use |
| --- | --- |
| `CreateOrder` | Create order + items (validates products, computes total) |
| `UpdateOrder` | Update order metadata + replace items (no stock change) |
| `UpdateOrderWithStockAdjustment` | Update order + items **and** restore/deduct stock atomically |
| `UpdateStatus` | Change order status (no stock change) |
| `UpdateStatusWithStockAdjustment` | Change status **and** deduct or restore stock atomically |

The stock-adjustment methods open a single Bun transaction, perform the order mutation, then call `InventoryBunRepository` (constructed with the transaction's `bun.Tx`) to adjust stock. If either step fails, both roll back.

`InventoryBunRepository` accepts `bun.IDB` (either `*bun.DB` or `bun.Tx`) so it can run standalone or inside an existing transaction via its `withTx` helper.

### Domain
**Location:** `internal/domain/`  
**Do:** Pure business logic, type-safe, testable  
**Don't:** I/O, dependencies, database

```go
func (o *Order) Approve(reviewer string) error {
	if o.Status != StatusPending {
		return ErrInvalidStatus
	}
	o.Status = StatusApproved
	return nil
}
```

---

## Key Rules

🔒 **Security**
- ✅ Use Bun ORM for ALL queries (parameterized, SQL injection prevention)
- ✅ Log sensitive operations (via domain events)
- ✅ Validate input in handlers
- ✅ Auth via HttpOnly cookie (SameSite=Lax, no JS-accessible tokens)
- ❌ Never use raw SQL or hardcode secrets
- ❌ No `X-CSRF-Token` in CORS headers (SameSite=Lax covers CSRF)

🏗️ **Structure**
- ✅ Handlers < 50 LOC (no database imports)
- ✅ Services have single responsibility (no database imports)
- ✅ Repositories return DTOs (not model types)
- ✅ Request types in `internal/handlers/requests/`, response types in `internal/dto/`
- ✅ Business logic goes in domain/ or services/
- ✅ Stock-affecting order mutations use `OrderStore` transactional methods (atomic)
- ❌ No `internal/models` or DB driver imports in `internal/services/`, `internal/handlers/`, or `internal/domain/`

🧪 **Testing**
- ✅ Unit tests: Domain logic and service routing (no database needed)
- ✅ Integration tests: Repository + Store with real PostgreSQL
- ✅ Mocks: Test services in isolation
- ✅ Rollback tests: `internal/repository/order_store_test.go` proves atomic rollback

---

## Common Tasks

### Add a new endpoint
1. Create handler in `internal/handlers/`
2. Create request type in `internal/handlers/requests/` if needed
3. Create service method in `internal/services/`
4. Add repository method if needed
5. Wire route in `internal/server/server.go`
6. Add tests (unit test for routing, integration test if DB is involved)
7. Update `docs/API.md`

### Add a database entity
1. Add model: `internal/models/models.go`
2. Add table/index/FK in: `internal/database/bun_migrate.go`
3. Create repository, service, handler
4. Run: `make migrate`

### Add authentication to endpoint
1. Wrap route: `router.With(authMiddleware.RequireSession).Post(...)`
2. Get user record in handler: `record, ok := auth.GetRecord(r.Context())`
3. Get role: `role := auth.RoleFromContext(r.Context())`

No token management is needed on the frontend — the `auth_token` cookie is set by login and sent automatically by the browser.

### Modify order status with stock impact
Use `OrderStore.UpdateStatusWithStockAdjustment` — never call `orderRepo.UpdateStatus` and `inventory.DeductForOrder` separately, as that leaves the system in an inconsistent state if one fails.

```go
// Correct: atomic
updated, err := s.orderStore.UpdateStatusWithStockAdjustment(ctx, id, "en_proceso", deductions, false)

// Wrong: silent data drift on failure
_, _ = s.inventory.DeductForOrder(ctx, id, items, "") // error discarded!
```

---

## Commands

```bash
# Development
go run ./cmd/api              # Run server
go test ./...                 # All tests (needs PostgreSQL for repository tests)
go fmt ./...                  # Format code
golangci-lint run             # Lint

# Unit tests only (no database required)
go test ./internal/apperror ./internal/events ./internal/services ./internal/validator

# Database
make migrate                  # Run migrations
make seed                     # Seed test data
make docker-up                # Start local PostgreSQL

# Setup
make bootstrap-admin          # Create first admin
```

---

## Project Structure

```
cmd/api              # Server entry point
internal/
├── handlers/        # HTTP layer (no database imports)
│   └── requests/    # Request-parsing DTOs (handler-local)
├── services/        # Business logic (no database imports)
├── repository/      # Data access — the ONLY layer that imports internal/models
├── models/          # Bun-tagged database models
├── database/        # Connection pool and migrations
├── domain/          # Pure business rules (no database imports)
├── dto/             # Response DTOs (shared across layers)
├── middleware/      # Auth, logging, etc.
└── ...
docs/               # Documentation
README.md           # Quick start
```

---

## Troubleshooting

**Database error**
```bash
# Start local PostgreSQL via Docker
make docker-up
# Or manually
docker run -d -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:16-alpine
```

**Repository tests skip silently**
Repository integration tests (`internal/repository/`) call `t.Skipf` if PostgreSQL is not reachable. Start a database with `make docker-up` or set `DATABASE_URL`.

**Type mismatch: "cannot use X as Y"**
- Check that repositories return DTOs, not model types
- Check that request types come from `internal/handlers/requests/`, not `internal/dto/`
- Example: `toOrderDTOFromModel(order, nil)` not `order`

---

## Need More Details?

- **Endpoints:** See `docs/API.md`
- **Deployment:** See `docs/DEPLOYMENT.md`

---
