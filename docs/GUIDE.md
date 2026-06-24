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

---

## Patterns

### Handler
**Location:** `internal/handlers/`  
**Do:** Parse input, validate, call service  
**Don't:** Business logic, database queries

```go
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.orderService.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "error")
		return
	}
	response.OK(w, http.StatusOK, order)
}
```

### Service
**Location:** `internal/services/`  
**Do:** Business rules, call repositories, publish events  
**Don't:** HTTP concerns, direct database access

```go
func (s *OrderService) GetByID(ctx context.Context, id string) (*Order, error) {
	// Validate, transform, orchestrate
	return s.repo.GetByID(ctx, id)
}
```

### Repository
**Location:** `internal/repository/`  
**Do:** Bun ORM queries ONLY, map model types to DTOs  
**Don't:** Business logic, raw SQL, expose model types to callers

```go
func (r *orderRepository) GetByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	var order models.Order
	if err := r.db.NewSelect().Model(&order).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Order{}, apperror.NotFoundError{}
	}
	return toDTOOrder(order), nil
}
```

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
- ❌ Never use raw SQL or hardcode secrets

🏗️ **Structure**
- ✅ Handlers < 50 LOC (no database imports)
- ✅ Services have single responsibility (no database imports)
- ✅ Repositories return DTOs (not model types)
- ✅ Business logic goes in domain/ or services/
- ❌ No `internal/models` or DB driver imports in `internal/services/`, `internal/handlers/`, or `internal/domain/`

🧪 **Testing**
- ✅ Unit tests: Test domain logic without database
- ✅ Integration tests: Test service + repository with real database
- ✅ Mocks: Test service behavior in isolation

---

## Common Tasks

### Add a new endpoint
1. Create handler in `internal/handlers/`
2. Create service method in `internal/services/`
3. Add repository method if needed
4. Wire route in `internal/server/server.go`
5. Add tests
6. Update `docs/API.md`

### Add a database entity
1. Add model: `internal/models/models.go`
2. Add table/index/FK in: `internal/database/bun_migrate.go`
3. Create repository, service, handler
4. Run: `make migrate`

### Add authentication to endpoint
1. Wrap route: `router.With(middleware.RequireSession).Post(...)`
2. Get user ID in handler: `userID, ok := auth.UserFromContext(r.Context())`

---

## Commands

```bash
# Development
go run ./cmd/api              # Run server
go test ./...                 # All tests
go fmt ./...                  # Format code
golangci-lint run ./...       # Lint

# Database
make migrate                  # Run migrations
make seed                     # Seed test data

# Setup
make bootstrap-admin          # Create first admin
```

---

## Project Structure

```
cmd/api              # Server entry point
internal/
├── handlers/        # HTTP layer (no database imports)
├── services/        # Business logic (no database imports)
├── repository/      # Data access — the ONLY layer that imports internal/models
├── models/          # Bun-tagged database models
├── database/        # Connection pool and migrations
├── domain/          # Pure business rules (no database imports)
├── middleware/      # Auth, logging, etc.
└── ...
docs/               # Documentation
README.md           # Quick start
```

---

## Troubleshooting

**Database error**
```bash
# Make sure PostgreSQL is running
psql -h localhost -U postgres
# Or use Docker
docker run -d -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15
```

**Type mismatch: "cannot use X as Y"**
- Check that repositories return DTOs, not model types
- Example: `toDTOOrder(order)` not `order`

---

## Need More Details?

- **Endpoints:** See `docs/API.md`
- **Deployment:** See `docs/DEPLOYMENT.md`

---
