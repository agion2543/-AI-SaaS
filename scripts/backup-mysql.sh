#!/usr/bin/env bash
set -euo pipefail
umask 077

DB_NAME="${DB_NAME:-saas_billing}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_HOST="${DB_HOST:-127.0.0.1}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
AGE_BIN="${AGE_BIN:-age}"
AGE_RECIPIENTS_FILE="${AGE_RECIPIENTS_FILE:-}"

if [[ -z "$AGE_RECIPIENTS_FILE" || ! -f "$AGE_RECIPIENTS_FILE" || ! -s "$AGE_RECIPIENTS_FILE" ]]; then
  echo "AGE_RECIPIENTS_FILE must name a non-empty age recipients file" >&2
  exit 1
fi
if ! command -v "$AGE_BIN" >/dev/null 2>&1; then
  echo "age executable not found: $AGE_BIN" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="$BACKUP_DIR/${DB_NAME}-${STAMP}.sql.age"
if [[ -e "$OUT" || -L "$OUT" ]]; then
  echo "Refusing to overwrite existing backup: $OUT" >&2
  exit 1
fi
TMP_SQL="$(mktemp "$BACKUP_DIR/.${DB_NAME}-${STAMP}.XXXXXX.tmp")"
TMP_AGE=""
cleanup() {
  if [[ -n "$TMP_SQL" ]]; then rm -f -- "$TMP_SQL"; fi
  if [[ -n "$TMP_AGE" ]]; then rm -f -- "$TMP_AGE"; fi
}
trap cleanup EXIT
TMP_AGE="$(mktemp "$BACKUP_DIR/.${DB_NAME}-${STAMP}.XXXXXX.tmp")"

MYSQLDUMP=(
  mysqldump
  -h "$DB_HOST"
  -u "$DB_USER"
  --default-character-set=utf8mb4
  --single-transaction
  --routines
  --triggers
  "$DB_NAME"
)
DUMP_STATUS=0
if [[ -n "$DB_PASSWORD" ]]; then
  MYSQL_PWD="$DB_PASSWORD" "${MYSQLDUMP[@]}" > "$TMP_SQL" || DUMP_STATUS=$?
else
  "${MYSQLDUMP[@]}" > "$TMP_SQL" || DUMP_STATUS=$?
fi
if (( DUMP_STATUS != 0 )); then
  printf 'ERROR: mysqldump failed with exit code %d\n' "$DUMP_STATUS" >&2
  exit "$DUMP_STATUS"
fi
if [[ ! -s "$TMP_SQL" ]]; then
  echo "Refusing to encrypt an empty database dump: $OUT" >&2
  exit 1
fi

# age writes binary output to stdout; direct redirection avoids text conversion.
# Remove both database-password variables before starting the unrelated age process.
AGE_STATUS=0
env -u MYSQL_PWD -u DB_PASSWORD "$AGE_BIN" -R "$AGE_RECIPIENTS_FILE" "$TMP_SQL" > "$TMP_AGE" || AGE_STATUS=$?
if (( AGE_STATUS != 0 )); then
  printf 'ERROR: age encryption failed with exit code %d\n' "$AGE_STATUS" >&2
  exit "$AGE_STATUS"
fi
if [[ ! -s "$TMP_AGE" ]]; then
  echo "Refusing to publish an empty encrypted backup: $OUT" >&2
  exit 1
fi
# Remove the private plaintext staging file before publishing ciphertext.
rm -f -- "$TMP_SQL"
TMP_SQL=""

# Same-directory rename is atomic. -n plus the source check prevents a race
# from replacing an existing backup created after the initial collision check.
mv -n -- "$TMP_AGE" "$OUT"
if [[ -e "$TMP_AGE" || -L "$TMP_AGE" || ! -s "$OUT" ]]; then
  echo "Refusing to overwrite or publish an invalid encrypted backup: $OUT" >&2
  exit 1
fi
TMP_AGE=""
echo "Encrypted backup created: $OUT"
