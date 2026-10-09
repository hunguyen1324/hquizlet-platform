"""Exercise atomic secret import, cleanup and refusal to rotate recovery keys."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("installer", Path(__file__).with_name("install-backup-env.py"))
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class BackupEnvTest(unittest.TestCase):
    def payload(self):
        return "\n".join([
            "R2_ACCOUNT_ID=test", "R2_ACCESS_KEY_ID=test", "R2_SECRET_ACCESS_KEY=test",
            "R2_BUCKET_NAME=backup", "BACKUP_PROVIDER=r2", "BACKUP_ENABLED=true",
            "BACKUP_ENCRYPTION_PASSWORD=" + "a" * 64, "BACKUP_ENCRYPTION_SALT=" + "b" * 64,
        ]) + "\n"

    def test_merge_preserves_primary_and_removes_payload(self):
        with tempfile.TemporaryDirectory() as directory:
            dest, source = Path(directory) / ".env", Path(directory) / "payload.env"
            dest.write_text("# primary\nSTORAGE_PROVIDER=minio\nPOSTGRES_PASSWORD=keep\nR2_BUCKET_NAME=old\nR2_BUCKET_NAME=duplicate\n")
            source.write_text(self.payload())
            installer.merge(source, dest)
            result = dest.read_text()
            self.assertIn("STORAGE_PROVIDER=minio", result)
            self.assertIn("POSTGRES_PASSWORD=keep", result)
            self.assertEqual(result.count("R2_BUCKET_NAME="), 1)
            self.assertFalse(source.exists())
            self.assertFalse(list(Path(directory).glob(".backup-env-*")))

    def test_failure_preserves_original_and_payload(self):
        for extra in ["STORAGE_PROVIDER=r2\n", "R2_BUCKET_NAME=duplicate\n"]:
            with self.subTest(extra=extra), tempfile.TemporaryDirectory() as directory:
                dest, source = Path(directory) / ".env", Path(directory) / "payload.env"
                dest.write_text("STORAGE_PROVIDER=minio\n")
                original = dest.read_bytes()
                source.write_text(self.payload() + extra)
                with self.assertRaises(ValueError):
                    installer.merge(source, dest)
                self.assertEqual(dest.read_bytes(), original)
                self.assertTrue(source.exists())

    def test_existing_encryption_keys_cannot_be_rotated(self):
        with tempfile.TemporaryDirectory() as directory:
            dest, source = Path(directory) / ".env", Path(directory) / "payload.env"
            dest.write_text("BACKUP_ENCRYPTION_PASSWORD=" + "c" * 64 + "\n")
            original = dest.read_bytes()
            source.write_text(self.payload())
            with self.assertRaises(ValueError):
                installer.merge(source, dest)
            self.assertEqual(dest.read_bytes(), original)


if __name__ == "__main__":
    unittest.main()
