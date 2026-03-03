#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

echo "Running Go tests with race detector and coverage..."
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

echo "Test run complete. Coverage report: ${ROOT_DIR}/coverage.html"
