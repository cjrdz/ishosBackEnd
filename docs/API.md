# API Documentation

The API contract is maintained as an interactive OpenAPI-based UI and a static spec file.

- **Interactive docs:** open **`/docs`** on a running server (e.g. `http://localhost:8080/docs`)
- **OpenAPI spec:** **`/openapi.yaml`** or **`/openapi.json`**
- **Build artifact:** `api/openapi.yaml`
- **Generate locally:** `go run ./cmd/api openapi > api/openapi.yaml`

See [`docs/GUIDE.md`](GUIDE.md) for endpoint patterns and the README for an overview.
