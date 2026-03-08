# ── Build stage ────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/api

# ── Run stage ──────────────────────────────────────────────
FROM alpine:3.20

# Security: run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /app/server .

# PocketBase stores its SQLite auth data here
RUN mkdir -p /app/pb_data && chown -R appuser:appgroup /app/pb_data
VOLUME ["/app/pb_data"]

USER appuser

EXPOSE 8080

# Chi HTTP server starts directly on :8080.
# When ENABLE_PB_ADMIN_UI=true, PocketBase admin UI also starts on :8090.
CMD ["./server"]
