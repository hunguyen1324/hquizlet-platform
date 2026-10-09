"""Verify actual rclone encryption, old versions, deletion recovery and restore locally."""
import argparse
import os
from pathlib import Path
import secrets
import subprocess
import tempfile


def main(binary):
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        source, encrypted, restored = root / "source", root / "ciphertext", root / "restored"
        source.mkdir()
        env = os.environ.copy()
        config = root / "empty.conf"
        config.write_text("")
        env.update({
            "RCLONE_CONFIG": str(config), "RCLONE_CONFIG_VAULT_TYPE": "crypt",
            "RCLONE_CONFIG_VAULT_REMOTE": encrypted.as_posix(),
            "RCLONE_CONFIG_VAULT_FILENAME_ENCRYPTION": "standard",
            "RCLONE_CONFIG_VAULT_DIRECTORY_NAME_ENCRYPTION": "true",
        })
        for name in ["PASSWORD", "PASSWORD2"]:
            result = subprocess.run([binary, "obscure", "-"], input=secrets.token_hex(32),
                                    text=True, capture_output=True, check=True, env=env)
            env["RCLONE_CONFIG_VAULT_" + name] = result.stdout.strip()

        def command(*args):
            result = subprocess.run([binary, *args, "--stats", "0"], env=env, capture_output=True)
            if result.returncode:
                raise RuntimeError("Local rclone validation command failed")

        original = b"first private teacher file"
        (source / "lesson.txt").write_bytes(original)
        command("copy", str(source), "vault:files", "--backup-dir", "vault:versions/first")
        ciphertext = [p for p in encrypted.rglob("*") if p.is_file()]
        assert ciphertext and all(original not in p.read_bytes() for p in ciphertext)
        assert all("lesson" not in p.name for p in ciphertext)
        changed = b"updated private teacher file"
        (source / "lesson.txt").write_bytes(changed)
        command("copy", str(source), "vault:files", "--backup-dir", "vault:versions/second")
        (source / "lesson.txt").unlink()
        command("copy", str(source), "vault:files", "--backup-dir", "vault:versions/third")
        command("copy", "vault:files", str(restored))
        assert (restored / "lesson.txt").read_bytes() == changed
        command("copyto", "vault:versions/second/lesson.txt", str(root / "old.txt"))
        assert (root / "old.txt").read_bytes() == original
        print("Actual rclone encryption, version preservation and restore passed (local only).")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--rclone", required=True)
    main(parser.parse_args().rclone)
