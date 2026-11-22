#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 2
}

if [[ "$(uname -s)" != "Linux" ]]; then
  fail "This local restore test requires Linux (Unix socket and /proc process checks)."
fi
if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
  fail "Refusing to run as root; use a non-root account for the temporary MySQL instance."
fi
if [[ -n "${MYSQL_DSN:-}" || -n "${MYSQL_PWD:-}" || -n "${DB_PASSWORD:-}" ]]; then
  fail "Refusing to run while MYSQL_DSN, MYSQL_PWD, or DB_PASSWORD is set in the caller environment; the test supplies synthetic values only to its isolated child process."
fi

for tool in uname dirname age age-keygen mysqld mysql mysqladmin ss env mktemp find cmp grep tr ps tail id mkdir chmod rm cat sleep date; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    fail "Missing local dependency: $tool. This test never downloads or installs tools; provide the local executable and retry."
  fi
done

resolve_executable() {
  local requested="$1" label="$2" resolved
  resolved="$(command -v "$requested" 2>/dev/null || true)"
  if [[ -z "$resolved" || ! -f "$resolved" || ! -x "$resolved" ]]; then
    fail "Cannot resolve local $label executable '$requested'. No download or installation will be attempted."
  fi
  printf '%s' "$resolved"
}

AGE_BIN="$(resolve_executable "${AGE_BIN:-age}" age)"
AGE_KEYGEN_BIN="$(resolve_executable "${AGE_KEYGEN_BIN:-age-keygen}" age-keygen)"
AGE_VERSION="$("$AGE_BIN" --version 2>&1)" || fail "Local age executable failed its --version check: $AGE_BIN"
if [[ -z "$AGE_VERSION" ]]; then
  fail "Local age executable returned an empty version string: $AGE_BIN"
fi
MYSQL_SERVER_VERSION="$(env -i "PATH=$PATH" mysqld --no-defaults --version 2>/dev/null || true)"
MYSQL_CLIENT_VERSION="$(env -i "PATH=$PATH" mysql --no-defaults --version 2>/dev/null || true)"
if [[ ! "$MYSQL_SERVER_VERSION" =~ Ver[[:space:]]8\. || ! "$MYSQL_CLIENT_VERSION" =~ Ver[[:space:]]8\. ]]; then
  fail "Requires locally installed MySQL 8.x server and client; found server '${MYSQL_SERVER_VERSION:-unknown}', client '${MYSQL_CLIENT_VERSION:-unknown}'."
fi
MYSQLD_BIN="$(command -v mysqld)"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
BACKUP_SCRIPT="$SCRIPT_DIR/backup-mysql.sh"
[[ -f "$BACKUP_SCRIPT" && -r "$BACKUP_SCRIPT" ]] || fail "Repository backup script not found or unreadable: $BACKUP_SCRIPT"

TMP_ROOT="$(mktemp -d /tmp/cue-backup-age-restore-XXXXXXXX)"
trap 'if [[ "$TMP_ROOT" == /tmp/cue-backup-age-restore-* && -d "$TMP_ROOT" && ! -L "$TMP_ROOT" ]]; then rm -rf -- "$TMP_ROOT"; fi' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 700 "$TMP_ROOT"
: > "$TMP_ROOT/.cue-backup-age-restore-owned"
chmod 600 "$TMP_ROOT/.cue-backup-age-restore-owned"
TEST_ROOT="$TMP_ROOT/test"
MYSQL_ROOT="$TMP_ROOT/mysql"
MYSQL_DATA="$MYSQL_ROOT/data"
MYSQL_TMP="$MYSQL_ROOT/tmp"
MYSQL_SOCKET="$MYSQL_ROOT/mysql.sock"
MYSQL_PIDFILE="$MYSQL_ROOT/mysql.pid"
MYSQL_LOG="$MYSQL_ROOT/mysqld.err"
MYSQL_INIT_LOG="$MYSQL_ROOT/initialize.err"
MYSQLD_PID=""
MYSQLD_JOB_PID=""

