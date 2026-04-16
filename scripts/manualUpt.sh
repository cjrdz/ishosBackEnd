#!/usr/bin/env bash
set -euo pipefail

# Controlled dependency update helper.
# Use for deliberate version bumps and always run build/tests before merging.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

APPLY=false
UPDATE_ALL=false
MODULE=""
VERSION=""
RUN_TESTS=true

usage() {
  cat <<'USAGE'
Usage:
  ./scripts/manualUpt.sh [options]

Options:
  --check-only                 Show outdated modules only (default)
  --apply                      Apply updates
  --module <module>            Update one module (requires --version)
  --version <version>          Version for --module (e.g. v0.36.9)
  --all                        Update all modules (go get -u ./...)
  --skip-tests                 Skip make test and make test-integration
  -h, --help                   Show help

Examples:
  ./scripts/manualUpt.sh
  ./scripts/manualUpt.sh --apply --module github.com/pocketbase/pocketbase --version v0.36.9
  ./scripts/manualUpt.sh --apply --all
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check-only) APPLY=false; shift ;;
    --apply) APPLY=true; shift ;;
    --module) MODULE="${2:-}"; shift 2 ;;
    --version) VERSION="${2:-}"; shift 2 ;;
    --all) UPDATE_ALL=true; shift ;;
    --skip-tests) RUN_TESTS=false; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

echo "==> Outdated modules"
go list -m -u all || true
echo

echo "==> PocketBase status"
go list -m -u github.com/pocketbase/pocketbase || true
echo

if [[ "${APPLY}" != "true" ]]; then
  echo "Check complete. Re-run with --apply to update."
  exit 0
fi

if [[ "${UPDATE_ALL}" == "true" && -n "${MODULE}" ]]; then
  echo "Use either --all OR --module/--version, not both." >&2
  exit 1
fi

if [[ "${UPDATE_ALL}" == "false" && -z "${MODULE}" ]]; then
  echo "When using --apply, provide --all OR --module + --version." >&2
  exit 1
fi

if [[ -n "${MODULE}" && -z "${VERSION}" ]]; then
  echo "--version is required when --module is provided." >&2
  exit 1
fi

if [[ "${UPDATE_ALL}" == "true" ]]; then
  echo "==> Updating all modules"
  go get -u ./...
else
  echo "==> Updating ${MODULE} to ${VERSION}"
  go get "${MODULE}@${VERSION}"
fi

echo "==> Tidying modules"
go mod tidy

echo "==> Verifying modules"
go mod verify

echo "==> Building"
go build ./...

if [[ "${RUN_TESTS}" == "true" ]]; then
  echo "==> Running tests"
  make test
  make test-integration
else
  echo "==> Skipping tests"
fi

echo "==> Done. Changed dependency files:"
git status --short go.mod go.sum || true