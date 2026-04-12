# Development Guide

## Local Setup

```sh
cp .env.example .env
go mod tidy
go generate ./ent
go run ./cmd/api
```

## Common Commands

```sh
make dev
make test
make migrate
make seed
make seed-flavors
make lint
```

## Project Layers

- `internal/handlers`: transport/API layer
- `internal/services`: business logic
- `internal/repository`: data access
- `internal/validator`: input validation rules

## Testing

- Unit/package tests: `go test ./...`
- Integration scaffolding: `tests/integration/*`
- Enable integration tests with:

```sh
RUN_INTEGRATION=1 go test ./tests/integration/...
```

Optional env vars for integration auth tests:

- `INTEGRATION_BASE_URL`
- `INTEGRATION_ADMIN_EMAIL`
- `INTEGRATION_ADMIN_PASSWORD`

## Coding Notes

- Prefer adding business logic in services, not handlers.
- Keep repositories Ent-specific and side-effect free.
- Use validator package for request payload checks.
- Regenerate Ent artifacts only via `go generate ./ent`.
