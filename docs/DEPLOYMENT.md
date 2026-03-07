# Deployment

## Build

```sh
go build -o server ./cmd/api
```

## Required Environment Variables

- `DATABASE_URL`
- `SUPABASE_URL`
- `SUPABASE_KEY`
- `PB_DATA_DIR` (default `./pb_data`)
- `ALLOWED_ORIGINS`

`DATABASE_URL` note:

- Prefer the Neon direct host (no `-pooler` in hostname) for this API service.
- Example: `postgres://user:pass@ep-xxx.us-east-2.aws.neon.tech/db?sslmode=require`
- If you see intermittent errors like `unnamed prepared statement does not exist`, verify you are not using the Neon pooler endpoint.

Optional:

- `PORT` (default `8080`)
- `SUPABASE_BUCKET`
- `WHATSAPP_PHONE_ID`
- `WHATSAPP_TOKEN`
- `WHATSAPP_NUMBER`
- `ORDERS_PER_HOUR`
- `ORDERS_PER_DAY`
- `LOGIN_ATTEMPTS_PER_MINUTE`
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
```
