# Deployment

## Build

```sh
go build -o server ./cmd/api
```

## Required Environment Variables

- `DATABASE_URL`
- `R2_ACCESS_KEY_ID`
- `R2_SECRET_ACCESS_KEY`
- `R2_PUBLIC_BASE_URL`
- `PB_DATA_DIR` (default `./pb_data`)
- `ALLOWED_ORIGINS`

`DATABASE_URL` note:

- Prefer the Neon direct host (no `-pooler` in hostname) for this API service.
- Example: `postgres://user:pass@ep-xxx.us-east-2.aws.neon.tech/db?sslmode=require`
- If you see intermittent errors like `unnamed prepared statement does not exist`, verify you are not using the Neon pooler endpoint.

Optional:

- `PORT` (default `8080`)
- `R2_BUCKET_NAME`
- `R2_REGION` (default `auto`)
- `R2_ENDPOINT` (optional if `R2_ACCOUNT_ID` is set)
- `R2_ACCOUNT_ID` (optional if `R2_ENDPOINT` is set)
- `WHATSAPP_PHONE_ID`
- `WHATSAPP_TOKEN`
- `WHATSAPP_NUMBER`
- `PUBLIC_STORE_URL` (default `http://localhost:4321`)
- `ORDERS_PER_HOUR` (default `5`)
- `ORDERS_PER_DAY` (default `20`)
- `TRACKING_REQUESTS_PER_HOUR` (default `60`)
- `TRACKING_TOKEN_SECRET` (default falls back to `DATABASE_URL`; **set an explicit secret in production**)
- `TRACKING_TOKEN_TTL_HOURS` (default `720` / 30 days)
- `LOGIN_ATTEMPTS_PER_MINUTE` (default `5`)
- `AUTH_COOKIE_TTL_HOURS` (default `24`)
- `AUTH_TOKEN_TTL_HOURS` (default `168` / 7 days)
- `REDIS_URL` — Redis connection URL for distributed rate limiting (optional; falls back to in-memory)
- `REDIS_PREFIX` (default `ishos:`)
- `ENABLE_PB_ADMIN_UI` (default `false`)
- `PB_ADMIN_UI_HOST` (default `127.0.0.1`)
- `PB_ADMIN_UI_PORT` (default `8090`)
- `PB_ADMIN_UI_ALLOWED_ORIGINS` (default `http://localhost:8090`)
- `ALLOW_PB_ADMIN_UI_IN_PRODUCTION` (default `false`)

## Admin UI Safety (PocketBase)

- Keep `ENABLE_PB_ADMIN_UI=false` in production.
- Keep `ALLOW_PB_ADMIN_UI_IN_PRODUCTION=false` unless there is a short-lived incident response need.
- If temporarily enabled, restrict `PB_ADMIN_UI_ALLOWED_ORIGINS` to specific trusted origins and disable it again after use.

## Persistent Data Requirement

`pb_data/` must be stored on a persistent volume in production because PocketBase auth data is written there.

## Platform Notes

### Railway

1. Connect GitHub repo.
2. Set env vars.
3. Start command: `./server`.
4. Mount persistent volume at `/app/pb_data`.

### Render

1. Create Web Service.
2. Build command: `go build -o server ./cmd/api`.
3. Start command: `./server`.
4. Attach persistent disk at `/app/pb_data`.

### Fly.io

1. Create volume: `fly volumes create pb_data --size 1`.
2. Mount at `/app/pb_data`.
3. Set secrets: `fly secrets set ...`.
4. Deploy: `fly deploy`.

## Smoke Checks After Deploy

```sh
curl https://<host>/health
curl https://<host>/api/v1/products
curl https://<host>/api/v1/categories
```
