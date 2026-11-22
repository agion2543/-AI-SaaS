#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
INTEGRITY_SCRIPT="$SCRIPT_DIR/backup-integrity.py"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/cue-backup-integrity-test.XXXXXX")"
trap 'rm -rf -- "$TEST_ROOT"' EXIT

BACKUP_DIR="$TEST_ROOT/synthetic backups"
MANIFEST="$TEST_ROOT/synthetic-manifest.json"
mkdir -p "$BACKUP_DIR"
BACKUP_A="$BACKUP_DIR/synthetic db-20261120-120000.sql"
BACKUP_B="$BACKUP_DIR/second-synthetic-20261120-120001.sql"
BACKUP_C="$BACKUP_DIR/encrypted-synthetic-20261120-120002.sql.age"
printf '%s\n' '-- synthetic fixture; not customer data' 'CREATE TABLE probe_a (id INTEGER);' > "$BACKUP_A"
printf '%s\n' '-- synthetic fixture; not customer data' 'CREATE TABLE probe_b (id INTEGER);' > "$BACKUP_B"
printf '%s\n' 'synthetic age ciphertext fixture; not real encryption' > "$BACKUP_C"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

run_create() {
  python3 "$INTEGRITY_SCRIPT" create --backup-dir "$BACKUP_DIR" --manifest "$MANIFEST"
}

run_verify() {
  python3 "$INTEGRITY_SCRIPT" verify --backup-dir "$BACKUP_DIR" --manifest "$MANIFEST"
}

before_a="$(sha256sum -- "$BACKUP_A")"
before_b="$(sha256sum -- "$BACKUP_B")"
stat_a="$(stat -c '%i:%s:%Y' -- "$BACKUP_A")"
stat_b="$(stat -c '%i:%s:%Y' -- "$BACKUP_B")"
before_c="$(sha256sum -- "$BACKUP_C")"
stat_c="$(stat -c '%i:%s:%Y' -- "$BACKUP_C")"
run_create > "$TEST_ROOT/create.log" || fail 'manifest creation failed for synthetic fixtures'
[[ -s "$MANIFEST" ]] || fail 'manifest was not created'
[[ "$(stat -c '%a' "$MANIFEST")" == 600 ]] || fail 'manifest permissions were not 0600'
grep -q 'encrypted-synthetic-20261120-120002.sql.age' "$MANIFEST" || fail 'manifest omitted the synthetic encrypted backup'
run_verify > "$TEST_ROOT/verify.log" || fail 'unchanged synthetic fixtures did not verify'
[[ "$(sha256sum -- "$BACKUP_A")" == "$before_a" ]] || fail 'manifest workflow changed the first synthetic backup'
[[ "$(sha256sum -- "$BACKUP_B")" == "$before_b" ]] || fail 'manifest workflow changed the second synthetic backup'
[[ "$(sha256sum -- "$BACKUP_C")" == "$before_c" ]] || fail 'manifest workflow changed the synthetic encrypted backup'
[[ "$(stat -c '%i:%s:%Y' -- "$BACKUP_A")" == "$stat_a" ]] || fail 'manifest workflow altered first synthetic backup metadata'
[[ "$(stat -c '%i:%s:%Y' -- "$BACKUP_B")" == "$stat_b" ]] || fail 'manifest workflow altered second synthetic backup metadata'
[[ "$(stat -c '%i:%s:%Y' -- "$BACKUP_C")" == "$stat_c" ]] || fail 'manifest workflow altered synthetic encrypted backup metadata'

# Creating a second manifest at the same path must not overwrite the first one.
manifest_before="$(sha256sum -- "$MANIFEST")"
if run_create > "$TEST_ROOT/collision.log" 2>&1; then
  fail 'existing manifest was unexpectedly overwritten'
fi
[[ "$(sha256sum -- "$MANIFEST")" == "$manifest_before" ]] || fail 'existing manifest changed after collision'

# Content changes must be detected; these are synthetic fixtures only.
printf '%s\n' '-- changed synthetic fixture' >> "$BACKUP_A"
if run_verify > "$TEST_ROOT/changed.log" 2>&1; then
  fail 'modified synthetic backup unexpectedly verified'
fi
printf '%s\n' '-- synthetic fixture; not customer data' 'CREATE TABLE probe_a (id INTEGER);' > "$BACKUP_A"

# Added or removed SQL files must also be detected.
printf '%s\n' '-- extra synthetic fixture' > "$BACKUP_DIR/unlisted-synthetic.sql"
if run_verify > "$TEST_ROOT/added.log" 2>&1; then
  fail 'unlisted synthetic backup unexpectedly verified'
fi
rm -- "$BACKUP_DIR/unlisted-synthetic.sql"
rm -- "$BACKUP_B"
if run_verify > "$TEST_ROOT/missing.log" 2>&1; then
  fail 'missing synthetic backup unexpectedly verified'
fi
printf '%s\n' '-- synthetic fixture; not customer data' 'CREATE TABLE probe_b (id INTEGER);' > "$BACKUP_B"

