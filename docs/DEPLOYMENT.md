# Deployment

## Versioning

This project follows [Semantic Versioning](https://semver.org/):

- `MAJOR.MINOR.PATCH`
- `MAJOR` — incompatible API or data-model changes
- `MINOR` — new features that are backward compatible
- `PATCH` — bug fixes and small improvements

The current version is stored in `VERSION` at the repository root and should be
updated in the same commit that promotes a release.

### Branch flow

```
feature/bugfix work  →  dev  →  sec  →  main
```

- `dev` — active development, integration branch
- `sec` — staging/pre-production sanity gate
- `main` — production releases

### Release process

1. Merge all planned work into `dev`.
2. Bump `VERSION` (or `package.json` in the frontend) and update this doc if the
   release process itself changed.
3. Open/merge a PR from `dev` → `sec` and validate in staging.
4. Open/merge a PR from `sec` → `main`.
5. Tag the merge commit on `main` as `v<VERSION>` (e.g. `v0.0.1`).

### Tags

- `v0.0.1` — initial clean baseline for all three branches.

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
- `REDIS_URL` — required in production for distributed rate limiting and login lockout state
- `TRACKING_TOKEN_SECRET` — required in production (do not derive from `DATABASE_URL`)

Operationally important values:

- `PB_DATA_DIR` for embedded PocketBase state
- `AUTH_TOKEN_TTL_HOURS`
- `AUTH_COOKIE_TTL_HOURS`
- `TRACKING_TOKEN_TTL_HOURS`
- `REDIS_PREFIX`
- `INACTIVITY_LOGOUT_SECONDS`
- `DB_MAX_OPEN_CONNS` (default 25)
- `DB_MAX_IDLE_CONNS` (default 10)
- `DB_CONN_MAX_LIFETIME_MINUTES` (default 30)
- `DB_CONN_MAX_IDLE_TIME_MINUTES` (default 5)
- `ADMIN_IMAGE_UPLOAD_MAX_MB` (default 15)
- `ADMIN_IMAGE_UPLOAD_MAX_WIDTH` (default 4096)
- `ADMIN_IMAGE_UPLOAD_MAX_HEIGHT` (default 4096)
- `ALLOWED_UPLOAD_FOLDERS` (default `products,categories,flavors,addons,container-types,profile,general`)

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
