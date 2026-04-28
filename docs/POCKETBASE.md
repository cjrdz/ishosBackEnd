# PocketBase Integration

PocketBase is embedded into the Go process and is used only for employee authentication.

## PocketBase scope

PocketBase owns:

- employee/admin account storage
- password hashing and credential verification
- employee JWT issuance and validation

PocketBase does not own the customer directory. `/api/v1/users` is backed by PostgreSQL `Customer` records.

## Code locations

- auth wrapper: `internal/auth/pocketbase.go`
- auth middleware: `internal/middleware/auth.go`
- persisted state: `PB_DATA_DIR` or `./pb_data`

## Auth routes

- `POST /api/v1/auth/login`
- `GET /api/v1/auth/session`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/lockout/reset` for admin unlock flow

Logout revokes the active token server-side. Session validation is done inside the Go process; there is no external auth API hop.

## Employee management routes

- `GET /api/v1/employees`
- `POST /api/v1/employees`
- `PATCH /api/v1/employees/{id}`
- `PATCH /api/v1/employees/{id}/deactivate`
- `DELETE /api/v1/employees/{id}`

All employee-management routes are admin-only.

## Operational notes

- persist `pb_data` in production
- keep PocketBase admin UI disabled in production unless there is a short-lived operational need
- do not treat PocketBase as a general-purpose datastore for customers or orders
- disabling an employee blocks future login attempts immediately
