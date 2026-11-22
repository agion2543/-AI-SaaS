#!/usr/bin/env python3
"""Create or verify a non-destructive SHA-256 manifest for SQL backup files."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import stat
import sys
import tempfile
import time
from pathlib import Path
from typing import Any


FORMAT = "cue-backup-integrity"
VERSION = 1
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
CHUNK_SIZE = 1024 * 1024


class IntegrityError(Exception):
    """An invalid directory, manifest, or backup file was encountered."""


def validate_name(name: Any) -> str:
    if (
        not isinstance(name, str)
        or not name
        or name in {".", ".."}
        or "/" in name
        or "\\" in name
        or not (name.endswith(".sql") or name.endswith(".sql.age"))
        or any(ord(character) < 32 or ord(character) == 127 for character in name)
    ):
        raise IntegrityError(f"invalid SQL or encrypted SQL backup filename in manifest or directory: {name!r}")
    return name


def resolve_backup_dir(value: str) -> Path:
    try:
        directory = Path(value).resolve(strict=True)
    except OSError as exc:
        raise IntegrityError(f"cannot resolve backup directory {value!r}: {exc}") from exc
    if not directory.is_dir():
        raise IntegrityError(f"backup directory is not a directory: {directory}")
    return directory


def inventory(directory: Path) -> dict[str, Path]:
    files: dict[str, Path] = {}
    try:
        with os.scandir(directory) as entries:
            for entry in entries:
                if not (entry.name.endswith(".sql") or entry.name.endswith(".sql.age")):
                    continue
                name = validate_name(entry.name)
                try:
                    info = entry.stat(follow_symlinks=False)
                except OSError as exc:
                    raise IntegrityError(f"cannot inspect {name!r}: {exc}") from exc
                if not stat.S_ISREG(info.st_mode):
                    raise IntegrityError(f"SQL backup is not a regular, non-symlink file: {name!r}")
                files[name] = Path(entry.path)
    except OSError as exc:
        raise IntegrityError(f"cannot list backup directory {directory}: {exc}") from exc
    if not files:
        raise IntegrityError(f"no direct *.sql or *.sql.age backup files found in {directory}")
    return dict(sorted(files.items()))


def fingerprint(path: Path) -> tuple[int, str]:
    flags = os.O_RDONLY
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    try:
        descriptor = os.open(path, flags)
    except OSError as exc:
        raise IntegrityError(f"cannot open backup {path.name!r} without following symlinks: {exc}") from exc

    digest = hashlib.sha256()
    total = 0
    try:
        with os.fdopen(descriptor, "rb") as source:
            before = os.fstat(source.fileno())
            if not stat.S_ISREG(before.st_mode):
                raise IntegrityError(f"backup is not a regular file: {path.name!r}")
            while True:
                chunk = source.read(CHUNK_SIZE)
                if not chunk:
                    break
                digest.update(chunk)
                total += len(chunk)
            after = os.fstat(source.fileno())
        current = os.stat(path, follow_symlinks=False)
    except OSError as exc:
        raise IntegrityError(f"cannot read backup {path.name!r}: {exc}") from exc

    identity_before = (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
    identity_after = (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns)
    identity_current = (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns)
    if identity_before != identity_after or identity_after != identity_current or total != after.st_size:
        raise IntegrityError(f"backup changed while it was being checked: {path.name!r}")
    return total, digest.hexdigest()


def atomic_create_new(path: Path, payload: bytes) -> None:
    try:
        parent = path.parent.resolve(strict=True)
    except OSError as exc:
        raise IntegrityError(f"manifest parent directory must already exist: {path.parent}") from exc
    if not parent.is_dir():
        raise IntegrityError(f"manifest parent is not a directory: {parent}")
    target = parent / path.name
    if target.suffix.lower() != ".json":
        raise IntegrityError("manifest path must end in .json")

    temporary_name: str | None = None
    try:
        descriptor, temporary_name = tempfile.mkstemp(
            prefix=f".{target.name}.", suffix=".tmp", dir=parent
        )
        with os.fdopen(descriptor, "wb") as destination:
            destination.write(payload)
            destination.flush()
            os.fsync(destination.fileno())
        # A hard link publishes atomically and fails if target already exists.
        os.link(temporary_name, target)
        try:
            directory_fd = os.open(parent, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
            try:
                os.fsync(directory_fd)
            finally:
                os.close(directory_fd)
        except OSError:
            # The manifest is already safely published; directory fsync is best effort.
            pass
    except FileExistsError as exc:
        raise IntegrityError(f"refusing to overwrite existing manifest: {target}") from exc
    except OSError as exc:
        raise IntegrityError(f"cannot create manifest {target}: {exc}") from exc
    finally:
        if temporary_name is not None:
            try:
                os.unlink(temporary_name)
            except FileNotFoundError:
                pass
            except OSError:
                pass


def create_manifest(directory: Path, manifest_path: Path) -> int:
    files = inventory(directory)
    records = []
    for name, path in files.items():
        size, digest = fingerprint(path)
        records.append({"name": name, "size_bytes": size, "sha256": digest})

    now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    document = {
        "format": FORMAT,
        "version": VERSION,
        "created_at_utc": now,
        "files": records,
    }
    payload = (json.dumps(document, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    atomic_create_new(manifest_path, payload)
    return len(records)


def read_manifest(path: Path) -> dict[str, Any]:
    try:
        with path.open("r", encoding="utf-8") as source:
            document = json.load(source)
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise IntegrityError(f"cannot read valid JSON manifest {path}: {exc}") from exc
    if not isinstance(document, dict) or document.get("format") != FORMAT or document.get("version") != VERSION:
        raise IntegrityError("unsupported or invalid manifest format/version")
    if not isinstance(document.get("created_at_utc"), str) or not document["created_at_utc"]:
        raise IntegrityError("manifest is missing created_at_utc")
    records = document.get("files")
    if not isinstance(records, list) or not records:
        raise IntegrityError("manifest must list at least one SQL backup")

    seen: set[str] = set()
    for record in records:
        if not isinstance(record, dict):
            raise IntegrityError("manifest contains an invalid file record")
        name = validate_name(record.get("name"))
        if name in seen:
            raise IntegrityError(f"manifest contains a duplicate filename: {name!r}")
        seen.add(name)
        size = record.get("size_bytes")
        digest = record.get("sha256")
        if isinstance(size, bool) or not isinstance(size, int) or size < 0:
            raise IntegrityError(f"manifest has an invalid size for {name!r}")
        if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
            raise IntegrityError(f"manifest has an invalid SHA-256 value for {name!r}")
    return document


def verify_manifest(directory: Path, manifest_path: Path) -> int:
    document = read_manifest(manifest_path)
    expected = {record["name"]: record for record in document["files"]}
    current = inventory(directory)
    expected_names = set(expected)
    current_names = set(current)
    missing = sorted(expected_names - current_names)
    added = sorted(current_names - expected_names)
    if missing or added:
        details = []
        if missing:
            details.append("missing: " + ", ".join(repr(name) for name in missing))
        if added:
            details.append("unlisted: " + ", ".join(repr(name) for name in added))
        raise IntegrityError("backup inventory differs from manifest (" + "; ".join(details) + ")")

    for name in sorted(expected):
        actual_size, actual_digest = fingerprint(current[name])
        record = expected[name]
        if actual_size != record["size_bytes"]:
            raise IntegrityError(f"size mismatch for {name!r}: expected {record['size_bytes']}, found {actual_size}")
        if actual_digest != record["sha256"]:
            raise IntegrityError(f"SHA-256 mismatch for {name!r}")
    return len(expected)


def backup_identity_snapshot(directory: Path) -> dict[str, tuple[int, int, int, int]]:
    identities: dict[str, tuple[int, int, int, int]] = {}
    for name, path in inventory(directory).items():
        try:
            info = path.lstat()
        except OSError as exc:
            raise IntegrityError(f"cannot inspect {name!r} during health check: {exc}") from exc
        if not stat.S_ISREG(info.st_mode):
            raise IntegrityError(f"SQL backup is not a regular, non-symlink file: {name!r}")
        identities[name] = (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns)
    return identities


def check_recent_backup(directory: Path, manifest_path: Path, max_age_seconds: int) -> tuple[int, str, int]:
    if max_age_seconds <= 0:
        raise IntegrityError("max-age-seconds must be a positive integer")

    before = backup_identity_snapshot(directory)
    encrypted = [(name, identity) for name, identity in before.items() if name.endswith(".sql.age")]
    if not encrypted:
        raise IntegrityError(f"no direct encrypted .sql.age backup found in {directory}")

    newest_name, newest_identity = max(encrypted, key=lambda entry: (entry[1][3], entry[0]))
    manifest_count = verify_manifest(directory, manifest_path)
    after = backup_identity_snapshot(directory)
    if before != after:
        raise IntegrityError("backup inventory or file metadata changed during health check")

    age_ns = time.time_ns() - newest_identity[3]
    if age_ns < 0:
        raise IntegrityError(f"newest encrypted backup has a future modification time: {newest_name!r}")
    if age_ns > max_age_seconds * 1_000_000_000:
        age_seconds = age_ns // 1_000_000_000
        raise IntegrityError(
            f"newest encrypted backup is stale: {newest_name!r} is {age_seconds}s old; "
            f"maximum allowed is {max_age_seconds}s"
        )
    return manifest_count, newest_name, age_ns


def positive_seconds(value: str) -> int:
    try:
        seconds = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be a positive integer number of seconds") from exc
    if seconds <= 0:
        raise argparse.ArgumentTypeError("must be a positive integer number of seconds")
    return seconds


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create, verify, or health-check local SQL backups; this tool never modifies or deletes backups."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)
    for command in ("create", "verify"):
        subparser = subparsers.add_parser(command)
        subparser.add_argument("--backup-dir", required=True, help="explicit directory containing direct *.sql or *.sql.age files")
        subparser.add_argument("--manifest", required=True, help="explicit JSON manifest path")
    health_parser = subparsers.add_parser("health", help="check newest encrypted backup age and verify its manifest")
    health_parser.add_argument("--backup-dir", required=True, help="explicit directory containing direct *.sql or *.sql.age files")
    health_parser.add_argument("--manifest", required=True, help="explicit matching JSON manifest path")
    health_parser.add_argument(
        "--max-age-seconds",
        type=positive_seconds,
        required=True,
        help="caller-selected maximum age of the newest .sql.age file's local mtime; no default is assumed",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        directory = resolve_backup_dir(args.backup_dir)
        manifest = Path(args.manifest)
        if args.command == "create":
            count = create_manifest(directory, manifest)
            print(f"Created read-only integrity manifest for {count} SQL backup(s): {manifest}")
        elif args.command == "verify":
            count = verify_manifest(directory, manifest)
            print(f"PASS: {count} SQL backup(s) match manifest: {manifest}")
        else:
            count, newest_name, age_ns = check_recent_backup(directory, manifest, args.max_age_seconds)
            print(
                f"PASS: newest encrypted backup {newest_name!r} is {age_ns / 1_000_000_000:.1f}s old "
                f"(maximum {args.max_age_seconds}s); {count} SQL backup(s) match manifest: {manifest}"
            )
    except IntegrityError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
