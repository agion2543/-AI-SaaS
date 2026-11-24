#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd -P)"
HOME_DIR="${HOME:-/home/ubuntu}"
SAFE_PATH="${PATH:-/usr/bin:/bin}"
WITH_AGE_RESTORE=0
WITH_SCHEMA_LIFECYCLE=0

usage() {
  cat <<'USAGE'
Usage: bash scripts/test-local-regression.sh [--with-age-restore] [--with-schema-lifecycle]

Runs offline Go tests/build/vet, all web/src Node tests, a temporary-output
Vite build, shell syntax checks, and synthetic backup tests. The optional
age/MySQL restore drill is attempted only when explicitly requested and its
local safety prerequisites are satisfied; missing prerequisites are reported
as SKIP. The optional schema lifecycle drill runs only when explicitly
requested and local MySQL 8 prerequisites are available. No dependencies are
installed or downloaded.
USAGE
}

while (($#)); do
  case "$1" in
    --with-age-restore) WITH_AGE_RESTORE=1 ;;
    --with-schema-lifecycle) WITH_SCHEMA_LIFECYCLE=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

fail() {
  echo "ERROR: $*" >&2
  exit 2
}

[[ "$(uname -s)" == "Linux" ]] || fail "This baseline uses Linux/GNU shell test helpers; no tests were run."
for tool in bash go node python3 env find sort mktemp rm; do
  command -v "$tool" >/dev/null 2>&1 || fail "Required local tool not found: $tool (nothing will be installed)."
done
[[ -x "$REPO_ROOT/web/node_modules/.bin/vite" ]] || fail "Local Vite dependency is missing at web/node_modules/.bin/vite; install nothing for this run and retry after dependencies are already available."

TMP_PARENT="${TMPDIR:-/tmp}"
[[ -d "$TMP_PARENT" && ! -L "$TMP_PARENT" ]] || fail "TMPDIR must be an existing, non-symlink directory."
TMP_PARENT="$(cd -- "$TMP_PARENT" && pwd -P)"
TMP_ROOT="$(mktemp -d "$TMP_PARENT/cue-local-regression.XXXXXXXX")"
chmod 700 "$TMP_ROOT"
: > "$TMP_ROOT/.cue-local-regression-owned"
chmod 600 "$TMP_ROOT/.cue-local-regression-owned"
TMP_WORK="$TMP_ROOT/work-tmp"
mkdir -m 700 "$TMP_WORK" "$TMP_ROOT/web-dist"

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  if [[ -n "${TMP_ROOT:-}" && -d "$TMP_ROOT" && ! -L "$TMP_ROOT" && -f "$TMP_ROOT/.cue-local-regression-owned" ]]; then
    local canonical=""
    canonical="$(cd -- "$TMP_ROOT" 2>/dev/null && pwd -P || true)"
    if [[ "$canonical" == "$TMP_ROOT" && "$canonical" == "$TMP_PARENT"/cue-local-regression.* ]]; then
      if ! rm -rf -- "$canonical"; then
        echo "WARNING: could not remove this run's temporary directory: $canonical" >&2
        status=1
      fi
    else
      echo "WARNING: temporary path failed canonical-path checks and was preserved: $TMP_ROOT" >&2
      status=1
    fi
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

run_clean() {
  env -i "PATH=$SAFE_PATH" "HOME=$HOME_DIR" "TMPDIR=$TMP_WORK" "$@"
}

# Keep the existing module/build caches but discard all other inherited settings,
# including application DB credentials and integration-test DSNs.
mapfile -t GO_CACHE_PATHS < <(go env GOCACHE GOMODCACHE)
[[ "${#GO_CACHE_PATHS[@]}" -eq 2 && -n "${GO_CACHE_PATHS[0]}" && -n "${GO_CACHE_PATHS[1]}" ]] || fail "Could not resolve local Go caches."
GO_ENV=(
  "PATH=$SAFE_PATH"
  "HOME=$HOME_DIR"
  "TMPDIR=$TMP_WORK"
  "GOCACHE=${GO_CACHE_PATHS[0]}"
  "GOMODCACHE=${GO_CACHE_PATHS[1]}"
  "GOTOOLCHAIN=local"
  "GOENV=off"
  "GOWORK=off"
  "GOPROXY=off"
  "GOSUMDB=off"
  "GOFLAGS=-mod=readonly"
)
run_go() {
  env -i "${GO_ENV[@]}" go "$@"
}

printf '%s\n' '== Go unit tests (offline; DB integration DSNs absent) =='
run_go test -count=1 ./...
printf '%s\n' '== Go build (offline) =='
run_go build ./...
printf '%s\n' '== Go vet (offline) =='
run_go vet ./...