isolated() {
  env -i \
    "PATH=$PATH" \
    "HOME=$TMP_ROOT" \
    "USER=$RUN_USER" \
    "LOGNAME=$RUN_USER" \
    "TMPDIR=$MYSQL_TMP" \
    "$@"
}

is_our_mysqld() {
  local pid="$1" cmdline identity skip_networking server_datadir server_socket server_pidfile canonical_data pid_from_file
  [[ "$pid" =~ ^[0-9]+$ && -r "/proc/$pid/cmdline" && -e "/proc/$pid/exe" ]] || return 1
  [[ "/proc/$pid/exe" -ef "$MYSQLD_BIN" ]] || return 1
  cmdline="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
  if [[ "$cmdline" == *"--datadir=$MYSQL_DATA"* &&
        "$cmdline" == *"--socket=$MYSQL_SOCKET"* &&
        "$cmdline" == *"--skip-networking=ON"* ]]; then
    return 0
  fi
  [[ -f "$MYSQL_PIDFILE" && -S "$MYSQL_SOCKET" ]] || return 1
  pid_from_file="$(cat "$MYSQL_PIDFILE" 2>/dev/null || true)"
  [[ "$pid_from_file" == "$pid" ]] || return 1
  identity="$(isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch --skip-column-names \
    -e 'SELECT @@skip_networking, @@datadir, @@socket, @@pid_file' 2>/dev/null)" || return 1
  IFS=$'\t' read -r skip_networking server_datadir server_socket server_pidfile <<< "$identity"
  canonical_data="$(cd -- "$server_datadir" 2>/dev/null && pwd -P)" || return 1
  [[ "$skip_networking" == 1 && "$canonical_data" == "$MYSQL_DATA" &&
     "$server_socket" == "$MYSQL_SOCKET" && "$server_pidfile" == "$MYSQL_PIDFILE" ]]
}

is_mysqld_zombie() {
  local state
  state="$(ps -o stat= -p "$1" 2>/dev/null || true)"
  state="${state//[[:space:]]/}"
  [[ "$state" == Z* ]]
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  local server_preserved=0 log_file

  if [[ -n "$MYSQLD_PID" ]] && kill -0 "$MYSQLD_PID" 2>/dev/null && is_our_mysqld "$MYSQLD_PID"; then
    if [[ -S "$MYSQL_SOCKET" ]]; then
      isolated mysqladmin --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root shutdown >/dev/null 2>&1 || true
    fi
    for _ in {1..40}; do
      if ! kill -0 "$MYSQLD_PID" 2>/dev/null || is_mysqld_zombie "$MYSQLD_PID"; then break; fi
      sleep 0.25
    done
    if kill -0 "$MYSQLD_PID" 2>/dev/null && ! is_mysqld_zombie "$MYSQLD_PID" && is_our_mysqld "$MYSQLD_PID"; then
      kill -TERM "$MYSQLD_PID" 2>/dev/null || true
      for _ in {1..20}; do
        if ! kill -0 "$MYSQLD_PID" 2>/dev/null || is_mysqld_zombie "$MYSQLD_PID"; then break; fi
        sleep 0.25
      done
    fi
    if kill -0 "$MYSQLD_PID" 2>/dev/null && ! is_mysqld_zombie "$MYSQLD_PID" && is_our_mysqld "$MYSQLD_PID"; then
      kill -KILL "$MYSQLD_PID" 2>/dev/null || true
      sleep 0.25
    fi
  fi

  if [[ -n "$MYSQLD_JOB_PID" ]] && { ! kill -0 "$MYSQLD_JOB_PID" 2>/dev/null || is_mysqld_zombie "$MYSQLD_JOB_PID"; }; then
    wait "$MYSQLD_JOB_PID" 2>/dev/null || true
  fi
  if [[ -n "$MYSQLD_PID" ]] && kill -0 "$MYSQLD_PID" 2>/dev/null && ! is_mysqld_zombie "$MYSQLD_PID"; then
    server_preserved=1
    status=1
    echo "WARNING: MySQL PID $MYSQLD_PID is still running or cannot be verified as stopped; preserving its temporary datadir for safety." >&2
  else
    [[ -z "$MYSQLD_PID" ]] || echo "Stopped temporary socket-only MySQL PID $MYSQLD_PID."
  fi

  if [[ $status -ne 0 ]]; then
    for log_file in "$MYSQL_LOG" "$MYSQL_INIT_LOG"; do
      if [[ -f "$log_file" ]]; then
        echo "Last lines of this run's temporary MySQL log ($log_file):" >&2
        tail -n 40 "$log_file" >&2 || true
      fi
    done
  fi

  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" && ! -L "$TMP_ROOT" && -f "$TMP_ROOT/.cue-backup-age-restore-owned" ]]; then
    local canonical=""
    canonical="$(cd -- "$TMP_ROOT" 2>/dev/null && pwd -P || true)"
    if [[ "$canonical" == "$TMP_ROOT" && "$canonical" == /tmp/cue-backup-age-restore-* ]]; then
      if [[ $server_preserved -eq 1 ]]; then
        if rm -rf -- "$TEST_ROOT"; then
          echo "Removed this run's age binaries, keys, SQL, ciphertext and decrypted test files; preserved only the live MySQL data under $MYSQL_ROOT." >&2
        else
          echo "WARNING: could not fully remove this run's test artifacts under $TEST_ROOT; MySQL data remains under $MYSQL_ROOT." >&2
          status=1
        fi
      else
        if rm -rf -- "$canonical" && [[ ! -e "$canonical" ]]; then
          echo "Removed this run's temporary age binaries, keys, SQL, ciphertext, decrypted dump and MySQL data."
        else
          echo "WARNING: could not fully remove this run's temporary directory: $canonical" >&2
          status=1
        fi
      fi
    else
      echo "WARNING: temporary root failed canonical-path checks and was preserved: $TMP_ROOT" >&2
      status=1
    fi
  fi
  exit "$status"
}

