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
Repository (Ent ORM queries → DTO)
    ↓
PostgreSQL
```

**Rules:** Handlers and services must never import `ent`. Repositories must map Ent types to DTOs before returning.

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
**Do:** Ent ORM queries ONLY, map Ent types to DTOs  
**Don't:** Business logic, raw SQL, expose Ent types to callers

```go
func (r *orderRepository) GetByID(ctx context.Context, id uuid.UUID) (dto.Order, error) {
	entOrder, err := r.client.Order.Get(ctx, id)
	if err != nil {
		return dto.Order{}, apperror.NotFoundError{}
	}
	return toDTOOrder(entOrder), nil
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
- ✅ Use Ent ORM for ALL queries (automatic SQL injection prevention)
- ✅ Log sensitive operations (via domain events)
- ✅ Validate input in handlers
- ❌ Never use raw SQL or hardcode secrets

🏗️ **Structure**
- ✅ Handlers < 50 LOC (no Ent imports)
- ✅ Services have single responsibility (no Ent imports)
- ✅ Repositories return DTOs (not Ent types)
- ✅ Business logic goes in domain/ or services/
- ❌ No `ent` import in `internal/services/`, `internal/handlers/`, or `internal/domain/`

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
1. Create schema: `ent/schema/myentity.go`
2. Run: `go generate ./ent`
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
go generate ./ent             # Generate Ent code (after schema changes)

# Setup
make bootstrap-admin          # Create first admin
```

---

## Project Structure

```
cmd/api              # Server entry point
ent/schema/          # Entity definitions (edit these — do NOT edit generated ent/ files)
internal/
├── handlers/        # HTTP layer (no ent imports)
├── services/        # Business logic (no ent imports)
├── repository/      # Data access — the ONLY layer that imports ent
├── domain/          # Pure business rules (no ent imports)
├── middleware/      # Auth, logging, etc.
└── ...
docs/               # Documentation
README.md           # Quick start
README.md           # Quick start
```

---

## Troubleshooting

**Build error: "undefined reference to ent/order"**
```bash
go generate ./ent
```

**Database error**
```bash
# Make sure PostgreSQL is running
psql -h localhost -U postgres
# Or use Docker
docker run -d -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15
```

**Type mismatch: "cannot use X as Y"**
- Check that repositories return domain types, not Ent types
- Example: `mapEntOrderToDomain(entOrder)` not `entOrder`

---

## Need More Details?

- **Endpoints:** See `docs/API.md`
- **Deployment:** See `docs/DEPLOYMENT.md`

---
