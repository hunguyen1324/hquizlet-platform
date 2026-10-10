#!/usr/bin/env bash
# Deterministic failure/retry tests; does not contact MinIO or R2.
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf -- "$test_dir"' EXIT
mkdir -p "$test_dir/bin" "$test_dir/state"
export CALL_LOG="$test_dir/calls"
cat > "$test_dir/bin/rclone" <<'MOCK'
#!/usr/bin/env bash
if [[ "$1" == obscure ]]; then cat > /dev/null; echo obscured-test; exit 0; fi
printf '%s\n' "$*" >> "$CALL_LOG"
if [[ "$1" == size ]]; then echo '{"bytes":0}'; exit 0; fi
if [[ "$1" == lsjson ]]; then
  if [[ " $* " == *" --stat "* ]]; then echo '{"Size":1}'; else echo '[{"Path":"quiz-audio/test","Size":1}]'; fi
  exit 0
fi
if [[ "${FAIL_OBJECT:-0}" == 1 && "$1" == copyto ]]; then exit 1; fi
if [[ "${FAIL_UPLOAD:-0}" == 1 && "$1" == copyto ]]; then exit 1; fi
exit 0
MOCK
# Git Bash has no flock; production image installs util-linux for real locking.
printf '#!/usr/bin/env bash\nexit 0\n' > "$test_dir/bin/flock"
# Windows cannot execute the Bash rclone mock directly from Python.
if [[ "${OS:-}" == Windows_NT ]]; then
  export MOCK_RCLONE="$(cygpath -w "$test_dir/bin/rclone")"
  export MOCK_BASH="$(cygpath -w "$(command -v bash)")"
  cat > "$test_dir/bridge.py" <<'BRIDGE'
import os, runpy, subprocess, sys
original = subprocess.run
def run(command, **kwargs):
    if command[0] == "rclone":
        command = [os.environ["MOCK_BASH"], os.environ["MOCK_RCLONE"], *command[1:]]
    return original(command, **kwargs)
subprocess.run = run
sys.argv = sys.argv[1:]
runpy.run_path(sys.argv[0], run_name="__main__")
BRIDGE
  export PYTHON_BRIDGE="$(cygpath -w "$test_dir/bridge.py")"
  printf '#!/usr/bin/env bash\nexec python "$PYTHON_BRIDGE" "$@"\n' > "$test_dir/bin/python3"
fi
chmod +x "$test_dir/bin/"*
export PATH="$test_dir/bin:$PATH" BACKUP_STATE_DIR="$test_dir/state"
export BACKUP_ENABLED=true R2_ACCOUNT_ID=test R2_ACCESS_KEY_ID=test R2_SECRET_ACCESS_KEY=test
export R2_BUCKET_NAME=backup BACKUP_ENCRYPTION_PASSWORD=test-password BACKUP_ENCRYPTION_SALT=test-salt
export MINIO_ROOT_USER=test MINIO_ROOT_PASSWORD=test

# Success creates durable markers, backups both buckets and skips temporary imports.
bash "$script_dir/backup-data.sh" once > /dev/null
[[ -f "$BACKUP_STATE_DIR/last-success" ]]
grep -q 'copyto primary:hquizlet/quiz-audio/test vault:files/hquizlet/quiz-audio/test' "$CALL_LOG"
grep -q 'copyto primary:hquizlet-imports/quiz-audio/test vault:files/hquizlet-imports/quiz-audio/test' "$CALL_LOG"
grep -q -- '--exclude /imports/\*\*' "$CALL_LOG"
grep -q -- '--backup-dir vault:versions/' "$CALL_LOG"
! grep -Eq '^(sync|delete|purge) ' "$CALL_LOG"
! grep -Eq 'database|dump|pg_restore|pg_dump' "$CALL_LOG"

# Failure leaves no success marker. R2 receives no database dump.
for failure in FAIL_OBJECT; do
  rm -f "$BACKUP_STATE_DIR/last-success"
  if env "$failure=1" bash "$script_dir/backup-data.sh" once > /dev/null; then
    echo "Expected failure: $failure" >&2; exit 1
  fi
  [[ ! -f "$BACKUP_STATE_DIR/last-success" ]]
  [[ -z "$(find "$BACKUP_STATE_DIR" -name database.dump -print)" ]]
done
bash "$script_dir/backup-data.sh" once > /dev/null # Retry succeeds.
[[ -f "$BACKUP_STATE_DIR/last-success" ]]
echo 'Backup worker success, failure, retry and cleanup tests passed.'
