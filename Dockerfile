# ── Build stage ────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/api
RUN mkdir -p /app/pb_data

# Provide CA roots for outbound TLS calls from the final minimal image.
FROM alpine:3.22 AS certs
RUN apk --no-cache add ca-certificates

# ── Run stage ──────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=builder --chown=65532:65532 /app/pb_data /app/pb_data
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# PocketBase stores its SQLite auth data here
# VOLUME ["/app/pb_data"]

EXPOSE 8080

# Chi HTTP server starts directly on :8080.
# When ENABLE_PB_ADMIN_UI=true, PocketBase admin UI also starts on :8090.
ENTRYPOINT ["/app/server"]
