#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

# Load project env if present
if [[ -f ".env" ]]; then
  # shellcheck disable=SC1091
  set -o allexport
  source .env
  set +o allexport
fi

EMAIL="${BOOTSTRAP_ADMIN_EMAIL:-}"
PASSWORD="${BOOTSTRAP_ADMIN_PASSWORD:-}"
NAME="${BOOTSTRAP_ADMIN_NAME:-Admin}"
PHONE="${BOOTSTRAP_ADMIN_PHONE:-}"

if [[ -z "${EMAIL}" ]]; then
  read -r -p "Admin email: " EMAIL
fi

if [[ -z "${PASSWORD}" ]]; then
  read -r -s -p "Admin password: " PASSWORD
  echo ""
fi

if [[ -z "${EMAIL}" || -z "${PASSWORD}" ]]; then
  echo "Admin email and password are required." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Required command 'go' not found. Please install Go first." >&2
  exit 1
fi

echo "Creating first admin user..."
go run ./cmd/bootstrap -email "${EMAIL}" -password "${PASSWORD}" -name "${NAME}" -phone "${PHONE}"
echo "Done. You can now log in at /api/v1/auth/login and create more users from the admin UI."
