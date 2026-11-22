#!/usr/bin/env bash
set -Eeuo pipefail

if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
  echo "Refusing to run as root; run as a non-root user so MySQL stays confined to your temporary directory." >&2
  exit 2
fi
if [[ -n "${MYSQL_DSN:-}" || -n "${MYSQL_PWD:-}" || -n "${MYSQL_OTP_IT_DSN:-}" ]]; then
  echo "Refusing to run with inherited database credentials or a preconfigured OTP test DSN." >&2
  exit 2
fi
for tool in go mysqld mysql mysqladmin ss; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Required local tool not found: $tool" >&2
    exit 2
  fi
done
MYSQL_VERSION="$(mysqld --no-defaults --version 2>/dev/null || true)"
if [[ ! "$MYSQL_VERSION" =~ Ver[[:space:]]8\. ]]; then
  echo "Requires a locally installed MySQL 8.x server; found: ${MYSQL_VERSION:-unknown}" >&2
  exit 2
fi
REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
if [[ ! -f "$REPO_ROOT/go.mod" ]]; then
  echo "Run this script from a complete repository checkout." >&2
  exit 2
fi
cd -- "$REPO_ROOT"

TMP_ROOT=""
MYSQLD_PID=""
MYSQLD_LOG=""
RUN_USER="$(id -un)"
cleanup() {
  local status=$?
  trap - EXIT INT TERM
  if [[ -n "$MYSQLD_PID" ]] && kill -0 "$MYSQLD_PID" 2>/dev/null; then
    if [[ -S "$TMP_ROOT/mysql.sock" ]]; then
      isolated mysqladmin --no-defaults --protocol=SOCKET --socket="$TMP_ROOT/mysql.sock" --user=root shutdown >/dev/null 2>&1 || true
    fi
    for _ in {1..40}; do
      kill -0 "$MYSQLD_PID" 2>/dev/null || break
      sleep 0.25
    done
    if kill -0 "$MYSQLD_PID" 2>/dev/null; then
      kill -TERM "$MYSQLD_PID" 2>/dev/null || true
      for _ in {1..20}; do
        kill -0 "$MYSQLD_PID" 2>/dev/null || break
        sleep 0.25
      done
    fi
    if kill -0 "$MYSQLD_PID" 2>/dev/null; then
      echo "WARNING: isolated mysqld PID $MYSQLD_PID remains alive; preserving its temporary directory $TMP_ROOT." >&2
      status=1
    else
      wait "$MYSQLD_PID" 2>/dev/null || true
      echo "Stopped isolated mysqld PID $MYSQLD_PID."
    fi
  fi
  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" && ! -L "$TMP_ROOT" && -f "$TMP_ROOT/.cue-otp-mysql-owned" ]]; then
    local canonical=""
    canonical="$(cd -- "$TMP_ROOT" 2>/dev/null && pwd -P || true)"
    if [[ "$canonical" == "$TMP_ROOT" && "$canonical" == /tmp/cue-otp-mysql-* ]]; then
      if [[ -z "$MYSQLD_PID" ]] || ! kill -0 "$MYSQLD_PID" 2>/dev/null; then
        rm -rf -- "$canonical"
        echo "Removed only this run's isolated temporary directory: $canonical"
      else
        echo "WARNING: preserving temporary directory because mysqld is still running: $canonical" >&2
        status=1
      fi
    else
      echo "WARNING: temporary root failed canonical-path checks and was preserved: $TMP_ROOT" >&2
      status=1
    fi
  fi
  if [[ $status -ne 0 && -n "$MYSQLD_LOG" && -f "$MYSQLD_LOG" ]]; then
    echo "Last lines from isolated MySQL log:" >&2
    tail -n 40 "$MYSQLD_LOG" >&2 || true
  fi
  exit "$status"
}
isolated() {
  env -i \
    "PATH=$PATH" \
    "HOME=$TMP_ROOT" \
    "USER=$RUN_USER" \
    "LOGNAME=$RUN_USER" \
    "TMPDIR=$TMP_ROOT/tmp" \
    "$@"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

TMP_ROOT="$(mktemp -d /tmp/cue-otp-mysql-XXXXXXXX)"
chmod 700 "$TMP_ROOT"
if [[ "$(cd -- "$TMP_ROOT" && pwd -P)" != "$TMP_ROOT" || -L "$TMP_ROOT" ]]; then
  echo "Refusing a non-canonical temporary root." >&2
  exit 2
fi
: > "$TMP_ROOT/.cue-otp-mysql-owned"
chmod 600 "$TMP_ROOT/.cue-otp-mysql-owned"
mkdir -m 700 "$TMP_ROOT/data" "$TMP_ROOT/tmp"
if [[ -e "$TMP_ROOT/.env" ]]; then
  echo "Refusing to run with an .env file inside the temporary root." >&2
  exit 2
fi

MYSQL_SOCKET="$TMP_ROOT/mysql.sock"
MYSQLD_PIDFILE="$TMP_ROOT/mysql.pid"
MYSQLD_LOG="$TMP_ROOT/mysqld.err"
GOCACHE_DIR="$(go env GOCACHE)"
GOMODCACHE_DIR="$(go env GOMODCACHE)"
isolated mysqld --no-defaults --initialize-insecure --user="$RUN_USER" --basedir=/usr --datadir="$TMP_ROOT/data" --tmpdir="$TMP_ROOT/tmp" --secure-file-priv="$TMP_ROOT" --log-error="$TMP_ROOT/initialize.err"
isolated mysqld --no-defaults \
  --user="$RUN_USER" \
  --basedir=/usr \
  --datadir="$TMP_ROOT/data" \
  --tmpdir="$TMP_ROOT/tmp" \
  --secure-file-priv="$TMP_ROOT" \
  --socket="$MYSQL_SOCKET" \
  --pid-file="$MYSQLD_PIDFILE" \
  --skip-networking=ON \
  --skip-name-resolve \
  --mysqlx=OFF \
  --log-error="$MYSQLD_LOG" &
MYSQLD_PID=$!

ready=0
for _ in {1..80}; do
  if isolated mysqladmin --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root ping >/dev/null 2>&1; then
    ready=1
    break
  fi
  if ! kill -0 "$MYSQLD_PID" 2>/dev/null; then
    break
  fi
  sleep 0.25
done
if [[ $ready -ne 1 ]]; then
  echo "The isolated temporary MySQL instance did not become ready." >&2
  exit 1
fi

SAFETY="$(isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch --skip-column-names -e 'SELECT @@skip_networking, @@datadir, @@socket, @@transaction_isolation, VERSION()')"
IFS=$'\t' read -r SKIP_NETWORKING SERVER_DATADIR SERVER_SOCKET ISOLATION SERVER_VERSION <<< "$SAFETY"
if [[ "$SKIP_NETWORKING" != "1" || "$(cd -- "$SERVER_DATADIR" && pwd -P)" != "$TMP_ROOT/data" || "$SERVER_SOCKET" != "$MYSQL_SOCKET" || "$ISOLATION" != "REPEATABLE-READ" || "$SERVER_VERSION" != 8.* ]]; then
  echo "Refusing unexpected server settings; require MySQL 8.x, skip_networking=1, default REPEATABLE-READ, and this run's datadir/socket." >&2
  exit 1
fi
if ss -ltnpH 2>/dev/null | grep -F "pid=$MYSQLD_PID," >/dev/null; then
  echo "Refusing isolated server with a TCP listener." >&2
  exit 1
fi
SUFFIX="${TMP_ROOT##*/}"
SUFFIX="${SUFFIX//-/}"
DB_NAME="cue_otp_p1_${SUFFIX}"
isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root -e "CREATE DATABASE \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
OTP_DSN="root@unix($MYSQL_SOCKET)/$DB_NAME?parseTime=true&loc=UTC"

echo "Running synthetic OTP concurrency tests on isolated MySQL $SERVER_VERSION (default isolation $ISOLATION; Unix socket only)."
env -i \
  "PATH=$PATH" "HOME=${HOME:-$TMP_ROOT}" "TMPDIR=$TMP_ROOT/tmp" \
  "GOCACHE=$GOCACHE_DIR" "GOMODCACHE=$GOMODCACHE_DIR" \
  "GOTOOLCHAIN=local" "GOPROXY=off" "GOSUMDB=off" "GOFLAGS=-mod=readonly" \
  "MYSQL_OTP_IT_DSN=$OTP_DSN" \
  go test ./internal/service -run '^TestMySQLOTPConcurrentIssueAndConsume$' -count=1 -v
