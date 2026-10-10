#!/usr/bin/env python3
"""Cooperative bucket quota: reserve encrypted bytes before each object upload."""
import json
import os
import re
import subprocess
import sys


class BackupError(RuntimeError):
    pass


def operation_error(args, result):
    operation = args[0] if args[0] in {"size", "lsjson", "copyto"} else "operation"
    endpoint = "R2" if args[1].startswith("r2:") else "MinIO"
    if operation == "copyto":
        endpoint = "MinIO -> encrypted R2"
    details = result.stderr.lower()
    reason = "storage request failed"
    for needles, explanation in [
        (("max transfer", "max-transfer"), "rclone transfer limit reached"),
        (("accessdenied", "access denied", "statuscode: 403", "status code: 403"), "access denied; check bucket permissions"),
        (("nosuchbucket", "bucket does not exist", "bucket not found"), "bucket not found"),
        (("invalidaccesskeyid", "signaturedoesnotmatch"), "storage credentials or signature rejected"),
        (("unknown flag", "invalid argument", "invalid suffix", "failed to parse"), "invalid rclone flag or setting"),
        (("timeout", "deadline exceeded", "no such host", "connection refused"), "storage network connection failed"),
        (("corrupted on transfer", "sizes differ"), "object changed or upload integrity check failed"),
    ]:
        if any(needle in details for needle in needles):
            reason = explanation
            break
    return f"{endpoint} {operation} failed (rclone exit {result.returncode}): {reason}; uploads stopped"


def rclone(*args):
    command = ["rclone", *args, "--transfers", "1", "--checkers", "2",
               "--buffer-size", "4M", "--s3-upload-concurrency", "1",
               "--s3-chunk-size", "8M", "--multi-thread-streams", "0",
               "--max-backlog", "1000", "--bwlimit", os.getenv("BACKUP_BWLIMIT", "5M"),
               "--tpslimit", os.getenv("BACKUP_TPSLIMIT", "10"),
               "--retries", "1", "--low-level-retries", "5",
               "--contimeout", "15s", "--timeout", "2m", "--stats", "0"]
    result = subprocess.run(command, capture_output=True, text=True)
    if result.returncode:
        # rclone diagnostics can contain private object names or credentials.
        raise BackupError(operation_error(args, result))
    return result.stdout


def encrypted_size(size):
    if type(size) is not int or size < 0:
        raise BackupError("Invalid object size; uploads stopped")
    return size + 32 + 16 * ((size + 65535) // 65536)


def bucket_size(bucket):
    data = json.loads(rclone("size", "r2:" + bucket, "--json"))
    size = data["bytes"]
    if type(size) is not int or size < 0:
        raise BackupError("Cannot measure bucket usage; uploads stopped")
    return size


def backup(stamp):
    limit = int(os.getenv("BACKUP_MAX_STORAGE_BYTES", "9500000000"))
    if not 0 < limit <= 9500000000:
        raise BackupError("BACKUP_MAX_STORAGE_BYTES must be between 1 and 9500000000")
    bucket = os.environ["R2_BUCKET_NAME"]
    used = bucket_size(bucket)  # Raw encrypted storage, all prefixes and versions.
    if used >= limit:
        raise BackupError(f"R2 storage limit reached ({used}/{limit} bytes); uploads paused")
    sources = os.getenv("BACKUP_SOURCE_BUCKETS") or (
        os.getenv("STORAGE_BUCKET", "hquizlet") + "," + os.getenv("MINIO_IMPORT_BUCKET", "hquizlet-imports"))
    for source in sources.split(","):
        if not re.fullmatch(r"[a-z0-9][a-z0-9.-]+[a-z0-9]", source):
            raise BackupError("Invalid backup source bucket")
        objects = json.loads(rclone("lsjson", "primary:" + source, "--recursive", "--files-only",
                                    "--exclude", "/imports/**"))
        for obj in objects:
            path = obj["Path"]
            if not isinstance(path, str) or not path or path.startswith("/") or any(
                    part in {".", ".."} for part in path.split("/")):
                raise BackupError("Invalid object path; uploads stopped")
            size = obj["Size"]
            reserve = encrypted_size(size)
            # Reserve cumulatively from the bucket measurement at cycle start.
            # Only one writer may use this bucket; avoid a full LIST for every file.
            # Never refund reservations within a cycle, including unchanged objects.
            if used + reserve > limit:
                raise BackupError(f"R2 storage limit would be exceeded ({used}/{limit} bytes); uploads paused")
            parent = path.rpartition("/")[0]
            versions = f"vault:versions/{stamp}/{source}" + ("/" + parent if parent else "")
            # copyto retains rclone's unchanged-object checks. --max-size rejects a
            # source that grew since enumeration; smaller objects need less reservation.
            # Empty files still consume the crypt header and are reserved above.
            rclone("copyto", f"primary:{source}/{path}", f"vault:files/{source}/{path}",
                   "--backup-dir", versions, "--max-size", str(size) + "B",
                   "--max-transfer", str(size + 1) + "B", "--cutoff-mode", "CAUTIOUS")
            used += reserve
            # Ensure a growing source skipped by --max-size cannot mark success.
            current = json.loads(rclone("lsjson", f"primary:{source}/{path}", "--stat"))
            if current["Size"] > size:
                raise BackupError("Source object grew during backup; retry next cycle")


if __name__ == "__main__":
    try:
        backup(sys.argv[1])
    except BackupError as error:
        print(str(error), flush=True)
        sys.exit(1)
    except (ValueError, KeyError, TypeError, OSError, IndexError):
        print("Invalid quota configuration or storage response; uploads stopped", flush=True)
        sys.exit(1)
