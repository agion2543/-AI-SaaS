#!/usr/bin/env python3
"""Static regression check for PowerShell SetUnixFileMode overload lookup.

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

print(
    "PASS: SetUnixFileMode uses an exact overload signature with a null guard (static only)."
)
