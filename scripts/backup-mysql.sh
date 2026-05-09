#!/usr/bin/env bash
set -euo pipefail

DB_NAME="${DB_NAME:-saas_billing}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_HOST="${DB_HOST:-127.0.0.1}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"

mkdir -p "$BACKUP_DIR"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="$BACKUP_DIR/${DB_NAME}-${STAMP}.sql"

if [[ -n "$DB_PASSWORD" ]]; then
  MYSQL_PWD="$DB_PASSWORD" mysqldump \
    -h "$DB_HOST" \
    -u "$DB_USER" \
    --default-character-set=utf8mb4 \
    --single-transaction \
    --routines \
    --triggers \
    "$DB_NAME" > "$OUT"
else
  mysqldump \
    -h "$DB_HOST" \
    -u "$DB_USER" \
    --default-character-set=utf8mb4 \
    --single-transaction \
    --routines \
    --triggers \
    "$DB_NAME" > "$OUT"
fi

echo "Backup created: $OUT"