mapfile -d '' WEB_TESTS < <(find "$REPO_ROOT/web/src" -type f -name '*.test.js' -print0 | sort -z)
((${#WEB_TESTS[@]} > 0)) || fail "No web/src/*.test.js files were found."
printf '== Frontend Node tests (%d files; offline) ==\n' "${#WEB_TESTS[@]}"
run_clean node --test "${WEB_TESTS[@]}"

printf '%s\n' '== Frontend Vite production build (temporary outDir; no install) =='
run_clean "$REPO_ROOT/web/node_modules/.bin/vite" build "$REPO_ROOT/web" --outDir "$TMP_ROOT/web-dist" --emptyOutDir
[[ -s "$TMP_ROOT/web-dist/index.html" && -s "$TMP_ROOT/web-dist/mobile.html" ]] || fail "Vite build did not produce both expected HTML entry points."

printf '%s\n' '== Shell syntax checks =='
mapfile -d '' SHELL_SCRIPTS < <(find "$SCRIPT_DIR" -maxdepth 1 -type f -name '*.sh' -print0 | sort -z)
for file in "${SHELL_SCRIPTS[@]}"; do
  run_clean bash -n "$file"
done
printf 'PASS: bash -n checked %d scripts/*.sh files.\n' "${#SHELL_SCRIPTS[@]}"

printf '%s\n' '== Static GitHub upload filter regression (no PowerShell or gh) =='
run_clean python3 "$SCRIPT_DIR/test-upload-github-filter.py"

printf '%s\n' '== Static PowerShell backup reflection regression (no PowerShell) =='
run_clean python3 "$SCRIPT_DIR/test-backup-mysql-reflection.py"

printf '%s\n' '== Production deployment preflight tests (synthetic environment only) =='
run_clean python3 "$SCRIPT_DIR/test-production-preflight.py"

printf '%s\n' '== Synthetic backup tests (no real DB or age) =='
for test_script in test-backup-mysql.sh test-backup-integrity.sh test-backup-retention.sh; do
  run_clean bash "$SCRIPT_DIR/$test_script"
done

if command -v pwsh >/dev/null 2>&1; then
  printf '%s\n' '== Synthetic PowerShell backup test (local pwsh runtime) =='
  run_clean pwsh -NoLogo -NoProfile -NonInteractive -File "$SCRIPT_DIR/test-backup-mysql.ps1"
else
  printf '%s\n' 'SKIP: scripts/test-backup-mysql.ps1 (pwsh is not installed; no runtime was downloaded).'
fi

if ((WITH_AGE_RESTORE)); then
  AGE_RESTORE_SKIP=""
  if [[ "$(uname -s)" != "Linux" ]]; then
    AGE_RESTORE_SKIP="Linux is required"
  elif [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    AGE_RESTORE_SKIP="the restore harness refuses to run as root"
  else
    for tool in age age-keygen mysqld mysql mysqladmin ss; do
      if ! command -v "$tool" >/dev/null 2>&1; then
        AGE_RESTORE_SKIP="missing local dependency: $tool"
        break
      fi
    done
  fi
  if [[ -z "$AGE_RESTORE_SKIP" ]]; then
    MYSQL_SERVER_VERSION="$(run_clean mysqld --no-defaults --version 2>/dev/null || true)"
    MYSQL_CLIENT_VERSION="$(run_clean mysql --no-defaults --version 2>/dev/null || true)"
    if [[ ! "$MYSQL_SERVER_VERSION" =~ Ver[[:space:]]8\. || ! "$MYSQL_CLIENT_VERSION" =~ Ver[[:space:]]8\. ]]; then
      AGE_RESTORE_SKIP="local MySQL 8.x server and client are required (found server '${MYSQL_SERVER_VERSION:-unknown}', client '${MYSQL_CLIENT_VERSION:-unknown}')"
    fi
  fi
  if [[ -n "$AGE_RESTORE_SKIP" ]]; then
    printf 'SKIP: optional age/MySQL synthetic restore test: %s.\n' "$AGE_RESTORE_SKIP"
  else
    printf '%s\n' '== Optional synthetic age/MySQL restore drill (temporary socket-only server) =='
    run_clean bash "$SCRIPT_DIR/test-backup-restore-age.sh"
  fi
else
  printf '%s\n' 'SKIP: optional age/MySQL restore drill (not requested; use --with-age-restore to check local prerequisites).'
fi

if ((WITH_SCHEMA_LIFECYCLE)); then
  SCHEMA_LIFECYCLE_SKIP=""
  if [[ "$(uname -s)" != "Linux" ]]; then
    SCHEMA_LIFECYCLE_SKIP="Linux is required"
  elif [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    SCHEMA_LIFECYCLE_SKIP="the schema lifecycle harness refuses to run as root"
  elif [[ -n "${MYSQL_DSN:-}" || -n "${MYSQL_PWD:-}" ]]; then
    SCHEMA_LIFECYCLE_SKIP="MYSQL_DSN or MYSQL_PWD is set; the harness refuses inherited database credentials"
  else
    for tool in mysqld mysql mysqladmin mysqldump ss; do
      if ! command -v "$tool" >/dev/null 2>&1; then
        SCHEMA_LIFECYCLE_SKIP="missing local dependency: $tool"
        break
      fi
    done
  fi
  if [[ -z "$SCHEMA_LIFECYCLE_SKIP" ]]; then
    MYSQL_SERVER_VERSION="$(mysqld --no-defaults --version 2>/dev/null || true)"
    if [[ ! "$MYSQL_SERVER_VERSION" =~ Ver[[:space:]]8\. ]]; then
      SCHEMA_LIFECYCLE_SKIP="local MySQL 8.x server is required (found '${MYSQL_SERVER_VERSION:-unknown}')"
    fi
  fi
  if [[ -n "$SCHEMA_LIFECYCLE_SKIP" ]]; then
    printf 'SKIP: optional isolated schema lifecycle test: %s.\n' "$SCHEMA_LIFECYCLE_SKIP"
  else
    printf '%s\n' '== Optional isolated MySQL 8 schema lifecycle (temporary socket-only server) =='
    bash "$SCRIPT_DIR/validate-schema-lifecycle.sh"
  fi
else
  printf '%s\n' 'SKIP: optional schema lifecycle test (not requested; use --with-schema-lifecycle when local MySQL 8 is available).'
fi

printf '%s\n' 'PASS: local offline regression baseline completed; temporary build/test files are being removed.'
