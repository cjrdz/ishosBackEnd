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
- Handlers: < 50 LOC, HTTP only
- Services: Business logic, call repositories
- Repositories: Ent ORM queries only

### 3. Test

```bash
go test ./...
go fmt ./...
golangci-lint run ./...
```

### 4. Commit

```
<type>(<scope>): <description>

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
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
- Ensure tests pass
- Update docs/API.md if endpoint changed

---

## Code Review Checklist

Before submitting:
- [ ] `go fmt ./...` ✅
- [ ] `golangci-lint run ./...` ✅
- [ ] `go test ./...` ✅
- [ ] `go build ./cmd/api` ✅
- [ ] Tests cover happy path + errors ✅
- [ ] No hardcoded secrets ✅
- [ ] Commit message format correct ✅

---

## Rules

**Security:**
- ✅ Use Ent ORM for all queries
- ✅ Log sensitive operations
- ✅ Validate input
- ❌ No raw SQL
- ❌ No hardcoded secrets

**Architecture:**
- ✅ Handler: < 50 LOC, HTTP only
- ✅ Service: Business logic
- ✅ Repository: Ent ORM only
- ✅ Domain: Pure business rules (testable without DB)

**Testing:**
- ✅ Unit tests: Domain logic
- ✅ Integration tests: Service + Repository
- ✅ Mocks: Test services in isolation

---

## Need Help?

- **Patterns:** See [docs/GUIDE.md](docs/GUIDE.md)
- **API reference:** See [docs/API.md](docs/API.md)

---
