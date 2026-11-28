#!/usr/bin/env bash
set -Eeuo pipefail

ALLOW_NO_COMPOSE=false
if [[ "$#" -gt 1 ]]; then
  echo "Usage: $0 [--allow-no-compose]" >&2
  exit 2
fi
if [[ "$#" -eq 1 ]]; then
  if [[ "$1" != "--allow-no-compose" ]]; then
    echo "Usage: $0 [--allow-no-compose]" >&2
    exit 2
  fi
  ALLOW_NO_COMPOSE=true
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd -P)"
COMPOSE_FILES=(
  docker-compose.yml
  docker-compose.prod.yml
  docker-compose.new-install.yml
  docker-compose.managed.yml
)

for compose_file in "${COMPOSE_FILES[@]}"; do
  if [[ ! -f "$REPO_ROOT/$compose_file" ]]; then
    echo "Missing Compose configuration: $compose_file" >&2
    exit 1
  fi
done

TMP_PARENT="${TMPDIR:-/tmp}"
TMP_ROOT="$(mktemp -d "$TMP_PARENT/cue-compose-config.XXXXXXXX")"
chmod 700 "$TMP_ROOT"
cleanup() {
  if [[ -n "${TMP_ROOT:-}" && -d "$TMP_ROOT" && ! -L "$TMP_ROOT" ]]; then
    rm -rf -- "$TMP_ROOT"
  fi
}
trap cleanup EXIT

mkdir -p "$TMP_ROOT/home" "$TMP_ROOT/docker-config" "$TMP_ROOT/web"
chmod 700 "$TMP_ROOT/home" "$TMP_ROOT/docker-config" "$TMP_ROOT/web"

# Copy only the Compose YAML files; never copy, source, or inspect the repository's
# .env.production. Their env_file entries resolve to this synthetic file instead.
for compose_file in "${COMPOSE_FILES[@]}"; do
  cp -- "$REPO_ROOT/$compose_file" "$TMP_ROOT/$compose_file"
done

cat > "$TMP_ROOT/.env.production" <<'SYNTHETIC_ENV'
MYSQL_ROOT_PASSWORD=synthetic_root_password_for_config_validation
MYSQL_APP_PASSWORD=synthetic_app_password_for_config_validation
MYSQL_DSN=mysql://synthetic_user:synthetic_password@tcp(mysql:3306)/synthetic_database
SYNTHETIC_ENV
# The default Compose references env_file .env (not .env.production);
# give it the same synthetic values so `docker compose config` can parse
# it without touching any real credential.
cat > "$TMP_ROOT/.env" <<'SYNTHETIC_ENV'
MYSQL_ROOT_PASSWORD=synthetic-root-password
MYSQL_APP_PASSWORD=synthetic-app-password
MYSQL_DSN=mysql://synthetic_user:synthetic_password@tcp(mysql:3306)/synthetic_database
SYNTHETIC_ENV
cat > "$TMP_ROOT/compose-interpolation.env" <<'SYNTHETIC_INTERPOLATION'
MYSQL_ROOT_PASSWORD=synthetic_root_password_for_config_validation
MYSQL_APP_PASSWORD=synthetic_app_password_for_config_validation
SYNTHETIC_INTERPOLATION

# Provide placeholder build contexts so Compose can validate paths without
# building or starting anything. These files exist only in the temporary tree.
printf '%s\n' '# synthetic config-validation placeholder; never built' > "$TMP_ROOT/Dockerfile"
printf '%s\n' '# synthetic config-validation placeholder; never built' > "$TMP_ROOT/web/Dockerfile"

SAFE_PATH="${PATH:-/usr/local/bin:/usr/bin:/bin}"
COMPOSE_AVAILABLE=false
if command -v docker >/dev/null 2>&1 \
  && env -i PATH="$SAFE_PATH" HOME="$TMP_ROOT/home" DOCKER_CONFIG="$TMP_ROOT/docker-config" \
    docker compose version >/dev/null 2>&1; then
  COMPOSE_AVAILABLE=true
fi

if [[ "$COMPOSE_AVAILABLE" != true && "$ALLOW_NO_COMPOSE" != true ]]; then
  echo "Docker Compose v2 is required for this check (no containers are started)." >&2
  echo "For local YAML-only validation without Docker, run: $0 --allow-no-compose" >&2
  exit 1
fi

if [[ "$COMPOSE_AVAILABLE" == true ]]; then
  for compose_file in "${COMPOSE_FILES[@]}"; do
    # env -i prevents inherited production credentials/hosts from affecting
    # interpolation or selecting a remote Docker context. Config is parsed only.
    if ! env -i PATH="$SAFE_PATH" HOME="$TMP_ROOT/home" DOCKER_CONFIG="$TMP_ROOT/docker-config" \
      docker compose \
        --project-directory "$TMP_ROOT" \
        --env-file "$TMP_ROOT/compose-interpolation.env" \
        --file "$TMP_ROOT/$compose_file" \
        config --quiet; then
      echo "Compose config validation failed: $compose_file" >&2
      exit 1
    fi
    printf 'Compose config validated: %s\n' "$compose_file"
  done
else
  python3 - "$TMP_ROOT" "${COMPOSE_FILES[@]}" <<'PY'
from pathlib import Path
import sys

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required for local YAML-only validation without Docker.") from exc

root = Path(sys.argv[1]).resolve()
for name in sys.argv[2:]:
    path = root / name
    if path.parent != root or not path.is_file():
        raise SystemExit(f"Expected an isolated temporary Compose file: {name}")
    with path.open(encoding="utf-8") as stream:
        config = yaml.safe_load(stream)
    if not isinstance(config, dict) or not isinstance(config.get("services"), dict):
        raise SystemExit(f"Invalid Compose YAML structure: {name}")
    print(f"Compose YAML syntax validated (Docker Compose unavailable): {name}")
PY
fi