# Health checks require an explicit threshold, a recent encrypted file, and a matching manifest.
HEALTH_DIR="$TEST_ROOT/health-backups"
HEALTH_BACKUP="$HEALTH_DIR/health-synthetic-20261121-120000.sql.age"
HEALTH_MANIFEST="$TEST_ROOT/health-manifest.json"
mkdir -p "$HEALTH_DIR"
printf '%s\n' 'synthetic health-check ciphertext fixture; not real encryption' > "$HEALTH_BACKUP"
touch -d '1 hour ago' "$HEALTH_BACKUP"
python3 "$INTEGRITY_SCRIPT" create --backup-dir "$HEALTH_DIR" --manifest "$HEALTH_MANIFEST" > "$TEST_ROOT/health-create.log" || fail 'health-test manifest creation failed'

if python3 "$INTEGRITY_SCRIPT" health --backup-dir "$HEALTH_DIR" --manifest "$HEALTH_MANIFEST" > "$TEST_ROOT/health-no-threshold.log" 2>&1; then
  fail 'health check accepted an omitted age threshold'
fi
grep -q -- '--max-age-seconds' "$TEST_ROOT/health-no-threshold.log" || fail 'missing-threshold failure did not identify the required option'

if python3 "$INTEGRITY_SCRIPT" health --backup-dir "$HEALTH_DIR" --manifest "$HEALTH_MANIFEST" --max-age-seconds 7200 > "$TEST_ROOT/health-pass.log" 2>&1; then
  grep -q 'PASS: newest encrypted backup' "$TEST_ROOT/health-pass.log" || fail 'healthy status was not reported'
else
  fail 'recent synthetic encrypted backup with a matching manifest did not pass health check'
fi

touch -d '3 hours ago' "$HEALTH_BACKUP"
HEALTH_STALE_STATUS=0
if python3 "$INTEGRITY_SCRIPT" health --backup-dir "$HEALTH_DIR" --manifest "$HEALTH_MANIFEST" --max-age-seconds 7200 > "$TEST_ROOT/health-stale.log" 2>&1; then
  fail 'stale encrypted backup unexpectedly passed health check'
else
  HEALTH_STALE_STATUS=$?
fi
[[ "$HEALTH_STALE_STATUS" -eq 1 ]] || fail "stale health check returned $HEALTH_STALE_STATUS instead of 1"
grep -q 'newest encrypted backup is stale' "$TEST_ROOT/health-stale.log" || fail 'stale failure did not explain the age violation'

touch -d '1 hour ago' "$HEALTH_BACKUP"
printf '%s\n' 'synthetic tamper' >> "$HEALTH_BACKUP"
if python3 "$INTEGRITY_SCRIPT" health --backup-dir "$HEALTH_DIR" --manifest "$HEALTH_MANIFEST" --max-age-seconds 7200 > "$TEST_ROOT/health-mismatch.log" 2>&1; then
  fail 'tampered synthetic encrypted file unexpectedly passed health check'
fi
grep -Eq 'size mismatch|SHA-256 mismatch' "$TEST_ROOT/health-mismatch.log" || fail 'health mismatch failure did not identify the manifest discrepancy'

PLAIN_HEALTH_DIR="$TEST_ROOT/plain-only-health"
PLAIN_HEALTH_MANIFEST="$TEST_ROOT/plain-only-health-manifest.json"
mkdir -p "$PLAIN_HEALTH_DIR"
printf '%s\n' 'synthetic SQL-only fixture' > "$PLAIN_HEALTH_DIR/plain-only.sql"
python3 "$INTEGRITY_SCRIPT" create --backup-dir "$PLAIN_HEALTH_DIR" --manifest "$PLAIN_HEALTH_MANIFEST" > /dev/null || fail 'plain-only health-test manifest creation failed'
if python3 "$INTEGRITY_SCRIPT" health --backup-dir "$PLAIN_HEALTH_DIR" --manifest "$PLAIN_HEALTH_MANIFEST" --max-age-seconds 7200 > "$TEST_ROOT/health-no-encrypted.log" 2>&1; then
  fail 'plain SQL without an encrypted backup unexpectedly passed health check'
fi
grep -q 'no direct encrypted .sql.age backup found' "$TEST_ROOT/health-no-encrypted.log" || fail 'no-encrypted failure did not explain the missing artifact'

# A SQL-named symlink is rejected rather than followed.
ln -s -- "$BACKUP_A" "$BACKUP_DIR/synthetic-link.sql"
if run_verify > "$TEST_ROOT/symlink.log" 2>&1; then
  fail 'SQL-named symlink unexpectedly verified'
fi
rm -- "$BACKUP_DIR/synthetic-link.sql"
run_verify > "$TEST_ROOT/final-verify.log" || fail 'restored synthetic fixtures did not verify'

printf '%s\n' 'PASS: synthetic integrity manifests, fresh/stale health thresholds, manifest mismatch, no-overwrite, read-only behavior, inventory, and symlink checks'