RUN_USER="$(id -un)"
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -m 700 "$TEST_ROOT" "$MYSQL_ROOT" "$MYSQL_DATA" "$MYSQL_TMP" "$TEST_ROOT/bin" "$TEST_ROOT/mockbin" "$TEST_ROOT/backup"

printf 'Local age: %s (%s)\n' "$AGE_BIN" "$AGE_VERSION"
printf 'Local MySQL server/client: %s / %s\n' "$MYSQL_SERVER_VERSION" "$MYSQL_CLIENT_VERSION"
printf 'This run uses only a synthetic mysqldump mock and a temporary socket-only MySQL datadir.\n'

# Generate a disposable age identity and public recipient; no key is read from the caller or repository.
IDENTITY="$TEST_ROOT/synthetic-identity.txt"
RECIPIENTS="$TEST_ROOT/synthetic-recipients.txt"
"$AGE_KEYGEN_BIN" -o "$IDENTITY" >/dev/null 2>&1 || fail "Local age-keygen failed to create a temporary identity."
chmod 600 "$IDENTITY"
"$AGE_KEYGEN_BIN" -y "$IDENTITY" > "$RECIPIENTS" || fail "Local age-keygen could not derive the temporary recipient."
chmod 600 "$RECIPIENTS"

PAYLOAD="synthetic-restore-canary-${BASHPID}-${RANDOM}"
SQL_FILE="$TEST_ROOT/synthetic-dump.sql"
cat > "$SQL_FILE" <<SQL
-- Synthetic restore fixture only; no business or customer data.
CREATE DATABASE restore_validation;
USE restore_validation;
CREATE TABLE restore_probe (id INT PRIMARY KEY, payload VARBINARY(96) NOT NULL);
INSERT INTO restore_probe (id, payload) VALUES (1, '$PAYLOAD');
SQL
chmod 600 "$SQL_FILE"

# The dump mock emits only the fixture above and verifies it received only the synthetic dump password.
cat > "$TEST_ROOT/mockbin/mysqldump" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ "${MYSQL_PWD-}" == 'synthetic-only-mysqldump-password' ]] || { echo 'synthetic mysqldump mock did not receive its expected synthetic password' >&2; exit 65; }
printf '%s\n' 'called' > "$MOCK_DUMP_MARKER"
cat -- "$SYNTHETIC_SQL_FILE"
MOCK
chmod 700 "$TEST_ROOT/mockbin/mysqldump"

