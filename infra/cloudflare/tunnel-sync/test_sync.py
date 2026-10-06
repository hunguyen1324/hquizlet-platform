import importlib.util
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("tunnel_sync", Path(__file__).with_name("sync.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SyncTests(unittest.TestCase):
    def config(self):
        return {"account_id": "a" * 32, "worker_name": "hquizlet",
                "public_origin": "https://app.workers.dev", "origin_proxy_secret": "test",
                "initial_origin": "https://old.trycloudflare.com"}

    def docker(self, *args):
        if args[0] == "ps":
            return "container\n"
        if args[0] == "inspect":
            return "2026-10-06T00:00:00Z\n"
        self.assertEqual(args, ("logs", "--since", "2026-10-06T00:00:00Z", "container"))
        return "URL https://new.trycloudflare.com"

    def test_dry_run_does_not_mutate(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "docker", self.docker), \
                patch.object(module, "healthy"), patch.object(module, "set_origin") as update:
            state = Path(directory) / "state.json"
            module.sync(self.config(), state, True)
            update.assert_not_called()
            self.assertFalse(state.exists())

    def test_failed_candidate_never_updates(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "docker", self.docker), \
                patch.object(module, "healthy", side_effect=RuntimeError("unhealthy")), \
                patch.object(module, "set_origin") as update:
            with self.assertRaises(RuntimeError):
                module.sync(self.config(), Path(directory) / "state.json")
            update.assert_not_called()

    def test_public_failure_rolls_back_without_saving(self):
        def health(origin, secret=None):
            if origin == "https://app.workers.dev":
                raise RuntimeError("offline")
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "docker", self.docker), \
                patch.object(module, "healthy", health), patch.object(module.time, "sleep"), \
                patch.object(module, "set_origin") as update:
            state = Path(directory) / "state.json"
            with self.assertRaises(RuntimeError):
                module.sync(self.config(), state)
            self.assertEqual([call.args[1] for call in update.call_args_list],
                             ["https://new.trycloudflare.com", "https://old.trycloudflare.com"])
            self.assertFalse(state.exists())

    def test_success_saves_and_next_run_does_not_update(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "docker", self.docker), \
                patch.object(module, "healthy"), patch.object(module, "set_origin") as update:
            state = Path(directory) / "state.json"
            module.sync(self.config(), state)
            module.sync(self.config(), state)
            self.assertTrue(state.exists())
            self.assertEqual(update.call_count, 1)


if __name__ == "__main__":
    unittest.main()
