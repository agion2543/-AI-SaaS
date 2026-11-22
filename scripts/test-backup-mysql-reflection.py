#!/usr/bin/env python3
"""Static regression checks for backup reflection and age environment isolation.

This test reads the PowerShell source only; it does not invoke PowerShell,
MySQL, age, gh, or the network.
"""
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
BACKUP_SCRIPT = ROOT / "scripts" / "backup-mysql.ps1"
source = BACKUP_SCRIPT.read_text(encoding="utf-8")

ambiguous_lookup = re.search(
    r'(?is)\.GetMethod\s*\(\s*["\']SetUnixFileMode["\']\s*\)',
    source,
)
if ambiguous_lookup:
    sys.exit("FAIL: SetUnixFileMode lookup must specify an overload signature")

exact_lookup = re.search(
    r'(?is)\$setUnixFileModeMethod\s*=\s*\[System\.IO\.File\]\.GetMethod\s*\(\s*'
    r'["\']SetUnixFileMode["\']\s*,\s*\[type\[\]\]\s*@\s*\(\s*'
    r'\[string\]\s*,\s*\[System\.IO\.UnixFileMode\]\s*\)\s*\)',
    source,
)
if not exact_lookup:
    sys.exit(
        "FAIL: expected exact SetUnixFileMode(string, UnixFileMode) reflection lookup"
    )

if not re.search(
    r'(?is)if\s*\(\s*\$null\s+-ne\s+\$setUnixFileModeMethod\s*\)',
    source,
):
    sys.exit("FAIL: missing null guard for the optional Unix file-mode method")

if not re.search(
    r'(?is)\[System\.IO\.File\]::SetUnixFileMode\(\s*\$temporaryDirectory\s*,\s*\$privateMode\s*\)',
    source,
):
    sys.exit("FAIL: expected temporary directory to receive its private Unix mode")

age_invocation = source.find('& $ageExecutable -R $AgeRecipientsFile -o $temporaryEncrypted $temporarySql')
if age_invocation < 0:
    sys.exit("FAIL: could not find the age process invocation")

for variable in ("MYSQL_PWD", "DB_PASSWORD"):
    removal = re.search(
        rf'(?im)^\s*Remove-Item\s+-LiteralPath\s+Env:{variable}\s+-ErrorAction\s+SilentlyContinue\s*$',
        source,
    )
    if not removal:
        sys.exit(f"FAIL: {variable} must be explicitly removed through the Env provider")
    if removal.start() > age_invocation:
        sys.exit(f"FAIL: {variable} is removed only after the age process invocation")

for variable, age_snapshot, parent_snapshot in (
    ("MYSQL_PWD", "ageMysqlPwd", "previousMysqlPwd"),
    ("DB_PASSWORD", "ageDbPassword", "previousDbPassword"),
):
    capture = re.compile(
        rf'(?is)\${age_snapshot}\s*=\s*\[Environment\]::GetEnvironmentVariable\('
        rf'"{variable}"\s*,\s*"Process"\)'
    )
    if not capture.search(source):
        sys.exit(f"FAIL: {variable} must be captured before the age invocation")
    age_restore = re.compile(
        rf'(?is)\[Environment\]::SetEnvironmentVariable\('
        rf'"{variable}"\s*,\s*\${age_snapshot}\s*,\s*"Process"\)'
    )
    if not age_restore.search(source):
        sys.exit(f"FAIL: {variable} is not restored after the age invocation")
    parent_restore = re.compile(
        rf'(?is)\[Environment\]::SetEnvironmentVariable\('
        rf'"{variable}"\s*,\s*\${parent_snapshot}\s*,\s*"Process"\)'
    )
    if not parent_restore.search(source):
        sys.exit(f"FAIL: original parent {variable} is not restored at script exit")

print(
    "PASS: exact Unix mode overload; age removes both DB variables and restores snapshots (static only)."
)
