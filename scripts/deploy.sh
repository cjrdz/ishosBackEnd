#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

echo "Building production binary..."
mkdir -p bin
CGO_ENABLED=0 GOOS=linux go build -o bin/server ./cmd/api

echo "Build complete: ${ROOT_DIR}/bin/server"
echo "Next step: copy binary to your host and restart the service manager."
