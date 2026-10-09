"""Round-trip a synthetic encrypted fixture on R2, then remove only its unique prefix."""
import argparse
import os
from pathlib import Path
import re
import subprocess
import tempfile
import uuid


def main(env_file, binary):
    settings = {}
    for line in env_file.read_text(encoding="utf-8-sig").splitlines():
        match = re.fullmatch(r"([A-Z_0-9]+)=(.*)", line)
        if match:
            settings[match[1]] = match[2].strip().strip("\"'")
    endpoint = settings.get("R2_ENDPOINT") or "https://" + settings["R2_ACCOUNT_ID"] + ".r2.cloudflarestorage.com"
    prefix = settings.get("BACKUP_PREFIX", "hquizlet-backup")
    if not re.fullmatch(r"[a-zA-Z0-9_-]+", prefix) or not endpoint.startswith("https://"):
        raise ValueError("Invalid backup configuration")
    # Dedicated namespace: never touches existing snapshots or objects.
    remote = "r2:" + settings["R2_BUCKET_NAME"] + "/" + prefix + "/validation-" + uuid.uuid4().hex
    with tempfile.TemporaryDirectory() as directory:
        local = Path(directory)
        config = local / "empty.conf"
        config.write_text("")
        environment = os.environ.copy()
        environment.update({
            "RCLONE_CONFIG": str(config), "RCLONE_CONFIG_R2_TYPE": "s3",
            "RCLONE_CONFIG_R2_PROVIDER": "Cloudflare", "RCLONE_CONFIG_R2_REGION": "auto",
            "RCLONE_CONFIG_R2_ENDPOINT": endpoint,
            "RCLONE_CONFIG_R2_ACCESS_KEY_ID": settings["R2_ACCESS_KEY_ID"],
            "RCLONE_CONFIG_R2_SECRET_ACCESS_KEY": settings["R2_SECRET_ACCESS_KEY"],
            "RCLONE_CONFIG_R2_NO_CHECK_BUCKET": "true",
            "RCLONE_CONFIG_R2_FORCE_PATH_STYLE": "true",
            "RCLONE_CONFIG_VAULT_TYPE": "crypt", "RCLONE_CONFIG_VAULT_REMOTE": remote,
            "RCLONE_CONFIG_VAULT_FILENAME_ENCRYPTION": "standard",
            "RCLONE_CONFIG_VAULT_DIRECTORY_NAME_ENCRYPTION": "true",
        })
        for name, key in [("PASSWORD", "BACKUP_ENCRYPTION_PASSWORD"), ("PASSWORD2", "BACKUP_ENCRYPTION_SALT")]:
            result = subprocess.run([binary, "obscure", "-"], input=settings[key],
                                    text=True, capture_output=True, check=True, env=environment)
            environment["RCLONE_CONFIG_VAULT_" + name] = result.stdout.strip()

        def command(*args):
            result = subprocess.run([binary, *args, "--stats", "0", "--retries", "1",
                                     "--contimeout", "15s", "--timeout", "30s"],
                                    env=environment, capture_output=True)
            if result.returncode:
                raise RuntimeError("R2 validation failed; SDK output suppressed to protect credentials")

        fixture = local / "fixture.txt"
        fixture.write_bytes(b"HQuizlet synthetic backup validation - no user data\n")
        try:
            command("copyto", str(fixture), "vault:fixture.txt")
            restored = local / "restored.txt"
            command("copyto", "vault:fixture.txt", str(restored))
            if restored.read_bytes() != fixture.read_bytes():
                raise RuntimeError("R2 restore content mismatch")
        finally:
            # vault points only at the fresh validation UUID above, not the backup root.
            try:
                command("purge", "vault:")
            except RuntimeError:
                print("Validation cleanup failed; remove dedicated prefix: " + remote)
                raise
        print("R2 encrypted upload/download verified; synthetic remote fixture removed.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--env", type=Path, required=True)
    parser.add_argument("--rclone", required=True)
    args = parser.parse_args()
    try:
        main(args.env, args.rclone)
    except (KeyError, ValueError, OSError, RuntimeError, subprocess.CalledProcessError):
        parser.exit(1, "R2 encrypted backup validation failed; no secrets printed.\n")
