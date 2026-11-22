#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RETENTION_SCRIPT="$SCRIPT_DIR/backup-retention.py"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/cue-backup-retention-test.XXXXXX")"
trap 'rm -rf -- "$TEST_ROOT"' EXIT
BACKUP_DIR="$TEST_ROOT/synthetic backups"
mkdir -p "$BACKUP_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

OLD_A="$BACKUP_DIR/synthetic_db-20260801-120000.sql.age"
OLD_B="$BACKUP_DIR/synthetic_db-20260802-120000.sql.age"
RECENT="$BACKUP_DIR/synthetic_db-20261120-120000.sql.age"
PLAIN="$BACKUP_DIR/legacy-synthetic-20260801-120000.sql"
OTHER="$BACKUP_DIR/notes.txt"
printf '%s\n' 'synthetic old ciphertext A' > "$OLD_A"
printf '%s\n' 'synthetic old ciphertext B' > "$OLD_B"
printf '%s\n' 'synthetic recent ciphertext' > "$RECENT"
printf '%s\n' 'synthetic legacy plaintext; must not be deleted' > "$PLAIN"
printf '%s\n' 'synthetic unrelated file' > "$OTHER"
touch -d '90 days ago' "$OLD_A" "$OLD_B"
touch -d '3 days ago' "$RECENT"
touch -d '90 days ago' "$PLAIN" "$OTHER"

# Dry-run must enumerate only old encrypted artifacts, show a plan token, and mutate nothing.
python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" > "$TEST_ROOT/dry-run.log" || fail 'dry-run failed'
grep -q "$(basename "$OLD_A")" "$TEST_ROOT/dry-run.log" || fail 'dry-run omitted first expired encrypted backup'
grep -q "$(basename "$OLD_B")" "$TEST_ROOT/dry-run.log" || fail 'dry-run omitted second expired encrypted backup'
if grep -q "$(basename "$PLAIN")\|$(basename "$OTHER")" "$TEST_ROOT/dry-run.log"; then fail 'dry-run listed plaintext or unrelated files'; fi
[[ -e "$OLD_A" && -e "$OLD_B" && -e "$RECENT" && -e "$PLAIN" && -e "$OTHER" ]] || fail 'dry-run changed files'
TOKEN="$(awk '/^Confirmation token: / {print $3}' "$TEST_ROOT/dry-run.log")"
[[ "$TOKEN" =~ ^[0-9a-f]{24}$ ]] || fail 'dry-run did not print a valid snapshot confirmation token'

# No --apply, no token, a wrong token, or a changed candidate set must never delete.
if python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" --apply > "$TEST_ROOT/no-token.log" 2>&1; then fail '--apply without a token unexpectedly succeeded'; fi
if python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" --apply --confirm-token 000000000000000000000000 > "$TEST_ROOT/wrong-token.log" 2>&1; then fail 'wrong token unexpectedly succeeded'; fi
[[ -e "$OLD_A" && -e "$OLD_B" ]] || fail 'invalid confirmation deleted a candidate'
LATE="$BACKUP_DIR/late-synthetic-20260803-120000.sql.age"
printf '%s\n' 'synthetic newly discovered expired ciphertext' > "$LATE"
touch -d '91 days ago' "$LATE"
if python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" --apply --confirm-token "$TOKEN" > "$TEST_ROOT/stale-token.log" 2>&1; then fail 'stale candidate token unexpectedly succeeded'; fi
[[ -e "$OLD_A" && -e "$OLD_B" && -e "$LATE" ]] || fail 'stale token deleted a file'
rm -- "$LATE"

# Explicit application of the current reviewed snapshot removes only expired .sql.age files.
python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" > "$TEST_ROOT/fresh-plan.log" || fail 'fresh dry-run failed'
TOKEN="$(awk '/^Confirmation token: / {print $3}' "$TEST_ROOT/fresh-plan.log")"
python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" --apply --confirm-token "$TOKEN" > "$TEST_ROOT/apply.log" || fail 'confirmed synthetic cleanup failed'
[[ ! -e "$OLD_A" && ! -e "$OLD_B" ]] || fail 'confirmed cleanup retained expired encrypted candidates'
[[ -e "$RECENT" && -e "$PLAIN" && -e "$OTHER" ]] || fail 'cleanup removed a recent, plaintext, or unrelated file'

# Refuse a plan that would delete the final remaining encrypted backup.
LAST_DIR="$TEST_ROOT/only-one"
mkdir -p "$LAST_DIR"
LAST="$LAST_DIR/only-synthetic-20260801-120000.sql.age"
printf '%s\n' 'synthetic last backup' > "$LAST"
touch -d '90 days ago' "$LAST"
python3 "$RETENTION_SCRIPT" --backup-dir "$LAST_DIR" > "$TEST_ROOT/last-plan.log" || fail 'last-backup dry-run failed'
grep -q 'BLOCKED: refusing to delete the last remaining encrypted backup' "$TEST_ROOT/last-plan.log" || fail 'last backup was not protected'
if grep -q '^Confirmation token:' "$TEST_ROOT/last-plan.log"; then fail 'blocked last-backup plan exposed an apply token'; fi
[[ -e "$LAST" ]] || fail 'dry-run removed the last backup'

