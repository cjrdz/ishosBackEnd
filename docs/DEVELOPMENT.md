# Development Guide

## Local setup

```sh
cp .env.example .env
go mod tidy
go generate ./ent
make migrate
go run ./cmd/api
```

`make migrate` and API startup both ensure the schema is up to date.

## Common commands

```sh
make test
make coverage-focus
make migrate
make seed
make lint
make bootstrap-admin
go run ./cmd/bootstrap -email admin@domain.com -password StrongPass123! -name Admin
go run ./cmd/seed
```

## Layering rules

- handlers own HTTP concerns only
- services own business rules and orchestration
- repositories own Ent persistence only
- middleware owns cross-cutting policy
- validator owns request validation rules

If a handler starts deciding business policy, move that logic into a service.

## Testing

Quick package test run:

```sh
go test ./...
```

Preferred local validation before merge:

```sh
go test ./...
make test
make coverage-focus
go build ./...
```

Integration scaffolding remains opt-in:

```sh
RUN_INTEGRATION=1 go test ./tests/integration/...
```

## Adding or changing an entity

Follow the same sequence used for orders, products, and customers:

1. Update `ent/schema/<entity>.go`
2. Regenerate Ent code with `go generate ./ent`
3. Add or update repository methods
4. Add or update service logic
5. Add or update handler transport mapping
6. Register routes in `internal/server/server.go`
7. Update validation rules in `internal/validator`
8. Update [API.md](API.md) and the README if the public contract changed

## Customer directory note

`/api/v1/users` is implemented by `handlers/users.go`, but the underlying domain object is `Customer`. Keep that distinction clear in code reviews and docs.

## Frontend integration note

The Astro frontend consumes the backend through BFF routes. When backend paths change, update:

- route registration in `internal/server/server.go`
- backend docs
- frontend BFF route mappings under `ishosFrontEnd/src/pages/api`

## Guardrails

- Do not bypass global security headers casually.
- Prefer explicit route middleware over handler-local auth checks.
- Keep upload paths stable; media URLs are part of the frontend contract.
- Prefer Redis-backed rate limiting in shared environments, but preserve in-memory fallback.
