#!/usr/bin/env python3
"""Read-only validation for the production Compose environment."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path
from urllib.parse import urlsplit

REQUIRED_ENV_FILE_VALUES = (
    "APP_ENV",
    "APP_URL",
    "FRONTEND_URL",
    "JWT_SECRET",
    "AES_SECRET",
    "INITIAL_ADMIN_USERNAME",
    "INITIAL_ADMIN_PASSWORD",
    "MYSQL_DSN",
    "ALIPAY_NOTIFY_URL",
    "ALIPAY_RETURN_URL",
)
REQUIRED_COMPOSE_VALUES = ("MYSQL_ROOT_PASSWORD", "MYSQL_APP_PASSWORD")
PLACEHOLDER_MARKERS = ("replace-with", "your-", "change-me", "changeme", "placeholder")
DSN_PATTERN = re.compile(r"^([^:]+):([^@]*)@tcp\(([^)]*)\)/([^?]+)(?:\?.*)?$")
ENV_KEY_PATTERN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


class PreflightError(Exception):
    """A safe-to-display validation error that never includes configuration values."""


def _is_placeholder(value: str) -> bool:
    lowered = value.strip().lower()
    return any(marker in lowered for marker in PLACEHOLDER_MARKERS)


def _parse_env_file(path: Path) -> dict[str, str]:
    try:
        text = path.read_text(encoding="utf-8-sig")
    except OSError:
        raise PreflightError("cannot read the specified environment file") from None
    except UnicodeError:
        raise PreflightError("environment file is not valid UTF-8") from None

    values: dict[str, str] = {}
    for line_number, raw_line in enumerate(text.splitlines(), start=1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export ") :].lstrip()
        if "=" not in line:
            raise PreflightError(f"environment file has a malformed assignment on line {line_number}")

        key, raw_value = line.split("=", 1)
        key = key.strip()
        if not ENV_KEY_PATTERN.fullmatch(key):
            raise PreflightError(f"environment file has an invalid variable name on line {line_number}")
        if key in values:
            raise PreflightError(f"environment file assigns {key} more than once")

        raw_value = raw_value.lstrip()
        if raw_value.startswith('"'):
            end = _closing_quote(raw_value, '"')
            if end is None or not _valid_trailing_text(raw_value[end + 1 :]):
                raise PreflightError(f"environment file has an invalid quoted value for {key}")
            try:
                value = json.loads(raw_value[: end + 1])
            except (json.JSONDecodeError, TypeError):
                raise PreflightError(f"environment file has an invalid quoted value for {key}") from None
        elif raw_value.startswith("'"):
            end = raw_value.find("'", 1)
            if end < 0 or not _valid_trailing_text(raw_value[end + 1 :]):
                raise PreflightError(f"environment file has an invalid quoted value for {key}")
            value = raw_value[1:end]
        else:
            value = _strip_unquoted_comment(raw_value).strip()
        values[key] = value
    return values


def _closing_quote(value: str, quote: str) -> int | None:
    escaped = False
    for index in range(1, len(value)):
        char = value[index]
        if escaped:
            escaped = False
        elif char == "\\":
            escaped = True
        elif char == quote:
            return index
    return None


def _valid_trailing_text(text: str) -> bool:
    text = text.strip()
    return not text or text.startswith("#")


def _strip_unquoted_comment(value: str) -> str:
    for index, char in enumerate(value):
        if char == "#" and index > 0 and value[index - 1].isspace():
            return value[:index]
    return value


def _validate_url(name: str, value: str) -> None:
    try:
        parsed = urlsplit(value)
        valid = parsed.scheme in ("http", "https") and bool(parsed.hostname)
        placeholder_host = parsed.hostname == "example.com" or (parsed.hostname or "").endswith(".example.com")
    except ValueError:
        valid = False
        placeholder_host = False
    if not valid or placeholder_host:
        raise PreflightError(f"{name} must be an absolute HTTP(S) URL, not an example-domain placeholder")


def _validate_dsn(dsn: str, app_password: str) -> None:
    match = DSN_PATTERN.fullmatch(dsn)
    if not match:
        raise PreflightError("MYSQL_DSN must use the expected MySQL TCP DSN format")
    username, dsn_password, address, database = match.groups()
    if username != "saas_user" or address != "mysql:3306" or database != "saas_billing":
        raise PreflightError("MYSQL_DSN must target saas_user@mysql:3306/saas_billing for the local production Compose database")
    if not dsn_password:
        raise PreflightError("MYSQL_DSN must include a non-empty database password")
    if dsn_password != app_password:
        raise PreflightError("MYSQL_DSN password does not match MYSQL_APP_PASSWORD")


def validate(env_file: Path, process_env: dict[str, str]) -> None:
    values = _parse_env_file(env_file)

    for name in REQUIRED_ENV_FILE_VALUES:
        if not values.get(name, "").strip():
            raise PreflightError(f"{env_file.name} is missing required value {name}")
    for name in REQUIRED_COMPOSE_VALUES:
        value = process_env.get(name, "")
        if not value.strip():
            raise PreflightError(f"required Compose environment variable {name} is not set")
        if _is_placeholder(value):
            raise PreflightError(f"{name} must not contain a placeholder value")

    if values["APP_ENV"].strip().lower() != "production":
        raise PreflightError("APP_ENV must be production")
    _validate_url("APP_URL", values["APP_URL"].strip())
    _validate_url("FRONTEND_URL", values["FRONTEND_URL"].strip())
    _validate_url("ALIPAY_NOTIFY_URL", values["ALIPAY_NOTIFY_URL"].strip())
    _validate_url("ALIPAY_RETURN_URL", values["ALIPAY_RETURN_URL"].strip())

    jwt_secret = values["JWT_SECRET"].strip()
    if len(jwt_secret.encode("utf-8")) < 32 or _is_placeholder(jwt_secret):
        raise PreflightError("JWT_SECRET must be a non-placeholder secret of at least 32 bytes")

    aes_secret = values["AES_SECRET"]
    if len(aes_secret.encode("utf-8")) != 32 or _is_placeholder(aes_secret):
        raise PreflightError("AES_SECRET must be a non-placeholder secret of exactly 32 bytes")

    initial_password = values["INITIAL_ADMIN_PASSWORD"].strip()
    if len(initial_password.encode("utf-8")) < 12 or _is_placeholder(initial_password):
        raise PreflightError("INITIAL_ADMIN_PASSWORD must be a non-placeholder value of at least 12 bytes")

    _validate_dsn(values["MYSQL_DSN"].strip(), process_env["MYSQL_APP_PASSWORD"])


def main(argv: list[str] | None = None) -> int:
    repo_root = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(
        description="Read-only preflight for the production Compose environment; no database or container is accessed."
    )
    parser.add_argument(
        "--env-file",
        type=Path,
        default=repo_root / ".env.production",
        help="environment file to validate (default: repository .env.production)",
    )
    args = parser.parse_args(argv)

    try:
        validate(args.env_file, dict(os.environ))
    except PreflightError as error:
        print(f"FAIL: {error}", file=sys.stderr)
        return 1

    print("Production deployment preflight passed.")
    print("Required production settings and Compose password variables are present and valid.")
    print("MYSQL_DSN targets saas_user@mysql:3306/saas_billing, and its password matches MYSQL_APP_PASSWORD.")
    print("Read-only check: no database connection or container operation was performed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
