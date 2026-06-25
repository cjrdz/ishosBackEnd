# Contributing

Welcome! Here's how to work on this project.

---

## Before You Start

Read [docs/GUIDE.md](docs/GUIDE.md) to understand the code structure.

---

## Workflow

### 1. Pick Your Task

```bash
git checkout -b fix/something
# or
git checkout -b feat/something
```

### 2. Make Changes

Follow patterns in [docs/GUIDE.md](docs/GUIDE.md):
- Handlers: < 50 LOC, HTTP only, request types in `internal/handlers/requests/`
- Services: Business logic, call repositories
- Repositories: Bun ORM queries only
- Stock-affecting order mutations must use `OrderStore.UpdateStatusWithStockAdjustment` or `UpdateOrderWithStockAdjustment` (atomic)

### 3. Test

```bash
# Unit tests (no database required)
go test ./internal/apperror ./internal/events ./internal/services ./internal/validator

# All tests including repository integration tests (requires PostgreSQL)
make docker-up   # start local Postgres
go test ./...
go fmt ./...
golangci-lint run
```

> CI runs unit tests and repository integration tests automatically (with a Postgres service container). Local repository tests skip gracefully if no database is reachable.

### 4. Commit

```
<type>(<scope>): <description>
```

Examples:
```
feat(orders): add approval workflow
fix(auth): prevent token replay
refactor(services): split order service
docs(guide): add pattern examples
test(domain): add state machine tests
```

### 5. PR

- Link any related issues
- Ensure CI passes (build, unit tests, repository integration tests, lint)
- Update `docs/API.md` if endpoint changed

---

## Code Review Checklist

Before submitting:
- [ ] `go fmt ./...`
- [ ] `golangci-lint run`
- [ ] `go build ./cmd/api`
- [ ] Unit tests pass (`go test ./internal/apperror ./internal/events ./internal/services ./internal/validator`)
- [ ] Repository integration tests pass (with PostgreSQL running)
- [ ] Tests cover happy path + errors
- [ ] No hardcoded secrets
- [ ] No `_, _ =` on error returns from inventory or order store calls
- [ ] Stock-affecting mutations are atomic (use `OrderStore` transactional methods)
- [ ] Commit message format correct

---

## Rules

**Security:**
- ✅ Use Bun ORM for all queries
- ✅ Log sensitive operations
- ✅ Validate input
- ✅ Auth via HttpOnly cookie (no token in response body)
- ❌ No raw SQL (use parameterized Bun queries)
- ❌ No hardcoded secrets
- ❌ No `X-CSRF-Token` header in CORS (SameSite=Lax covers CSRF)

**Architecture:**
- ✅ Handler: < 50 LOC, HTTP only, request types in `internal/handlers/requests/`
- ✅ Service: Business logic
- ✅ Repository: Bun ORM only, return DTOs (not model types)
- ✅ Domain: Pure business rules (testable without DB)
- ✅ Order service uses facade pattern (command/query/tracking/enrichment split)

**Testing:**
- ✅ Unit tests: Domain logic and service routing (no DB needed)
- ✅ Integration tests: Repository + Store with real PostgreSQL
- ✅ Mocks: Test services in isolation

---

## Need Help?

- **Patterns:** See [docs/GUIDE.md](docs/GUIDE.md)
- **API reference:** See [docs/API.md](docs/API.md)

---
