#!/usr/bin/env bash
# scripts/clean_seeds.sh
#
# Remove all rows from tables populated by the seeder:
# - order_items
# - orders
# - rate_limits
# - products
# - categories
#
# Usage:
#   chmod +x scripts/clean_seeds.sh
#   ./scripts/clean_seeds.sh          # interactive confirmation
#   ./scripts/clean_seeds.sh --yes    # run without prompting
#   ./scripts/clean_seeds.sh --dry-run  # show SQL that would be executed but don't run
#
# Notes:
# - This script executes DELETE statements against the DATABASE_URL in your environment.
# - It removes all data from the tables listed above.
# - Deletions are executed in this order: order_items -> orders -> rate_limits -> products -> categories.
# - If foreign key constraints prevent deletion, the script will show the DB error and stop.
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

# Load .env if present (non-invasive)
if [ -f ".env" ]; then
  # shellcheck disable=SC1090,SC1091
  set -o allexport
  source .env
  set +o allexport
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is not set. Set it in the environment or in a .env file in the project root." >&2
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "Required command 'psql' not found. Please install PostgreSQL client tools." >&2
  exit 1
fi

# Flags
FORCE=0
DRY_RUN=0

for arg in "$@"; do
  case "$arg" in
    -y|--yes|--force) FORCE=1 ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help)
      cat <<'USAGE'
Usage: clean_seeds.sh [--dry-run] [--yes]
  --dry-run   Print SQL that would be executed, do not run anything.
  --yes       Do not prompt for confirmation; run immediately.
  -h, --help  Show this help.
USAGE
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

run_sql() {
  # run SQL via psql using DATABASE_URL; prints output
  local sql="$1"
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "-- DRY-RUN: $sql"
  else
    # Use ON_ERROR_STOP to make psql return non-zero on error and stop at first error
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "$sql"
  fi
}

confirm() {
  if [ "$FORCE" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
    return 0
  fi
  echo "This will DELETE ALL rows from tables seeded by this project in the database pointed to by DATABASE_URL."
  echo "Target tables: order_items, orders, rate_limits, products, categories"
  printf "Continue? [y/N]: "
  read -r ans
  case "${ans,,}" in
    y|yes) return 0 ;;
    *) echo "Aborted."; exit 1 ;;
  esac
}

# Confirm with user
confirm

echo "Beginning cleanup of seeded rows..."

# 1) Delete all order_items
echo "-> Deleting all order_items..."
sql="DELETE FROM order_items;"
run_sql "$sql"

# 2) Delete all orders
echo "-> Deleting all orders..."
sql="DELETE FROM orders;"
run_sql "$sql"

# 3) Delete all rate_limits
echo "-> Deleting all rate_limits..."
sql="DELETE FROM rate_limits;"
run_sql "$sql"

# 4) Delete all products
echo "-> Deleting all products..."
sql="DELETE FROM products;"
run_sql "$sql"

# 5) Delete all categories
echo "-> Deleting all categories..."
sql="DELETE FROM categories;"
run_sql "$sql"

echo "Cleanup finished."

if [ "$DRY_RUN" -eq 1 ]; then
  echo "Note: dry-run mode; no changes were made."
fi

exit 0
