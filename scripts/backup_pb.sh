#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PB_DIR="${ROOT_DIR}/pb_data"
BACKUP_DIR="${PB_DIR}/backups"
TS="$(date +%Y%m%d_%H%M%S)"
ARCHIVE="${BACKUP_DIR}/pb_data_backup_${TS}.tar.gz"

if [[ ! -d "${PB_DIR}" ]]; then
  echo "pb_data directory not found at ${PB_DIR}"
  exit 1
fi

mkdir -p "${BACKUP_DIR}"

tar -czf "${ARCHIVE}" \
  --exclude='backups' \
  -C "${PB_DIR}" .

echo "Backup created: ${ARCHIVE}"