# Symlink candidates and symlinked directory roots fail closed.
ln -s -- "$RECENT" "$BACKUP_DIR/linked-synthetic-20260801-120000.sql.age"
if python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" > "$TEST_ROOT/symlink.log" 2>&1; then fail 'SQL age symlink unexpectedly accepted'; fi
[[ -L "$BACKUP_DIR/linked-synthetic-20260801-120000.sql.age" && -e "$RECENT" ]] || fail 'symlink check changed its target'
ln -s -- "$BACKUP_DIR" "$TEST_ROOT/backup-dir-link"
if python3 "$RETENTION_SCRIPT" --backup-dir "$TEST_ROOT/backup-dir-link" > "$TEST_ROOT/dir-symlink.log" 2>&1; then fail 'symlinked backup directory unexpectedly accepted'; fi

if python3 "$RETENTION_SCRIPT" --backup-dir "$BACKUP_DIR" --keep-days 29 > "$TEST_ROOT/short-retention.log" 2>&1; then fail 'retention shorter than 30 days unexpectedly accepted'; fi

# Simulate a replacement arriving after the final path check but before the atomic quarantine move.
python3 - "$RETENTION_SCRIPT" "$TEST_ROOT" <<'PY'
import importlib.util
import os
import sys
import tempfile
import time
from pathlib import Path
from unittest.mock import patch

script = Path(sys.argv[1]).resolve()
test_root = Path(sys.argv[2])
spec = importlib.util.spec_from_file_location("backup_retention_race_test", script)
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)

with tempfile.TemporaryDirectory(prefix="retention-race-", dir=test_root) as temp:
    directory = Path(temp)
    candidate = directory / "synthetic_db-20260801-120000.sql.age"
    candidate.write_bytes(b"approved synthetic backup")
    old = time.time() - 90 * 24 * 60 * 60
    os.utime(candidate, (old, old))
    planned = module.inventory(directory)
    original_unlink = Path.unlink
    original_rename = os.rename
    injected = {"value": False}

    def inject_replacement():
        injected["value"] = True
        original_unlink(candidate)
        candidate.write_bytes(b"unapproved replacement")

    def racing_unlink(path, *args, **kwargs):
        if path == candidate and not injected["value"]:
            inject_replacement()
        return original_unlink(path, *args, **kwargs)

    def racing_rename(source, destination, *args, **kwargs):
        if Path(source) == candidate and not injected["value"]:
            inject_replacement()
        return original_rename(source, destination, *args, **kwargs)

    try:
        with patch.object(module.os, "rename", new=racing_rename), patch.object(Path, "unlink", new=racing_unlink):
            module.apply_plan(directory, planned)
    except module.RetentionError as exc:
        if "candidate changed during deletion" not in str(exc):
            raise
    else:
        raise SystemExit("FAIL: replacement race was not rejected")

    if not injected["value"] or not candidate.is_file() or candidate.read_bytes() != b"unapproved replacement":
        raise SystemExit("FAIL: unapproved replacement was not preserved at its original path")
    if list(directory.glob(module.RETENTION_QUARANTINE_PREFIX + "*")):
        raise SystemExit("FAIL: successful restoration left a quarantine directory behind")

PY

# An interrupted cleanup must be visible and block later retention runs, not hide backups silently.
STALE_ROOT="$TEST_ROOT/stale-quarantine-case"
STALE_QUARANTINE="$STALE_ROOT/.cue-backup-retention-quarantine-interrupted"
mkdir -p "$STALE_QUARANTINE"
printf '%s\n' 'synthetic preserved encrypted backup' > "$STALE_QUARANTINE/synthetic_db-20260801-120000.sql.age"
if python3 "$RETENTION_SCRIPT" --backup-dir "$STALE_ROOT" > "$TEST_ROOT/stale-quarantine.log" 2>&1; then fail 'stale retention quarantine unexpectedly accepted'; fi
grep -q 'unfinished retention quarantine found' "$TEST_ROOT/stale-quarantine.log" || fail 'stale quarantine failure did not explain how to recover'
[[ -e "$STALE_QUARANTINE/synthetic_db-20260801-120000.sql.age" ]] || fail 'stale quarantine check changed its preserved backup'

printf '%s\n' 'PASS: synthetic retention dry-run, exact-token apply, drift, last-backup, symlink, TOCTOU, and interrupted-quarantine safeguards'
