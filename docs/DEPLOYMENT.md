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

### Project history

- `2026-07-27` — Clean baseline reset. All three branches (`main`, `sec`, `dev`)
  were squashed to a single `v0.0.1` commit. GitHub Actions workflow runs,
  artifacts, and deployment environments were removed. Old Railway deployments
  were marked `REMOVED` so only the `v0.0.1` baseline deployment remains active.

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
- `REDIS_URL` — optional. Without it, rate limits and login lockouts live in memory (fine for one instance; they reset on redeploy). Set it (Redis or Valkey) before running more than one replica.
- `TRACKING_TOKEN_SECRET` — required in production (do not derive from `DATABASE_URL`)
- `BFF_SHARED_SECRET` — required in production (≥32 chars); must equal `BFF_SHARED_SECRET` on the frontend
  (Cloudflare secret / Vercel env) so per-customer rate limits work behind the BFF
- `ENV=production` — on Railway the service refuses to boot in the `production` environment without it

Operationally important values:

- `STORE_TIMEZONE` (default `America/El_Salvador`) — analytics day/week/month buckets and date filters
- `R2_ENDPOINT` — account endpoint only (`https://<account-id>.r2.cloudflarestorage.com`); the dashboard's
  "S3 API" URL ends in `/<bucket>`, which the server strips with a warning

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

- Use the **direct** Neon host, not the `-pooler` host: migrations and background jobs rely on
  Postgres advisory locks, which PgBouncer transaction pooling breaks.
- Run with SSL enabled.
- Schema bootstrap (`cmd/migrate`, also run at server start) is versioned (`schema_version` table) and
  runs in one transaction under an advisory lock, so an interrupted first boot rolls back cleanly.
  Bump `schemaVersion` in `internal/database/bun_migrate.go` when adding a schema step.

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
- mount `pb_data` on a persistent volume (`PB_DATA_DIR=/app/pb_data`)
- healthcheck path: `/health`
- deploys from GitHub `main`; variable changes trigger a redeploy unless staged with
  `railway variable set --skip-deploys` (deleting a variable always redeploys)

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
