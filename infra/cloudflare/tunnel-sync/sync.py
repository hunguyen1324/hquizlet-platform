#!/usr/bin/env python3
"""Update only UPSTREAM_ORIGIN using Cloudflare's dedicated secret API."""
import argparse
import json
import os
import re
import stat
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

EXPECTED = {"gateway", "auth", "study", "quiz", "class", "payment", "file"}


def docker(*args):
    return subprocess.check_output(["docker", *args], stderr=subprocess.STDOUT,
                                   timeout=20, text=True)


def tunnel_origin(logs):
    matches = re.findall(r"https://[a-zA-Z0-9-]+\.trycloudflare\.com", logs)
    if not matches:
        raise RuntimeError("Current tunnel session has no URL yet")
    return matches[-1]


def healthy(origin, secret=None):
    headers = {"Cache-Control": "no-cache"}
    if secret:
        headers["X-Origin-Proxy-Secret"] = secret
    request = urllib.request.Request(origin.rstrip("/") + "/api/healthz/services", headers=headers)
    with urllib.request.urlopen(request, timeout=15) as response:
        body = json.load(response)
    services = {item["name"]: item["status"] for item in body.get("services", [])}
    if not EXPECTED.issubset(services) or any(services[name] != "ok" for name in EXPECTED):
        raise RuntimeError("Health response does not contain seven healthy services")


def set_origin(config, origin):
    endpoint = ("https://api.cloudflare.com/client/v4/accounts/" + config["account_id"] +
                "/workers/scripts/" + config["worker_name"] + "/secrets")
    body = json.dumps({"name": "UPSTREAM_ORIGIN", "text": origin, "type": "secret_text"}).encode()
    request = urllib.request.Request(endpoint, data=body, method="PUT", headers={
        "Authorization": "Bearer " + config["api_token"], "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=25) as response:
        result = json.load(response)
    if not result.get("success"):
        raise RuntimeError("Cloudflare rejected upstream update")


def sync(config, state_path, dry_run=False):
    if not re.fullmatch(r"[a-fA-F0-9]{32}", config["account_id"]):
        raise RuntimeError("Invalid account ID")
    if not re.fullmatch(r"[a-zA-Z0-9_-]+", config["worker_name"]):
        raise RuntimeError("Invalid Worker name")
    if not re.fullmatch(r"https://[a-zA-Z0-9.-]+\.workers\.dev/?", config["public_origin"]):
        raise RuntimeError("Invalid public Worker origin")
    ids = docker("ps", "-q", "--filter", "label=com.docker.compose.project=" +
                 config.get("compose_project", "docker"), "--filter",
                 "label=com.docker.compose.service=cloudflared-quick").split()
    if len(ids) != 1:
        raise RuntimeError("Expected exactly one running Quick Tunnel container")
    started = docker("inspect", "--format", "{{.State.StartedAt}}", ids[0]).strip()
    origin = tunnel_origin(docker("logs", "--since", started, ids[0]))
    healthy(origin, config["origin_proxy_secret"])
    previous = json.loads(state_path.read_text()) if state_path.exists() else {}
    if previous.get("origin") == origin:
        try:
            healthy(config["public_origin"])
            print("Upstream unchanged; public health OK")
            return
        except Exception:
            pass  # Reconcile dashboard changes even when the tunnel URL is unchanged.
    if dry_run:
        print("Dry run: verified current tunnel; no Cloudflare changes")
        return
    rollback_origin = previous.get("origin") or config.get("initial_origin")
    set_origin(config, origin)
    for attempt in range(4):
        try:
            healthy(config["public_origin"])
            break
        except Exception:
            if attempt == 3:
                if rollback_origin and rollback_origin != origin:
                    set_origin(config, rollback_origin)
                raise RuntimeError("Public health failed after update; state not saved") from None
            time.sleep(5)
    state_path.parent.mkdir(parents=True, exist_ok=True)
    temporary = state_path.with_suffix(".tmp")
    temporary.write_text(json.dumps({"origin": origin, "container": ids[0], "started": started}))
    temporary.chmod(0o600)
    temporary.replace(state_path)
    print("Upstream updated; seven services healthy through Worker")


def main():
    import fcntl  # Linux server only; tests can import the module on Windows.
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", default="/etc/hquizlet/tunnel-sync.json")
    parser.add_argument("--state", default="/var/lib/hquizlet-tunnel-sync/state.json")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    config_path = Path(args.config)
    if stat.S_IMODE(config_path.stat().st_mode) & 0o077:
        raise RuntimeError("Config must be readable only by its owner (chmod 600)")
    state_path = Path(args.state)
    state_path.parent.mkdir(parents=True, exist_ok=True)
    with (state_path.parent / "sync.lock").open("w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("Another sync is running")
            return
        sync(json.loads(config_path.read_text()), state_path, args.dry_run)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        # Avoid printing API responses, request headers or configuration secrets.
        print("Tunnel sync failed: " + type(exc).__name__ +
              (": " + str(exc) if isinstance(exc, RuntimeError) else ""), file=sys.stderr)
        sys.exit(1)
