#!/usr/bin/env python3
"""Static regression check for the GitHub uploader's environment-file filter.

This check deliberately does not execute PowerShell, gh, or any network action.
"""
from fnmatch import fnmatchcase
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
UPLOADER = ROOT / "scripts" / "upload-to-github.ps1"
source = UPLOADER.read_text(encoding="utf-8")

allowlist_match = re.search(
    r'(?is)\$envExampleFiles\s*=\s*@\((.*?)\)', source
)
if not allowlist_match:
    sys.exit("FAIL: uploader must declare the safe environment-template allowlist")
allowlist = re.findall(r'"([^"\r\n]+)"', allowlist_match.group(1))
expected_allowlist = [".env.example", ".env.production.example"]
if allowlist != expected_allowlist:
    sys.exit(f"FAIL: safe environment-template allowlist changed: {allowlist!r}")

filter_guard = re.compile(
    r'(?is)\$File\.Name\s+-eq\s+"\.env"\s+-or\s*'
    r'\(\s*\$File\.Name\s+-like\s+"\.env\.\*"\s+-and\s*'
    r'\$File\.Name\s+-notin\s+\$envExampleFiles\s*\)\s*\)'
)
if not filter_guard.search(source):
    sys.exit("FAIL: uploader must exclude .env and non-example .env.* files")

for template in expected_allowlist:
    if not (ROOT / template).is_file():
        sys.exit(f"FAIL: expected public template is missing: {template}")

# Mirror the checked PowerShell -like/-notin contract for representative names.
def should_skip(name: str) -> bool:
    return name == ".env" or (
        fnmatchcase(name, ".env.*") and name not in allowlist
    )

cases = {
    ".env": True,
    ".env.production": True,
    ".env.local": True,
    ".env.production.local": True,
    ".env.example": False,
    ".env.production.example": False,
    "README.md": False,
}
for name, expected in cases.items():
    actual = should_skip(name)
    if actual != expected:
        sys.exit(f"FAIL: filter result for {name!r} is {actual}, expected {expected}")

print("PASS: uploader excludes .env and non-example .env.* files; safe templates remain allowed (static only).")