# Guard the environment at the age boundary, then exec the real locally installed age binary.
AGE_GUARD="$TEST_ROOT/bin/age-credential-guard"
cat > "$AGE_GUARD" <<'AGEGUARD'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${MYSQL_PWD+x} ]]; then echo 'MYSQL_PWD reached the age process boundary' >&2; exit 70; fi
if [[ ${DB_PASSWORD+x} ]]; then echo 'DB_PASSWORD reached the age process boundary' >&2; exit 71; fi
real_age="${AGE_REAL_BIN:?local real age path is required}"
unset AGE_REAL_BIN
exec "$real_age" "$@"
AGEGUARD
chmod 700 "$AGE_GUARD"
MOCK_DUMP_MARKER="$TEST_ROOT/mysqldump-called"

# Run the repository Bash backup script with a clean environment and the mock first in PATH.
env -i \
  "PATH=$TEST_ROOT/mockbin:$PATH" \
  "HOME=$TEST_ROOT" "USER=$RUN_USER" "LOGNAME=$RUN_USER" "TMPDIR=$TEST_ROOT" \
  "AGE_BIN=$AGE_GUARD" "AGE_REAL_BIN=$AGE_BIN" "AGE_RECIPIENTS_FILE=$RECIPIENTS" "BACKUP_DIR=$TEST_ROOT/backup" \
  DB_NAME=synthetic_restore_db DB_USER=synthetic_test_user DB_HOST=127.0.0.1 \
  DB_PASSWORD=synthetic-only-mysqldump-password MYSQL_PWD=preexisting-synthetic-value \
  "SYNTHETIC_SQL_FILE=$SQL_FILE" "MOCK_DUMP_MARKER=$MOCK_DUMP_MARKER" \
  bash "$BACKUP_SCRIPT"
[[ -f "$MOCK_DUMP_MARKER" ]] || fail "Backup script did not invoke the synthetic mysqldump mock."

mapfile -t BACKUP_ITEMS < <(find "$TEST_ROOT/backup" -mindepth 1 -maxdepth 1 -print)
if [[ "${#BACKUP_ITEMS[@]}" -ne 1 ]]; then
  fail "Backup directory must contain exactly one published .sql.age file."
fi
if [[ ! -f "${BACKUP_ITEMS[0]}" || "${BACKUP_ITEMS[0]}" != "$TEST_ROOT/backup/"*.sql.age ]]; then
  fail "Backup directory must contain exactly one published .sql.age file."
fi
CIPHERTEXT="${BACKUP_ITEMS[0]}"
if grep --binary-files=text -Fq "$PAYLOAD" "$CIPHERTEXT"; then
  fail "Synthetic plaintext canary was found in the encrypted artifact."
fi
if [[ -n "$(find "$TEST_ROOT/backup" -mindepth 1 \( -name '*.sql' -o -name '.*.tmp' \) -print -quit)" ]]; then
  fail "Plaintext SQL or a staging file remained in the backup directory."
fi

DECRYPTED="$TEST_ROOT/decrypted.sql"
"$AGE_BIN" --decrypt -i "$IDENTITY" "$CIPHERTEXT" > "$DECRYPTED" || fail "Real age could not decrypt the synthetic backup."
cmp -s "$SQL_FILE" "$DECRYPTED" || fail "Decrypted SQL differs byte-for-byte from the synthetic dump."
printf 'PASS: backup script encrypted with real age; decrypted bytes match the synthetic SQL exactly; only ciphertext remains in the backup directory.\n'

# Start a brand-new MySQL 8 server with no option files, no inherited environment, no TCP and no X listener.
env -i \
  "PATH=$PATH" "HOME=$TMP_ROOT" "USER=$RUN_USER" "LOGNAME=$RUN_USER" "TMPDIR=$MYSQL_TMP" \
  mysqld --no-defaults --initialize-insecure --user="$RUN_USER" --basedir=/usr \
    --datadir="$MYSQL_DATA" --tmpdir="$MYSQL_TMP" --secure-file-priv="$MYSQL_ROOT" \
    --log-error="$MYSQL_INIT_LOG"
