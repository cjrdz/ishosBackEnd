#!/usr/bin/env bash
# scripts/seed_all.sh
# Seeds categories, products, flavors, and addons via the Go seeder
# Requirements: go
#
# Usage:
#   chmod +x scripts/seed_all.sh
#   ./scripts/seed_all.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

# Load .env if present
if [ -f ".env" ]; then
  # shellcheck disable=SC1091
  set -o allexport
  # shellcheck disable=SC1090
  source .env
  set +o allexport
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is not set. Copy .env.example to .env and edit it with your Neon DATABASE_URL." >&2
  exit 1
fi

# Check required commands
for cmd in go; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Required command '$cmd' not found. Please install it." >&2
    exit 1
  fi
done

SEEDS_DIR="seeds"
CATEGORIES_FILE="$SEEDS_DIR/products_category.json"
PRODUCTS_FILE="$SEEDS_DIR/products_product.json"
FLAVORS_FILE="$SEEDS_DIR/flavors.json"
ADDONS_FILE="$SEEDS_DIR/addons.json"

if [ ! -f "$CATEGORIES_FILE" ]; then
  echo "Missing categories seed file: $CATEGORIES_FILE" >&2
  exit 1
fi
if [ ! -f "$PRODUCTS_FILE" ]; then
  echo "Missing products seed file: $PRODUCTS_FILE" >&2
  exit 1
fi
if [ ! -f "$FLAVORS_FILE" ]; then
  echo "Missing flavors seed file: $FLAVORS_FILE" >&2
  exit 1
fi
if [ ! -f "$ADDONS_FILE" ]; then
  echo "Missing addons seed file: $ADDONS_FILE" >&2
  exit 1
fi

# Run unified Go seeder
echo "Seeding categories, products, flavors, and addons using Go seeder..."
go run ./cmd/seed \
  --categories="$CATEGORIES_FILE" \
  --products="$PRODUCTS_FILE" \
  --flavors="$FLAVORS_FILE" \
  --addons="$ADDONS_FILE"

echo "Seeding completed."
