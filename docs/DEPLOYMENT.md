# Deployment

## Build

```sh
go build -o server ./cmd/api
```

## Required environment

Minimum production configuration:

- `DATABASE_URL`
- `R2_ACCESS_KEY_ID`
- `R2_SECRET_ACCESS_KEY`
- `R2_PUBLIC_BASE_URL`
- `ALLOWED_ORIGINS`

Operationally important values:

- `PB_DATA_DIR` for embedded PocketBase state
- `AUTH_TOKEN_TTL_HOURS`
- `AUTH_COOKIE_TTL_HOURS`
- `TRACKING_TOKEN_SECRET`
- `TRACKING_TOKEN_TTL_HOURS`
- `REDIS_URL` and `REDIS_PREFIX` when using shared rate limiting

## Database guidance

- Prefer the direct Neon host, not the pooler host, for the API service.
- Run with SSL enabled.
- Keep schema migration and runtime schema checks enabled.

## PocketBase persistence

`pb_data/` must live on persistent storage in production. Losing it means losing employee auth state.

Keep these disabled in normal production operation:

- `ENABLE_PB_ADMIN_UI=false`
- `ALLOW_PB_ADMIN_UI_IN_PRODUCTION=false`

## Release checklist

1. Build the binary.
2. Ensure database and R2 credentials are present.
3. Ensure `TRACKING_TOKEN_SECRET` is explicitly set.
4. Ensure `pb_data` is mounted on persistent storage.
5. Verify `ALLOWED_ORIGINS` matches the deployed frontend origin.
6. Run smoke checks after deploy.

## Smoke checks

```sh
curl https://<host>/health
curl https://<host>/api/v1/products
curl https://<host>/api/v1/categories
curl https://<host>/api/v1/settings/store/public
```

## Platform notes

### Railway

- start command: `./server`
- mount `pb_data` on a persistent volume

### Render

- build command: `go build -o server ./cmd/api`
- start command: `./server`
- mount `pb_data` on a persistent disk

### Fly.io

- create a volume for `pb_data`
- inject secrets for database, R2, auth, and tracking

## Frontend coupling

The Astro frontend proxies backend routes through its BFF. Production deploys must keep these aligned:

- backend origin and `PUBLIC_API_BASE_URL`
- CORS `ALLOWED_ORIGINS`
- store tracking and settings routes
