#!/usr/bin/env bash
# Deterministic failure/retry tests; does not contact MinIO, PostgreSQL or R2.
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
if [[ "${FAIL_OBJECT:-0}" == 1 && "$1" == copy ]]; then exit 1; fi
if [[ "${FAIL_UPLOAD:-0}" == 1 && "$1" == copyto ]]; then exit 1; fi
exit 0
MOCK
cat > "$test_dir/bin/pg_dump" <<'MOCK'
#!/usr/bin/env bash
echo dump >> "$CALL_LOG"
[[ "${FAIL_DUMP:-0}" == 1 ]] && exit 1
for arg in "$@"; do [[ "$arg" == --file=* ]] && printf 'dump-data' > "${arg#--file=}"; done
exit 0
MOCK
cat > "$test_dir/bin/pg_restore" <<'MOCK'
#!/usr/bin/env bash
[[ "${FAIL_VALIDATE:-0}" == 1 ]] && exit 1
exit 0
MOCK
# Git Bash has no flock; production image installs util-linux for real locking.
printf '#!/usr/bin/env bash\nexit 0\n' > "$test_dir/bin/flock"
chmod +x "$test_dir/bin/"*
export PATH="$test_dir/bin:$PATH" BACKUP_STATE_DIR="$test_dir/state"
export BACKUP_ENABLED=true R2_ACCOUNT_ID=test R2_ACCESS_KEY_ID=test R2_SECRET_ACCESS_KEY=test
export R2_BUCKET_NAME=backup BACKUP_ENCRYPTION_PASSWORD=test-password BACKUP_ENCRYPTION_SALT=test-salt
export MINIO_ROOT_USER=test MINIO_ROOT_PASSWORD=test POSTGRES_PASSWORD=test

# Success creates durable markers, backups both buckets and skips temporary imports.
bash "$script_dir/backup-data.sh" once > /dev/null
[[ -f "$BACKUP_STATE_DIR/last-success" && -f "$BACKUP_STATE_DIR/database-success" ]]
grep -q 'copy primary:hquizlet vault:files/hquizlet' "$CALL_LOG"
grep -q 'copy primary:hquizlet-imports vault:files/hquizlet-imports' "$CALL_LOG"
grep -q -- '--exclude /imports/\*\*' "$CALL_LOG"
grep -q -- '--backup-dir vault:versions/' "$CALL_LOG"
! grep -Eq '^(sync|delete|purge) ' "$CALL_LOG"
[[ "$(grep -c '^dump$' "$CALL_LOG")" == 1 ]]
bash "$script_dir/backup-data.sh" once > /dev/null
[[ "$(grep -c '^dump$' "$CALL_LOG")" == 1 ]] # DB interval avoids repeated dumps.

# Every failure leaves no success marker and cleans temporary plaintext dumps.
for failure in FAIL_OBJECT FAIL_DUMP FAIL_UPLOAD FAIL_VALIDATE; do
  rm -f "$BACKUP_STATE_DIR/last-success" "$BACKUP_STATE_DIR/database-success"
  if env "$failure=1" bash "$script_dir/backup-data.sh" once > /dev/null; then
    echo "Expected failure: $failure" >&2; exit 1
  fi
  [[ ! -f "$BACKUP_STATE_DIR/last-success" && ! -f "$BACKUP_STATE_DIR/database-success" ]]
  [[ -z "$(find "$BACKUP_STATE_DIR" -name database.dump -print)" ]]
done
bash "$script_dir/backup-data.sh" once > /dev/null # Retry succeeds.
[[ -f "$BACKUP_STATE_DIR/last-success" ]]
echo 'Backup worker success, failure, retry and cleanup tests passed.'
