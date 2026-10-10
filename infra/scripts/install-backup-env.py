#!/usr/bin/env python3
"""Atomically merge a temporary backup-only env payload and remove it on success."""
import argparse
import os
from pathlib import Path
import re
import tempfile

ALLOWED = {
    "R2_ACCOUNT_ID", "R2_ENDPOINT", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY",
    "R2_BUCKET_NAME", "BACKUP_PROVIDER", "BACKUP_ENABLED", "BACKUP_PREFIX",
    "BACKUP_ENCRYPTION_PASSWORD", "BACKUP_ENCRYPTION_SALT",
    "BACKUP_INTERVAL_SECONDS", "BACKUP_MAX_STORAGE_BYTES",
    "BACKUP_BWLIMIT", "BACKUP_TPSLIMIT", "BACKUP_SOURCE_BUCKETS",
    "NEON_BACKUP_URL", "POSTGRES_URL", "NEON_BACKUP_ENABLED",
    "NEON_BACKUP_INTERVAL_SECONDS", "NEON_BACKUP_RETRY_SECONDS",
    "NEON_BACKUP_KEEP", "NEON_BACKUP_TIMEOUT_SECONDS",
}
REQUIRED = {
    "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_BUCKET_NAME",
    "BACKUP_ENCRYPTION_PASSWORD", "BACKUP_ENCRYPTION_SALT",
}
LINE = re.compile(r"^([A-Z_0-9]+)=(.*)$")


def merge(payload: Path, target: Path) -> None:
    if payload.is_symlink() or target.is_symlink():
        raise ValueError("Symlink paths are not allowed")
    payload = payload.resolve(strict=True)
    target = target.resolve(strict=True)
    if payload == target:
        raise ValueError("Input and destination must differ")
    incoming = {}
    for line in payload.read_text(encoding="utf-8-sig").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        match = LINE.fullmatch(line)
        if not match or match[1] not in ALLOWED or match[1] in incoming:
            raise ValueError("Invalid or duplicate backup variable")
        incoming[match[1]] = match[2]
    def value(key):
        return incoming.get(key, "").strip().strip("\"'")
    if any(not value(key) for key in REQUIRED):
        raise ValueError("Missing required backup credentials or encryption keys")
    if not value("R2_ACCOUNT_ID") and not value("R2_ENDPOINT"):
        raise ValueError("R2 account or endpoint is required")
    if len(value("BACKUP_ENCRYPTION_PASSWORD")) < 32 or len(value("BACKUP_ENCRYPTION_SALT")) < 32:
        raise ValueError("Encryption keys must contain at least 32 characters")
    if value("BACKUP_PROVIDER") != "r2" or value("BACKUP_ENABLED") != "true":
        raise ValueError("Payload must enable R2 backup")
    old = target.read_text(encoding="utf-8-sig").splitlines()
    # Never silently rotate keys: existing backups require their original keys.
    for line in old:
        match = LINE.fullmatch(line)
        if match and match[1] in {"BACKUP_ENCRYPTION_PASSWORD", "BACKUP_ENCRYPTION_SALT"}:
            existing = match[2].strip().strip("\"'")
            if existing and existing != value(match[1]):
                raise ValueError("Existing backup encryption keys differ; migration required")
    output = []
    emitted = set()
    for line in old:
        match = LINE.fullmatch(line)
        if match and match[1] in incoming:
            if match[1] not in emitted:
                output.append(f"{match[1]}={incoming[match[1]]}")
                emitted.add(match[1])
        else:
            output.append(line)
    output.extend(f"{key}={val}" for key, val in incoming.items() if key not in emitted)
    # Keep destination owner; secure permissions before its atomic replacement.
    stat = target.stat()
    handle, name = tempfile.mkstemp(prefix=".backup-env-", dir=target.parent)
    try:
        os.chmod(name, 0o600)
        if hasattr(os, "chown"):
            os.chown(name, stat.st_uid, stat.st_gid)
        with os.fdopen(handle, "w", encoding="utf-8", newline="\n") as stream:
            stream.write("\n".join(output) + "\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, target)
        payload.unlink()
    finally:
        if os.path.exists(name):
            os.unlink(name)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--env", type=Path, required=True)
    args = parser.parse_args()
    try:
        merge(args.input, args.env)
    except (ValueError, OSError):
        parser.exit(1, "Backup env import failed; check paths, payload and existing keys. No credentials printed.\n")
    print("Backup variables installed; temporary payload removed.")
