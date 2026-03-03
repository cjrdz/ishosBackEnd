# PocketBase Integration

This project uses PocketBase as an embedded Go library. The optional PocketBase Admin UI server can be enabled for development.

## What PocketBase Handles

- Employee/admin account storage
- Password hashing and auth verification
- JWT token validation for protected routes

## Where It Lives

- Integration wrapper: `internal/auth/pocketbase.go`
- Auth middleware: `internal/middleware/auth.go`
- Data directory: `PB_DATA_DIR` (default `./pb_data`)

## Important Operational Notes

- Persist `pb_data/` in production.
- Do not commit `.db` files from `pb_data/`.
- Disabling an employee account prevents future login attempts.
- Keep `ENABLE_PB_ADMIN_UI=false` in production unless there is a short-lived operational need.

## Auth Route Overview

- `POST /api/v1/auth/login`
- `GET /api/v1/auth/session`
- `POST /api/v1/auth/logout`

## Employee Management

Admin-only endpoints:

- `GET /api/v1/employees`
- `POST /api/v1/employees`
- `PATCH /api/v1/employees/{id}`
- `POST /api/v1/employees/{id}/deactivate`
- `DELETE /api/v1/employees/{id}` (permanent delete)
