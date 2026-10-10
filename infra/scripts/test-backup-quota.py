import importlib.util
import json
import os
from pathlib import Path
import unittest
from types import SimpleNamespace
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("quota", Path(__file__).with_name("backup-quota.py"))
quota = importlib.util.module_from_spec(spec)
spec.loader.exec_module(quota)


class QuotaTest(unittest.TestCase):
    def test_error_identifies_stage_without_leaking_raw_output(self):
        result = SimpleNamespace(returncode=3, stderr="AccessDenied secret-key private-object-name")
        message = quota.operation_error(("size", "r2:private-bucket"), result)
        self.assertIn("R2 size failed", message)
        self.assertIn("access denied", message)
        self.assertNotIn("secret-key", message)
        self.assertNotIn("private", message)

    def test_transfer_limit_is_distinguished_from_storage_quota(self):
        result = SimpleNamespace(returncode=8, stderr="Max transfer limit reached")
        self.assertIn("rclone transfer limit reached", quota.operation_error(("copyto", "primary:private"), result))

    def simulate(self, used, objects, limit=1000, fail_size=False, growth=False):
        calls = []

        def run(*args):
            calls.append(args)
            if args[0] == "size":
                if fail_size:
                    raise quota.BackupError("Unavailable")
                return json.dumps({"bytes": used})
            if args[0] == "lsjson":
                if "--stat" in args:
                    return json.dumps({"Size": objects[0]["Size"] + int(growth)})
                return json.dumps(objects)
            return ""

        with patch.dict(os.environ, {"R2_BUCKET_NAME": "backup", "BACKUP_SOURCE_BUCKETS": "hquizlet",
                                     "BACKUP_MAX_STORAGE_BYTES": str(limit)}), patch.object(quota, "rclone", side_effect=run):
            try:
                quota.backup("run")
            except quota.BackupError:
                return calls, False
        return calls, True

    def test_crypt_boundaries(self):
        for size, expected in [(0, 32), (1, 49), (65536, 65584), (65537, 65601)]:
            self.assertEqual(quota.encrypted_size(size), expected)

    def test_full_bucket_never_uploads(self):
        calls, ok = self.simulate(1000, [{"Path": "x", "Size": 1}])
        self.assertFalse(ok)
        self.assertFalse(any(c[0] == "copyto" for c in calls))

    def test_next_file_would_cross_limit(self):
        calls, ok = self.simulate(952, [{"Path": "x", "Size": 1}])
        self.assertFalse(ok)
        self.assertFalse(any(c[0] == "copyto" for c in calls))

    def test_exact_fit_and_nested_version_path(self):
        calls, ok = self.simulate(951, [{"Path": "quiz-audio/x", "Size": 1}])
        self.assertTrue(ok)
        upload = next(c for c in calls if c[0] == "copyto")
        self.assertIn("vault:versions/run/hquizlet/quiz-audio", upload)
        self.assertEqual(upload[upload.index("--max-size") + 1], "1B")

    def test_empty_files_consume_quota(self):
        calls, ok = self.simulate(950, [{"Path": "x", "Size": 0}, {"Path": "y", "Size": 0}])
        self.assertFalse(ok)
        self.assertEqual(sum(c[0] == "copyto" for c in calls), 1)

    def test_measurement_failure_blocks_upload(self):
        calls, ok = self.simulate(0, [], fail_size=True)
        self.assertFalse(ok)
        self.assertFalse(any(c[0] == "copyto" for c in calls))

    def test_source_growth_does_not_mark_success(self):
        _, ok = self.simulate(0, [{"Path": "x", "Size": 1}], growth=True)
        self.assertFalse(ok)

    def test_cannot_raise_limit_above_9_5_gb(self):
        calls, ok = self.simulate(0, [], limit=10000000000)
        self.assertFalse(ok)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
