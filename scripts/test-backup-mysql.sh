#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_SCRIPT="$SCRIPT_DIR/backup-mysql.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/cue-backup-test.XXXXXX")"
trap 'rm -rf -- "$TEST_ROOT"' EXIT
MOCK_BIN="$TEST_ROOT/mock-bin"
RECIPIENTS_FILE="$TEST_ROOT/synthetic-recipients.txt"
SYNTHETIC_DB_PASSWORD='synthetic-only-password-do-not-use'
mkdir -p "$MOCK_BIN"
printf '%s\n' 'age1syntheticrecipientnotarealkey000000000000000000000000000000000000000000000000000000000000' > "$RECIPIENTS_FILE"

cat > "$MOCK_BIN/date" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '20261120-120000'
EOF

cat > "$MOCK_BIN/mysqldump" <<'EOF'
#!/usr/bin/env bash
if [[ -n "${MOCK_DUMP_ARGS_FILE:-}" ]]; then printf '%s\n' "$@" > "$MOCK_DUMP_ARGS_FILE"; fi
if [[ -n "${MOCK_DUMP_PASSWORD_FILE:-}" ]]; then printf '%s' "${MYSQL_PWD-unset}" > "$MOCK_DUMP_PASSWORD_FILE"; fi
case "${MOCK_DUMP_MODE:-success}" in
  success)
    cat <<'SQL'
-- synthetic isolated backup fixture; not customer data
CREATE TABLE `backup_probe` (`id` int NOT NULL);
INSERT INTO `backup_probe` VALUES (1);
SQL
    ;;
  empty)
    exit 0
    ;;
  partial-failure)
    printf '%s\n' '-- partial synthetic output'
    exit 42
    ;;
  *)
    echo "unknown MOCK_DUMP_MODE" >&2
    exit 64
    ;;
esac
EOF

cat > "$MOCK_BIN/age" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ "$#" -eq 3 && "$1" == '-R' && "$2" == "$AGE_RECIPIENTS_FILE" ]] || {
  echo 'unexpected synthetic age arguments' >&2
  exit 65
}
input="$3"
[[ -s "$input" ]] || { echo 'age input was empty' >&2; exit 66; }
if [[ -n "${MOCK_AGE_ARGS_FILE:-}" ]]; then
  printf '%s\n' "$@" > "$MOCK_AGE_ARGS_FILE"
  printf 'MYSQL_PWD=%s\n' "${MYSQL_PWD+set}" >> "$MOCK_AGE_ARGS_FILE"
  printf 'DB_PASSWORD=%s\n' "${DB_PASSWORD+set}" >> "$MOCK_AGE_ARGS_FILE"
fi
case "${MOCK_AGE_MODE:-success}" in
  success)
    printf '%s\n' 'SYNTHETIC-AGE-MOCK-NOT-REAL-ENCRYPTION'
    base64 < "$input"
    ;;
  empty)
    exit 0
    ;;
  partial-failure)
    printf '%s\n' 'partial synthetic age output'
    exit 43
    ;;
  *)
    echo "unknown MOCK_AGE_MODE" >&2
    exit 64
    ;;
esac
EOF
chmod 700 "$MOCK_BIN/date" "$MOCK_BIN/mysqldump" "$MOCK_BIN/age"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

run_backup() {
  local mode="$1" backup_dir="$2" log_dir="$3"
  mkdir -p "$backup_dir" "$log_dir"
  PATH="$MOCK_BIN:$PATH" \
    MOCK_DUMP_MODE="$mode" \
    MOCK_AGE_MODE="${MOCK_AGE_MODE:-success}" \
    MOCK_DUMP_ARGS_FILE="$log_dir/mysqldump-args" \
    MOCK_DUMP_PASSWORD_FILE="$log_dir/mysqldump-password" \
    MOCK_AGE_ARGS_FILE="$log_dir/age-args" \
    AGE_BIN=age \
    AGE_RECIPIENTS_FILE="$RECIPIENTS_FILE" \
    BACKUP_DIR="$backup_dir" \
    DB_NAME=synthetic_test_db \
    DB_USER=synthetic_test_user \
    DB_HOST=127.0.0.1 \
    DB_PASSWORD="$SYNTHETIC_DB_PASSWORD" \
    MYSQL_PWD=preexisting-synthetic-password \
    bash "$BACKUP_SCRIPT" >"$log_dir/stdout" 2>"$log_dir/stderr"
}

assert_no_temporary_files() {
  local dir="$1"
  if find "$dir" -maxdepth 1 -type f -name '.*.tmp' -print -quit | grep -q .; then
    fail "temporary output was left in $dir"
  fi
}

