import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("neon_backup", Path(__file__).with_name("neon-backup.py"))
backup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(backup)
URI = "postgresql://owner:test-secret@ep-test.us-east-1.aws.neon.tech/neondb?sslmode=require"


class NeonBackupTest(unittest.TestCase):
    def test_host_network_source_accepts_ipv6_binding(self):
        with patch.dict(os.environ, {"NEON_BACKUP_SOURCE_HOST": "[2001:db8::10]", "POSTGRES_PASSWORD": "test"}):
            self.assertEqual(backup.source_environment()["PGHOST"], "2001:db8::10")

    def test_wildcard_binding_uses_loopback(self):
        for host, expected in [("0.0.0.0", "127.0.0.1"), ("::", "::1")]:
            with patch.dict(os.environ, {"NEON_BACKUP_SOURCE_HOST": host, "POSTGRES_PASSWORD": "test"}):
                self.assertEqual(backup.source_environment()["PGHOST"], expected)

    def test_missing_url_has_actionable_safe_message(self):
        with self.assertRaises(backup.ConfigurationError) as error:
            backup.neon_environment("")
        self.assertIn("server Data .env", backup.diagnostic(error.exception))

    def test_invalid_numeric_value_is_never_echoed(self):
        with patch.dict(os.environ, {"NEON_BACKUP_KEEP": "secret-value"}):
            with self.assertRaises(backup.ConfigurationError) as error:
                backup.setting_int("NEON_BACKUP_KEEP", "2")
        message = backup.diagnostic(error.exception)
        self.assertIn("NEON_BACKUP_KEEP", message)
        self.assertNotIn("secret-value", message)

    def test_authentication_error_is_classified_without_raw_stderr(self):
        with patch.object(backup.subprocess, "run") as run:
            run.return_value.returncode = 1
            run.return_value.stderr = 'password authentication failed: postgres://owner:secret-value@host'
            with self.assertRaises(backup.ToolError) as error:
                backup.execute(["psql"], {"PGHOST": "ep-test.neon.tech"})
        message = backup.diagnostic(error.exception)
        self.assertIn("Neon", message)
        self.assertIn("password authentication failed", message)
        self.assertNotIn("secret-value", message)

    def test_target_requires_tls_and_direct_neon_host(self):
        target = backup.neon_environment(URI)
        self.assertEqual(target["PGDATABASE"], "neondb")
        self.assertEqual(target["PGPASSWORD"], "test-secret")
        for bad in [URI.replace("require", "disable"), URI.replace("ep-test.", "ep-test-pooler."),
                    URI.replace("ep-test.us-east-1.aws.neon.tech", "localhost"), ""]:
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                backup.neon_environment(bad)

    def simulate(self, state_dir, failure=None):
        target = backup.neon_environment(URI)
        statements, commands = [], []

        def execute(args, env, sql=None):
            commands.append((args, env.copy()))
            if args[0] == "pg_dump":
                self.assertEqual(env["PGHOST"], "postgres")
                Path(next(arg[7:] for arg in args if arg.startswith("--file="))).write_bytes(b"test-dump")
            if args[0] == "pg_restore" and "--list" not in args:
                self.assertIn("--single-transaction", args)
                self.assertNotEqual(env["PGDATABASE"], "neondb")
                if failure == "restore":
                    raise RuntimeError("pg_restore failed")
            return ""

        def sql(env, query):
            statements.append((env.copy(), query))
            if "shobj_description" in query:
                data = json.loads((state_dir / "state.json").read_text())
                return "foreign-owner" if failure == "ownership" else "hquizlet-neon-backup:v1:" + data["owner"]
            if failure == "create" and query.startswith("CREATE"):
                raise RuntimeError("CREATE failed")
            return "1"

        with patch.object(backup, "execute", side_effect=execute), patch.object(backup, "sql", side_effect=sql):
            name = backup.backup_once(state_dir, {"PGHOST": "postgres"}, target)
        return name, statements, commands

    def test_success_preserves_admin_and_prunes_only_recorded_snapshots(self):
        with tempfile.TemporaryDirectory() as directory:
            state_dir = Path(directory)
            snapshots, statements = [], []
            for _ in range(3):
                name, calls, commands = self.simulate(state_dir)
                snapshots.append(name)
                statements.extend(calls)
                self.assertTrue(backup.NAME.fullmatch(name))
                self.assertFalse(any("test-secret" in str(args) for args, _ in commands))
            data = json.loads((state_dir / "state.json").read_text())
            self.assertEqual(data["snapshots"], snapshots[-2:])
            self.assertEqual(data["latest_database"], snapshots[-1])
            drops = [query for _, query in statements if query.startswith("DROP")]
            self.assertEqual(drops, [f'DROP DATABASE "{snapshots[0]}";'])
            self.assertFalse(any('"neondb"' in query for _, query in statements))
            self.assertFalse(list(state_dir.glob("run.*")))

    def test_failed_restore_keeps_previous_success(self):
        with tempfile.TemporaryDirectory() as directory:
            state_dir = Path(directory)
            self.simulate(state_dir)
            original = (state_dir / "state.json").read_bytes()
            with self.assertRaises(RuntimeError):
                self.simulate(state_dir, "restore")
            self.assertEqual((state_dir / "state.json").read_bytes(), original)
            self.assertFalse(list(state_dir.glob("run.*")))

    def test_create_failure_never_drops_existing_database(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(RuntimeError):
                self.simulate(Path(directory), "create")

    def test_foreign_marker_refuses_retention(self):
        with tempfile.TemporaryDirectory() as directory:
            state_dir = Path(directory)
            self.simulate(state_dir)
            self.simulate(state_dir)
            with self.assertRaises(ValueError):
                self.simulate(state_dir, "ownership")
            self.assertEqual(len(json.loads((state_dir / "state.json").read_text())["snapshots"]), 3)

    def test_changed_target_refuses_to_mix_backup_history(self):
        with tempfile.TemporaryDirectory() as directory:
            state_dir = Path(directory)
            self.simulate(state_dir)
            target = backup.neon_environment(URI.replace("ep-test.", "ep-other."))
            with self.assertRaises(ValueError):
                backup.load_state(state_dir / "state.json", target)

    def test_child_failure_does_not_expose_secrets(self):
        with patch.object(backup.subprocess, "run") as run:
            run.return_value.returncode = 1
            run.return_value.stderr = "test-secret"
            with self.assertRaises(RuntimeError) as error:
                backup.execute(["psql"], {"PGPASSWORD": "test-secret"})
            self.assertNotIn("test-secret", str(error.exception))


if __name__ == "__main__":
    unittest.main()
