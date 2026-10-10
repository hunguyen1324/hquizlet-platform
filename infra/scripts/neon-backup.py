#!/usr/bin/env python3
"""Create verified, versioned Neon databases from the primary PostgreSQL database."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from urllib.parse import parse_qs, unquote, urlsplit
import uuid

NAME = re.compile(r"^hquizlet_backup_[0-9]{14}_[a-f0-9]{8}$")


class ConfigurationError(ValueError):
    """Contains only explicitly safe configuration messages, never input values."""


class ToolError(RuntimeError):
    """Contains a classified error, never raw PostgreSQL stderr."""


def diagnostic(error):
    if isinstance(error, (ConfigurationError, ToolError)):
        return str(error)
    if isinstance(error, BlockingIOError):
        return "Another worker holds the state-volume lock"
    if isinstance(error, OSError):
        return "Cannot access worker files/state volume; check permissions and free disk space"
    return "Invalid numeric configuration or snapshot history; check NEON_BACKUP_* settings"


def setting_int(key, default):
    try:
        return int(os.getenv(key, default))
    except ValueError:
        raise ConfigurationError(f"{key} must be an integer") from None


def neon_environment(uri):
    if not uri or not uri.strip():
        raise ConfigurationError("NEON_BACKUP_URL / POSTGRES_URL is missing in the container; add it to server Data .env")
    try:
        url = urlsplit(uri)
        query = parse_qs(url.query)
        ssl = query.get("sslmode", ["require"])[0]
        if (url.scheme not in {"postgres", "postgresql"} or not url.hostname
                or not url.hostname.endswith(".neon.tech") or "-pooler." in url.hostname
                or not url.username or not url.password or not url.path.strip("/")
                or ssl not in {"require", "verify-ca", "verify-full"}):
            raise ValueError()
        result = clean_environment()
        result.update(PGHOST=url.hostname, PGPORT=str(url.port or 5432),
                      PGUSER=unquote(url.username), PGPASSWORD=unquote(url.password),
                      PGDATABASE=unquote(url.path[1:]), PGSSLMODE=ssl)
        if "channel_binding" in query:
            if query["channel_binding"][0] not in {"disable", "prefer", "require"}:
                raise ValueError()
            result["PGCHANNELBINDING"] = query["channel_binding"][0]
        return result
    except (ValueError, IndexError):
        raise ConfigurationError("Set a direct Neon PostgreSQL URL with TLS; pooled URLs are not supported") from None


def clean_environment():
    # Never inherit PGDATABASE, PGSERVICE or credentials for the other connection.
    env = {key: val for key, val in os.environ.items() if not key.startswith("PG")}
    # Do not pass URL secrets to child processes; libpq receives parsed credentials only.
    env.pop("NEON_BACKUP_URL", None)
    env.pop("POSTGRES_URL", None)
    env.update(PGCONNECT_TIMEOUT="15", PGOPTIONS="-c lock_timeout=10000")
    return env


def source_environment():
    env = clean_environment()
    host = os.getenv("NEON_BACKUP_SOURCE_HOST", "postgres").strip()
    # Compose port bindings use [IPv6]; libpq PGHOST needs the unbracketed address.
    if host.startswith("[") and host.endswith("]"):
        host = host[1:-1]
    if host == "0.0.0.0":
        host = "127.0.0.1"
    elif host == "::":
        host = "::1"
    env.update(PGHOST=host,
               PGPORT=os.getenv("NEON_BACKUP_SOURCE_PORT", "5432"),
               PGUSER=os.getenv("POSTGRES_USER", "hquizlet"),
               PGDATABASE=os.getenv("POSTGRES_DB", "hquizlet"),
               PGPASSWORD=os.getenv("POSTGRES_PASSWORD", ""), PGSSLMODE="prefer")
    if not env["PGPASSWORD"]:
        raise ConfigurationError("POSTGRES_PASSWORD is required for the primary database")
    return env


def execute(args, env, sql=None):
    # Errors may contain URLs, data or credentials. Never send raw tool output to logs.
    tool = args[0] if args[0] in {"psql", "pg_dump", "pg_restore"} else "PostgreSQL client"
    endpoint = "Neon" if env.get("PGHOST", "").endswith(".neon.tech") else "primary PostgreSQL"
    try:
        result = subprocess.run(args, input=sql, text=True, capture_output=True, env=env,
                                timeout=setting_int("NEON_BACKUP_TIMEOUT_SECONDS", "7200"))
    except (OSError, subprocess.TimeoutExpired):
        raise ToolError(f"{endpoint}: {tool} unavailable or timed out") from None
    if result.returncode:
        raw = (result.stderr or "").lower()
        reason = "check credentials, permissions, extensions and database/client versions"
        for fragment, description in [
            ("password authentication failed", "password authentication failed; check credentials in server .env"),
            ("could not translate host name", "DNS lookup failed; check container network/DNS"),
            ("connection refused", "connection refused; check primary container and network"),
            ("timeout", "connection or operation timed out"),
            ("permission denied", "insufficient database permissions"),
            ("does not exist", "database or required object does not exist"),
            ("server version mismatch", "PostgreSQL client/server major version mismatch"),
            ("ssl", "TLS negotiation failed; check CA certificates and SSL settings"),
        ]:
            if fragment in raw:
                reason = description
                break
        raise ToolError(f"{endpoint}: {tool} failed ({reason})")
    return result.stdout.strip()


def sql(env, statement):
    return execute(["psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1"], env, statement)


def atomic_state(path, data):
    temporary = path.with_suffix(".tmp")
    with temporary.open("w", encoding="utf-8") as stream:
        json.dump(data, stream)
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(temporary, 0o600)
    os.replace(temporary, path)


def target_id(env):
    return hashlib.sha256((env["PGHOST"] + ":" + env["PGPORT"] + "/" + env["PGDATABASE"] + "/" + env["PGUSER"]).encode()).hexdigest()


def load_state(path, env):
    if path.exists():
        data = json.loads(path.read_text())
        if data.get("target") != target_id(env) or not re.fullmatch(r"[a-f0-9]{32}", data.get("owner", "")):
            raise ConfigurationError("Neon target changed; use a new state volume to avoid mixing backup histories")
        if any(not NAME.fullmatch(name) for name in data.get("snapshots", [])):
            raise ConfigurationError("Invalid snapshot history")
        return data
    data = {"target": target_id(env), "owner": uuid.uuid4().hex, "snapshots": [], "last_success": 0}
    atomic_state(path, data)
    return data


def backup_once(state_dir, source, target, keep=2):
    if not 1 <= keep <= 30:
        raise ConfigurationError("NEON_BACKUP_KEEP must be between 1 and 30")
    path = state_dir / "state.json"
    state = load_state(path, target)
    # If earlier retention failed, resolve it before creating more databases.
    prune(state, path, target, keep)
    name = "hquizlet_backup_" + time.strftime("%Y%m%d%H%M%S", time.gmtime()) + "_" + uuid.uuid4().hex[:8]
    marker = "hquizlet-neon-backup:v1:" + state["owner"]
    created = False
    complete = False
    with tempfile.TemporaryDirectory(prefix="run.", dir=state_dir) as directory:
        dump = str(Path(directory) / "database.dump")
        try:
            # Source remains read-only; no changes to DATABASE_URL or application routing.
            execute(["pg_dump", "--format=custom", "--compress=6", "--no-owner", "--no-acl", "--file=" + dump], source)
            execute(["pg_restore", "--list", dump], source)
            sql(target, f'CREATE DATABASE "{name}" TEMPLATE template0;')
            created = True
            sql(target, f'COMMENT ON DATABASE "{name}" IS \'{marker}\';')
            snapshot = dict(target, PGDATABASE=name)
            execute(["pg_restore", "--no-owner", "--no-acl", "--exit-on-error", "--single-transaction", "--dbname=" + name, dump], snapshot)
            sql(snapshot, "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema');")
            # Prepare planner statistics for faster reads if this snapshot is used for recovery.
            sql(snapshot, "ANALYZE;")
            complete = True
            state["snapshots"].append(name)
            state["last_success"] = int(time.time())
            state["latest_database"] = name
            atomic_state(path, state)
        finally:
            if created and not complete:
                # Only the new database created in this attempt may be removed on failure.
                try:
                    sql(target, f'DROP DATABASE "{name}";')
                except RuntimeError:
                    print(f"Incomplete snapshot needs manual cleanup: {name}", flush=True)
    prune(state, path, target, keep)
    return name


def prune(state, path, target, keep):
    marker = "hquizlet-neon-backup:v1:" + state["owner"]
    # Retention never deletes neondb or databases absent from our ownership history.
    while len(state["snapshots"]) > keep:
        old = state["snapshots"][0]
        if old == target["PGDATABASE"] or old == state["latest_database"]:
            raise ConfigurationError("Refusing to remove the admin or current snapshot database")
        actual = sql(target, f"SELECT coalesce(shobj_description(oid,'pg_database'),'') FROM pg_database WHERE datname='{old}';")
        if actual != marker:
            raise ConfigurationError("Snapshot ownership marker changed; retention stopped")
        sql(target, f'DROP DATABASE "{old}";')
        state["snapshots"].pop(0)
        atomic_state(path, state)


def main():
    os.umask(0o077)
    mode = sys.argv[1] if len(sys.argv) > 1 else "loop"
    if mode not in {"loop", "once", "check", "health"}:
        raise ConfigurationError("Usage: neon-backup.py [loop|once|check|health]")
    interval = setting_int("NEON_BACKUP_INTERVAL_SECONDS", "86400")
    retry = setting_int("NEON_BACKUP_RETRY_SECONDS", "1800")
    if interval < 60 or retry < 60:
        raise ConfigurationError("Backup intervals must be >= 60 seconds")
    state_dir = Path(os.getenv("NEON_BACKUP_STATE_DIR", "/var/lib/hquizlet-neon-backup"))
    if mode == "health":
        data = json.loads((state_dir / "state.json").read_text())
        if not data.get("last_success") or time.time() - data["last_success"] > interval * 2 + 7200:
            raise ConfigurationError("No recent successful Neon snapshot")
        return
    if os.getenv("NEON_BACKUP_ENABLED", "false") != "true":
        print("Neon backup disabled", flush=True)
        return
    target = neon_environment(os.getenv("NEON_BACKUP_URL") or os.getenv("POSTGRES_URL", ""))
    source = source_environment()
    if mode == "check":
        print("Checking primary PostgreSQL connection...", flush=True)
        sql(source, "SELECT 1;")
        print("Primary connection OK. Checking Neon connection and CREATEDB permission...", flush=True)
        if sql(target, "SELECT rolcreatedb FROM pg_roles WHERE rolname=current_user;") != "t":
            raise ConfigurationError("Neon role requires CREATEDB permission for isolated snapshots")
        print("Primary and Neon connections OK; Neon role can create snapshot databases", flush=True)
        return
    state_dir.mkdir(parents=True, exist_ok=True)
    import fcntl  # Linux worker only; do not start simultaneous snapshots in the same volume.
    with (state_dir / "worker.lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        for stale in state_dir.glob("run.*"):
            if stale.is_dir() and not stale.is_symlink():
                shutil.rmtree(stale)
        while True:
            wait = retry
            try:
                state = load_state(state_dir / "state.json", target)
                remaining = interval - (time.time() - state["last_success"])
                if remaining <= 0 or mode == "once":
                    name = backup_once(state_dir, source, target, setting_int("NEON_BACKUP_KEEP", "2"))
                    print(f"Neon database backup completed: {name}", flush=True)
                    wait = interval
                else:
                    wait = max(1, remaining)
            except (ValueError, RuntimeError, OSError) as error:
                print(f"Neon backup incomplete: {diagnostic(error)}", flush=True)
                if mode == "once":
                    raise
            if mode == "once":
                return
            time.sleep(wait)


if __name__ == "__main__":
    # Ensure TemporaryDirectory cleanup also runs when Docker sends SIGTERM.
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(143))
    try:
        main()
    except (ValueError, RuntimeError, OSError, KeyError) as error:
        print(f"Neon backup stopped: {diagnostic(error)}. No credentials printed.", file=sys.stderr)
        sys.exit(1)
