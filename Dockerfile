# ── Build stage ────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/healthcheck ./cmd/healthcheck
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bootstrap ./cmd/bootstrap
RUN mkdir -p /app/pb_data

# ── Run stage ──────────────────────────────────────────────
FROM alpine:3.22

WORKDIR /app

# Provide CA roots for outbound TLS calls and a shell for Railway console.
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/server /app/server
COPY --from=builder /app/healthcheck /app/healthcheck
COPY --from=builder /app/bootstrap /app/bootstrap
COPY --from=builder /app/pb_data /app/pb_data

# PocketBase stores its SQLite auth data here
# VOLUME ["/app/pb_data"]

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/app/healthcheck", "-url", "http://127.0.0.1:8080/health"]

# Chi HTTP server starts directly on :8080.
# When ENABLE_PB_ADMIN_UI=true, PocketBase admin UI also starts on :8090.
ENTRYPOINT ["/app/server"]
