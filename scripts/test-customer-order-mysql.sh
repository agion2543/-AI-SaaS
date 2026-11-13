#!/usr/bin/env bash
set -Eeuo pipefail

if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
  echo "Refusing to run as root; use a non-root account so mysqld is confined to your own temporary directory." >&2
  exit 2
fi
if [[ -n "${MYSQL_DSN:-}" || -n "${MYSQL_PWD:-}" ]]; then
  echo "Refusing to run while MYSQL_DSN or MYSQL_PWD is set; this test never uses application/production credentials." >&2
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
if [[ ! -f "$REPO_ROOT/go.mod" || ! -f "$REPO_ROOT/internal/router/customer_order_mysql_integration_test.go" ]]; then
  echo "Run this command from a complete repository checkout." >&2
  exit 2
fi
cd -- "$REPO_ROOT"

TMP_ROOT=""
MYSQLD_PID=""
MYSQLD_LOG=""
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
      echo "WARNING: isolated mysqld PID $MYSQLD_PID did not stop; preserving its temporary datadir at $TMP_ROOT." >&2
      status=1
    else
      wait "$MYSQLD_PID" 2>/dev/null || true
      echo "Stopped isolated mysqld PID $MYSQLD_PID."
    fi
  fi

  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" && ! -L "$TMP_ROOT" && -f "$TMP_ROOT/.cue-customer-order-it-owned" ]]; then
    local canonical=""
    canonical="$(cd -- "$TMP_ROOT" 2>/dev/null && pwd -P || true)"
    if [[ "$canonical" == "$TMP_ROOT" && "$canonical" == /tmp/cue-customer-order-mysql-* ]]; then
      if [[ -z "$MYSQLD_PID" ]] || ! kill -0 "$MYSQLD_PID" 2>/dev/null; then
        rm -rf -- "$canonical"
        echo "Removed only this run's isolated temporary directory: $canonical"
      else
        echo "WARNING: preserving temporary directory because its mysqld is still running: $canonical" >&2
        status=1
      fi
    else
      echo "WARNING: temporary root failed canonical-path checks and was preserved: $TMP_ROOT" >&2
      status=1
    fi
  fi
  if [[ $status -ne 0 && -n "$MYSQLD_LOG" && -f "$MYSQLD_LOG" ]]; then
    echo "Last lines from the isolated server log:" >&2
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

TMP_ROOT="$(mktemp -d /tmp/cue-customer-order-mysql-XXXXXXXX)"
chmod 700 "$TMP_ROOT"
if [[ "$(cd -- "$TMP_ROOT" && pwd -P)" != "$TMP_ROOT" || -L "$TMP_ROOT" ]]; then
  echo "Refusing a non-canonical temporary root." >&2
  exit 2
fi
: > "$TMP_ROOT/.cue-customer-order-it-owned"
chmod 600 "$TMP_ROOT/.cue-customer-order-it-owned"
mkdir -m 700 "$TMP_ROOT/data" "$TMP_ROOT/tmp"
if [[ -e "$TMP_ROOT/.env" ]]; then
  echo "Refusing to run with an .env file inside the temporary root." >&2
  exit 2
fi

RUN_USER="$(id -un)"
MYSQL_SOCKET="$TMP_ROOT/mysql.sock"
MYSQLD_PIDFILE="$TMP_ROOT/mysql.pid"
MYSQLD_LOG="$TMP_ROOT/mysqld.err"
DB_NAME="cue_customer_order_it_$(date +%s)_${RANDOM}"
GOCACHE_DIR="$(go env GOCACHE)"
GOMODCACHE_DIR="$(go env GOMODCACHE)"

echo "Preparing one-off socket-only MySQL 8 integration database."
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
  echo "The newly initialized socket-only MySQL instance did not become ready." >&2
  exit 1
fi

SAFETY="$(isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch --skip-column-names -e 'SELECT @@skip_networking, @@datadir, @@socket, VERSION()')"
IFS=$'\t' read -r SKIP_NETWORKING SERVER_DATADIR SERVER_SOCKET SERVER_VERSION <<< "$SAFETY"
if [[ "$SKIP_NETWORKING" != "1" || "$(cd -- "$SERVER_DATADIR" && pwd -P)" != "$TMP_ROOT/data" || "$SERVER_SOCKET" != "$MYSQL_SOCKET" || "$SERVER_VERSION" != 8.* ]]; then
  echo "Refusing unexpected server settings; require MySQL 8.x, skip_networking=1, and this run's datadir/socket." >&2
  exit 1
fi
if ss -ltnpH 2>/dev/null | grep -F "pid=$MYSQLD_PID," >/dev/null; then
  echo "Refusing isolated server with a TCP listener." >&2
  exit 1
fi
if [[ ! "$DB_NAME" =~ ^cue_customer_order_it_[0-9]+_[0-9]+$ ]]; then
  echo "Refusing a non-synthetic database name." >&2
  exit 1
fi
isolated mysql --no-defaults --protocol=SOCKET --socket="$MYSQL_SOCKET" --user=root --batch -e "CREATE DATABASE \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"

CUSTOMER_ORDER_DSN="root@unix($MYSQL_SOCKET)/$DB_NAME?parseTime=true&loc=UTC"
echo "Running the mobile customer-order API integration test against disposable database $DB_NAME."
env -i \
  "PATH=$PATH" "HOME=${HOME:-$TMP_ROOT}" "TMPDIR=$TMP_ROOT/tmp" \
  "GOCACHE=$GOCACHE_DIR" "GOMODCACHE=$GOMODCACHE_DIR" \
  "GOTOOLCHAIN=local" "GOPROXY=off" "GOSUMDB=off" "GOFLAGS=-mod=readonly" \
  "MYSQL_CUSTOMER_ORDER_IT_DSN=$CUSTOMER_ORDER_DSN" \
  "MYSQL_CUSTOMER_ORDER_IT_ROOT=$TMP_ROOT" \
  go test ./internal/router -run '^TestMySQLMobileCustomerOrderAPIContract$' -count=1 -v
