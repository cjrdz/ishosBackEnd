# IshosFactory Backend

Go + Chi + Ent + PocketBase service. Manages orders, products, employees, customers, and settings.

---

## Quick Start

```bash
cp .env.example .env
go mod tidy
go generate ./ent
make migrate
go run ./cmd/api
```

Check health: `curl http://localhost:8080/health`

---

## Architecture

Clean 4-layer model with Ent isolated behind the repository boundary:

```
Handler   → Parse request, delegate to service
Service   → Business logic, call repository interface
Repository → Ent ORM queries, map to DTOs
Database  → PostgreSQL
```

---

## Documentation

- **[GUIDE.md](docs/GUIDE.md)** ← Start here! Patterns, common tasks, troubleshooting
- **[API.md](docs/API.md)** — All endpoints
- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** — Production setup
- **[docs/_archive/](docs/_archive/)** — Deep dives (if needed)

---

## Key Commands

```bash
go run ./cmd/api              # Run server
go test ./...                 # Run tests
make migrate                  # Database migration
go generate ./ent             # Generate Ent code
make bootstrap-admin          # Create first admin
```

---

## Structure

```
internal/
├── handlers/     # HTTP layer
├── services/     # Business logic
├── repository/   # Database access
├── domain/       # Business rules
└── ...

ent/schema/       # Entity definitions
docs/             # Documentation
```

---

## Security

✅ 100% Ent ORM (no SQL injection)  
✅ Audit logging (all sensitive ops)  
✅ Token revocation (secure logout)  
✅ Rate limiting (login + endpoints)  

---

## Rules

- ✅ Use Ent ORM for all queries (isolated in `internal/repository`)
- ✅ Business logic in services
- ✅ Keep handlers < 50 LOC
- ✅ Repository methods map Ent types → DTOs
- ✅ Test domain logic without database
- ❌ No raw SQL
- ❌ No hardcoded secrets
- ❌ No `ent` import in `internal/services`, `internal/handlers`, or `internal/domain`

---

## Need Help?

- **"How do I add an endpoint?"** → See [GUIDE.md](docs/GUIDE.md)
- **"Where's the API reference?"** → See [API.md](docs/API.md)
- **"How do I deploy?"** → See [DEPLOYMENT.md](docs/DEPLOYMENT.md)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for PR workflow and code review checklist.

---