env -i \
  "PATH=$PATH" "HOME=$TMP_ROOT" "USER=$RUN_USER" "LOGNAME=$RUN_USER" "TMPDIR=$MYSQL_TMP" \
  mysqld --no-defaults --user="$RUN_USER" --basedir=/usr \
    --datadir="$MYSQL_DATA" --tmpdir="$MYSQL_TMP" --secure-file-priv="$MYSQL_ROOT" \
    --socket="$MYSQL_SOCKET" --pid-file="$MYSQL_PIDFILE" --skip-networking=ON \
    --skip-name-resolve --mysqlx=OFF --local-infile=OFF --log-error="$MYSQL_LOG" &
MYSQLD_JOB_PID=$!
MYSQLD_PID="$MYSQLD_JOB_PID"

ready=0
for _ in {1..120}; do
  if isolated mysqladmin --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root ping >/dev/null 2>&1; then
    ready=1
    break
  fi
  if ! kill -0 "$MYSQLD_JOB_PID" 2>/dev/null; then break; fi
  sleep 0.25
done
[[ "$ready" -eq 1 ]] || fail "Temporary MySQL 8 socket-only server did not become ready; see its run-specific log above."
[[ -f "$MYSQL_PIDFILE" ]] || fail "Temporary MySQL did not create its PID file."
PID_FROM_FILE="$(cat "$MYSQL_PIDFILE")"
[[ "$PID_FROM_FILE" =~ ^[0-9]+$ ]] || fail "Temporary MySQL PID file has an invalid value."
MYSQLD_PID="$PID_FROM_FILE"
if [[ "$MYSQLD_PID" != "$MYSQLD_JOB_PID" ]]; then
  fail "Temporary MySQL PID does not match the foreground process; refusing unsafe cleanup."
fi
is_our_mysqld "$MYSQLD_PID" || fail "Temporary MySQL process identity did not match its run-specific datadir/socket."

SAFETY="$(isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch --skip-column-names \
  -e 'SELECT @@skip_networking, @@datadir, @@socket, VERSION()')"
IFS=$'\t' read -r skip_networking server_datadir server_socket server_version <<< "$SAFETY"
canonical_data="$(cd -- "$server_datadir" && pwd -P)"
if [[ "$skip_networking" != 1 || "$canonical_data" != "$MYSQL_DATA" || "$server_socket" != "$MYSQL_SOCKET" || "$server_version" != 8.* ]]; then
  fail "Temporary MySQL safety check failed (require MySQL 8, skip_networking=1, this datadir and this Unix socket)."
fi
TCP_LISTENERS="$(ss -ltnpH 2>/dev/null || true)"
if [[ "$TCP_LISTENERS" == *"pid=$MYSQLD_PID,"* ]]; then
  fail "Temporary MySQL unexpectedly opened a TCP listener."
fi
printf 'Verified isolated MySQL %s: skip_networking=1, private datadir, Unix socket only.\n' "$server_version"

# Import only the decrypted synthetic fixture through the private Unix socket; verify the exact row and count.
isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root < "$DECRYPTED"
RESTORED_ROW="$(isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch --raw --skip-column-names \
  -e 'SELECT id, CAST(payload AS CHAR) FROM restore_validation.restore_probe ORDER BY id')"
EXPECTED_ROW="$(printf '1\t%s' "$PAYLOAD")"
[[ "$RESTORED_ROW" == "$EXPECTED_ROW" ]] || fail "Restored record does not exactly match the expected synthetic row."
ROW_COUNT="$(isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch --skip-column-names \
  -e 'SELECT COUNT(*) FROM restore_validation.restore_probe')"
[[ "$ROW_COUNT" == 1 ]] || fail "Expected exactly one restored synthetic row; found '$ROW_COUNT'."
printf 'PASS: decrypted dump imported via Unix socket; exact query matched the synthetic payload and found one row.\n'
