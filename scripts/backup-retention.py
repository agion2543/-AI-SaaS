#!/usr/bin/env python3
"""Preview or safely prune old, direct-child age-encrypted MySQL backups."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import hmac
import json
import os
import re
import stat
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Sequence


DEFAULT_KEEP_DAYS = 30
BACKUP_NAME_RE = re.compile(r"^.+-(\d{8})-(\d{6})\.sql\.age$")
TOKEN_HEX_LENGTH = 24
RETENTION_QUARANTINE_PREFIX = ".cue-backup-retention-quarantine-"


class RetentionError(Exception):
    """A retention plan is unsafe or cannot be executed."""


@dataclass(frozen=True)
class BackupEntry:
    name: str
    path: Path
    size_bytes: int
    mtime_ns: int
    mtime_utc: dt.datetime
    device: int
    inode: int

    def identity(self) -> dict[str, int | str]:
        return {
            "name": self.name,
            "size_bytes": self.size_bytes,
            "mtime_ns": self.mtime_ns,
            "device": self.device,
            "inode": self.inode,
        }


def resolve_backup_dir(value: str) -> Path:
    raw = Path(value).expanduser()
    try:
        info = raw.lstat()
    except OSError as exc:
        raise RetentionError(f"cannot inspect explicit backup directory {raw}: {exc}") from exc
    if stat.S_ISLNK(info.st_mode):
        raise RetentionError(f"backup directory must not be a symlink: {raw}")
    if not stat.S_ISDIR(info.st_mode):
        raise RetentionError(f"backup path is not a directory: {raw}")
    try:
        directory = raw.resolve(strict=True)
    except OSError as exc:
        raise RetentionError(f"cannot resolve backup directory {raw}: {exc}") from exc
    if directory == Path(directory.anchor):
        raise RetentionError("refusing to manage a filesystem root as a backup directory")
    return directory


def reject_stale_quarantine(directory: Path) -> None:
    try:
        with os.scandir(directory) as iterator:
            stale = next((item.path for item in iterator if item.name.startswith(RETENTION_QUARANTINE_PREFIX)), None)
    except OSError as exc:
        raise RetentionError(f"cannot inspect backup directory for unfinished retention cleanup {directory}: {exc}") from exc
    if stale is not None:
        raise RetentionError(
            f"unfinished retention quarantine found at {stale}; inspect and restore any preserved backups before retrying"
        )


def validate_backup_name(name: str) -> None:
    match = BACKUP_NAME_RE.fullmatch(name)
    if not match:
        raise RetentionError(f"unexpected .sql.age filename; refusing to manage it: {name!r}")
    try:
        dt.datetime.strptime("".join(match.groups()), "%Y%m%d%H%M%S")
    except ValueError as exc:
        raise RetentionError(f"invalid timestamp in encrypted backup filename: {name!r}") from exc


def inventory(directory: Path) -> list[BackupEntry]:
    entries: list[BackupEntry] = []
    try:
        with os.scandir(directory) as iterator:
            for item in iterator:
                if not item.name.endswith(".sql.age"):
                    continue
                validate_backup_name(item.name)
                try:
                    info = item.stat(follow_symlinks=False)
                except OSError as exc:
                    raise RetentionError(f"cannot inspect encrypted backup {item.name!r}: {exc}") from exc
                if not stat.S_ISREG(info.st_mode):
                    raise RetentionError(f"encrypted backup is not a regular, non-symlink file: {item.name!r}")
                entries.append(
                    BackupEntry(
                        name=item.name,
                        path=Path(item.path),
                        size_bytes=info.st_size,
                        mtime_ns=info.st_mtime_ns,
                        mtime_utc=dt.datetime.fromtimestamp(info.st_mtime, dt.timezone.utc),
                        device=info.st_dev,
                        inode=info.st_ino,
                    )
                )
    except OSError as exc:
        raise RetentionError(f"cannot list backup directory {directory}: {exc}") from exc
    return sorted(entries, key=lambda entry: entry.name)


def confirmation_token(directory: Path, keep_days: int, entries: Sequence[BackupEntry]) -> str:
    payload = {
        "directory": str(directory),
        "keep_days": keep_days,
        "candidates": [entry.identity() for entry in sorted(entries, key=lambda item: item.name)],
    }
    encoded = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()[:TOKEN_HEX_LENGTH]


def scan(directory: Path, keep_days: int) -> tuple[dt.datetime, list[BackupEntry], list[BackupEntry]]:
    now = dt.datetime.now(dt.timezone.utc)
    cutoff = now - dt.timedelta(days=keep_days)
    all_entries = inventory(directory)
    candidates = [entry for entry in all_entries if entry.mtime_utc < cutoff]
    return cutoff, all_entries, candidates


def print_plan(
    directory: Path,
    keep_days: int,
    cutoff: dt.datetime,
    all_entries: Sequence[BackupEntry],
    candidates: Sequence[BackupEntry],
    token: str | None,
) -> None:
    print(f"Backup directory: {directory}")
    print(f"Retention: keep files modified within the last {keep_days} days (UTC cutoff {cutoff.isoformat()})")
    print(f"Encrypted backups found: {len(all_entries)}")
    print(f"Deletion candidates: {len(candidates)}")
    for entry in candidates:
        print(f"  {entry.name} | mtime={entry.mtime_utc.isoformat()} | size={entry.size_bytes} bytes")
    if candidates and len(candidates) == len(all_entries):
        print("BLOCKED: refusing to delete the last remaining encrypted backup; preserve at least one file.")
    elif token is not None:
        print(f"Confirmation token: {token}")
        print("No files deleted. Re-run with --apply --confirm-token <token> to apply exactly this candidate snapshot.")
    else:
        print("No deletions needed; no files were changed.")


def verify_unchanged(directory: Path, entries: Sequence[BackupEntry]) -> None:
    for planned in entries:
        try:
            current = planned.path.lstat()
        except OSError as exc:
            raise RetentionError(f"candidate changed since preview: {planned.name!r}: {exc}") from exc
        if not stat.S_ISREG(current.st_mode):
            raise RetentionError(f"candidate is no longer a regular file: {planned.name!r}")
        identity = (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns)
        expected = (planned.device, planned.inode, planned.size_bytes, planned.mtime_ns)
        if identity != expected:
            raise RetentionError(f"candidate changed since preview: {planned.name!r}")


def restore_quarantined(quarantined: Path, original: Path) -> str:
    try:
        # Hard-link creation is atomic and will not overwrite a new file at original.
        os.link(quarantined, original, follow_symlinks=False)
    except OSError as exc:
        return f"unexpected item preserved at {quarantined}; could not safely restore to {original}: {exc}"
    try:
        quarantined.unlink()
    except OSError as exc:
        return f"unexpected item restored to {original}; an extra recovery link remains at {quarantined}: {exc}"
    return f"unexpected item restored to {original}"


def apply_plan(directory: Path, entries: Sequence[BackupEntry]) -> int:
    # Validate the complete reviewed set before deleting any candidate.
    verify_unchanged(directory, entries)
    deleted = 0
    for planned in entries:
        try:
            quarantine_directory = Path(
                tempfile.mkdtemp(prefix=RETENTION_QUARANTINE_PREFIX, dir=directory)
            )
        except OSError as exc:
            raise RetentionError(f"cannot create a private retention quarantine: {exc}; already deleted {deleted}") from exc
        quarantined = quarantine_directory / planned.name
        try:
            try:
                # Rename the path atomically, then inspect the object that was actually moved.
                os.rename(planned.path, quarantined)
            except OSError as exc:
                raise RetentionError(
                    f"cannot move candidate {planned.name!r} into private quarantine: {exc}; already deleted {deleted}"
                ) from exc

            try:
                current = quarantined.lstat()
            except OSError as exc:
                raise RetentionError(
                    f"candidate moved into quarantine but could not be verified; preserved at {quarantined}: {exc}; "
                    f"already deleted {deleted}"
                ) from exc
            identity = (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns)
            expected = (planned.device, planned.inode, planned.size_bytes, planned.mtime_ns)
            if not stat.S_ISREG(current.st_mode) or identity != expected:
                recovery = restore_quarantined(quarantined, planned.path)
                raise RetentionError(
                    f"candidate changed during deletion: {planned.name!r}; {recovery}; already deleted {deleted}"
                )

            try:
                quarantined.unlink()
            except OSError as exc:
                recovery = restore_quarantined(quarantined, planned.path)
                raise RetentionError(
                    f"cannot delete verified candidate {planned.name!r}: {exc}; {recovery}; already deleted {deleted}"
                ) from exc
            deleted += 1
            print(f"Deleted: {planned.name}")
        finally:
            # If an unexpected item could not be restored, keep the non-empty quarantine for recovery.
            try:
                quarantine_directory.rmdir()
            except OSError:
                pass
    return deleted


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Preview or prune old encrypted MySQL backups. Dry-run is the default; "
            "deletion requires an explicit confirmation token for the current candidate snapshot."
        )
    )
    parser.add_argument("--backup-dir", required=True, help="explicit directory; only direct *.sql.age files are considered")
    parser.add_argument("--keep-days", type=int, default=DEFAULT_KEEP_DAYS, help="retention window in 24-hour days (minimum: 30; default: 30)")
    parser.add_argument("--apply", action="store_true", help="delete only the exact reviewed candidate set")
    parser.add_argument("--confirm-token", help="token printed by a matching dry-run; required with --apply")
    args = parser.parse_args(argv)
    if args.keep_days < DEFAULT_KEEP_DAYS:
        parser.error(f"--keep-days must be at least {DEFAULT_KEEP_DAYS}")
    if args.apply and not args.confirm_token:
        parser.error("--apply requires --confirm-token from a dry-run")
    if args.confirm_token and not args.apply:
        parser.error("--confirm-token is only valid with --apply")
    return args


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        directory = resolve_backup_dir(args.backup_dir)
        reject_stale_quarantine(directory)
        cutoff, all_entries, candidates = scan(directory, args.keep_days)
        blocked = bool(candidates) and len(candidates) == len(all_entries)
        token = confirmation_token(directory, args.keep_days, candidates) if candidates and not blocked else None
        print_plan(directory, args.keep_days, cutoff, all_entries, candidates, token)

        if not args.apply:
            return 0
        if blocked:
            raise RetentionError("refusing to delete every encrypted backup; preserve at least one file")
        if not candidates:
            raise RetentionError("there are no deletion candidates for the supplied confirmation token")
        if token is None or not hmac.compare_digest(args.confirm_token, token):
            raise RetentionError("confirmation token does not match the current candidate snapshot; nothing was deleted")

        deleted = apply_plan(directory, candidates)
        print(f"PASS: deleted {deleted} expired encrypted backup(s); retained {len(all_entries) - deleted}")
    except RetentionError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