# Success: only a synthetic ciphertext marker is published, with private mode.
SUCCESS_DIR="$TEST_ROOT/success path with spaces"
SUCCESS_LOG="$TEST_ROOT/log-success"
run_backup success "$SUCCESS_DIR" "$SUCCESS_LOG" || fail 'synthetic successful encrypted dump was rejected'
SUCCESS_FILE="$SUCCESS_DIR/synthetic_test_db-20261120-120000.sql.age"
grep -Fqx -- "Encrypted backup created: $SUCCESS_FILE" "$SUCCESS_LOG/stdout" || fail 'success status was not visible in stdout'
[[ -s "$SUCCESS_FILE" ]] || fail 'successful encrypted dump was not published as a non-empty file'
grep -q 'SYNTHETIC-AGE-MOCK-NOT-REAL-ENCRYPTION' "$SUCCESS_FILE" || fail 'published file did not contain the synthetic age marker'
if grep -q 'CREATE TABLE' "$SUCCESS_FILE"; then fail 'plaintext SQL appeared in the published artifact'; fi
[[ "$(cat "$SUCCESS_LOG/mysqldump-password")" == "$SYNTHETIC_DB_PASSWORD" ]] || fail 'mysqldump did not receive the synthetic password through MYSQL_PWD'
if grep -q "$SYNTHETIC_DB_PASSWORD" "$SUCCESS_LOG/mysqldump-args"; then fail 'password appeared in mysqldump command-line arguments'; fi
if grep -q "$SYNTHETIC_DB_PASSWORD" "$SUCCESS_LOG/age-args"; then fail 'password appeared in age command-line arguments'; fi
grep -q '^MYSQL_PWD=$' "$SUCCESS_LOG/age-args" || fail 'age received MYSQL_PWD in its environment'
grep -q '^DB_PASSWORD=$' "$SUCCESS_LOG/age-args" || fail 'age received DB_PASSWORD in its environment'
[[ "$(stat -c '%a' "$SUCCESS_FILE")" == 600 ]] || fail 'published file permissions were not 0600'
assert_no_temporary_files "$SUCCESS_DIR"

# Empty dumps, failed dumps, and failed/empty encryption must fail closed.
EMPTY_DIR="$TEST_ROOT/empty"
EMPTY_LOG="$TEST_ROOT/log-empty"
if run_backup empty "$EMPTY_DIR" "$EMPTY_LOG"; then fail 'empty dump unexpectedly succeeded'; fi
[[ ! -e "$EMPTY_DIR/synthetic_test_db-20261120-120000.sql.age" ]] || fail 'empty dump was published'
assert_no_temporary_files "$EMPTY_DIR"

PARTIAL_DIR="$TEST_ROOT/partial"
PARTIAL_LOG="$TEST_ROOT/log-partial"
PARTIAL_STATUS=0
if run_backup partial-failure "$PARTIAL_DIR" "$PARTIAL_LOG"; then fail 'failing dump command unexpectedly succeeded'; else PARTIAL_STATUS=$?; fi
[[ "$PARTIAL_STATUS" -eq 42 ]] || fail "dump exit status was not preserved (expected 42, got $PARTIAL_STATUS)"
grep -Fqx -- 'ERROR: mysqldump failed with exit code 42' "$PARTIAL_LOG/stderr" || fail 'dump failure status omitted its stage and exit code'
[[ ! -e "$PARTIAL_DIR/synthetic_test_db-20261120-120000.sql.age" ]] || fail 'partial dump was published'
assert_no_temporary_files "$PARTIAL_DIR"

AGE_FAILURE_DIR="$TEST_ROOT/age-failure"
AGE_FAILURE_LOG="$TEST_ROOT/log-age-failure"
AGE_FAILURE_STATUS=0
if MOCK_AGE_MODE=partial-failure run_backup success "$AGE_FAILURE_DIR" "$AGE_FAILURE_LOG"; then fail 'failing age command unexpectedly succeeded'; else AGE_FAILURE_STATUS=$?; fi
[[ "$AGE_FAILURE_STATUS" -eq 43 ]] || fail "age exit status was not preserved (expected 43, got $AGE_FAILURE_STATUS)"
grep -Fqx -- 'ERROR: age encryption failed with exit code 43' "$AGE_FAILURE_LOG/stderr" || fail 'age failure status omitted its stage and exit code'
[[ ! -e "$AGE_FAILURE_DIR/synthetic_test_db-20261120-120000.sql.age" ]] || fail 'partially encrypted output was published'
assert_no_temporary_files "$AGE_FAILURE_DIR"

AGE_EMPTY_DIR="$TEST_ROOT/age-empty"
AGE_EMPTY_LOG="$TEST_ROOT/log-age-empty"
if MOCK_AGE_MODE=empty run_backup success "$AGE_EMPTY_DIR" "$AGE_EMPTY_LOG"; then fail 'empty age output unexpectedly succeeded'; fi
[[ ! -e "$AGE_EMPTY_DIR/synthetic_test_db-20261120-120000.sql.age" ]] || fail 'empty encrypted output was published'
assert_no_temporary_files "$AGE_EMPTY_DIR"

# A pre-existing encrypted destination must remain byte-for-byte unchanged.
COLLISION_DIR="$TEST_ROOT/collision"
COLLISION_LOG="$TEST_ROOT/log-collision"
mkdir -p "$COLLISION_DIR" "$COLLISION_LOG"
COLLISION_FILE="$COLLISION_DIR/synthetic_test_db-20261120-120000.sql.age"
printf '%s\n' 'preserve existing synthetic encrypted file' > "$COLLISION_FILE"
if run_backup success "$COLLISION_DIR" "$COLLISION_LOG"; then fail 'existing destination was unexpectedly overwritten'; fi
[[ "$(cat "$COLLISION_FILE")" == 'preserve existing synthetic encrypted file' ]] || fail 'existing destination changed'
assert_no_temporary_files "$COLLISION_DIR"

printf '%s\n' 'PASS: synthetic mysqldump/age flow, ciphertext publication, failure cleanup, permissions, and no-overwrite checks'
