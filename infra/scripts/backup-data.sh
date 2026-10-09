#!/usr/bin/env bash
set -euo pipefail
umask 077
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/backup-common.sh"

[[ "${BACKUP_ENABLED:-false}" == true ]] || { echo 'Backup disabled'; exit 0; }
[[ "${BACKUP_PROVIDER:-r2}" == r2 ]] || { echo 'Only BACKUP_PROVIDER=r2 is supported' >&2; exit 1; }
mode="${1:-loop}"
[[ "$mode" == loop || "$mode" == once || "$mode" == check ]] || { echo 'Usage: backup-data.sh [loop|once|check]' >&2; exit 1; }
backup_configure
state_dir="${BACKUP_STATE_DIR:-/var/lib/hquizlet-backup}"
mkdir -p -- "$state_dir"
exec 9>"$state_dir/worker.lock"
flock -n 9 || { echo 'A backup worker is already running' >&2; exit 1; }
# A killed container may leave a partial dump; remove only this worker's old run dirs.
for stale in "$state_dir"/run.*; do
  [[ -d "$stale" && ! -L "$stale" ]] || continue
  rm -rf -- "$stale"
done
interval="${BACKUP_INTERVAL_SECONDS:-900}"
db_interval="${BACKUP_DATABASE_INTERVAL_SECONDS:-86400}"
[[ "$interval" =~ ^[0-9]+$ && "$db_interval" =~ ^[0-9]+$ ]] && (( interval >= 60 && db_interval >= 60 )) || { echo 'Backup intervals must be >= 60 seconds' >&2; exit 1; }
work_dir="$(mktemp -d "$state_dir/run.XXXXXXXX")"
cleanup() { rm -rf -- "$work_dir"; }
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT

log() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*"; }
run_cycle() {
  local stamp bucket last=0 now dump_due=false
  stamp="$(date -u +%Y%m%dT%H%M%SZ)-${RANDOM}"
  now="$(date +%s)"
  if [[ -f "$state_dir/database-success" ]]; then read -r last < "$state_dir/database-success"; fi
  [[ "$last" =~ ^[0-9]+$ ]] || last=0
  if (( now - last >= db_interval )); then
    dump_due=true
    # Dump before copying files so files referenced by this snapshot are covered.
    if ! pg_dump --format=custom --compress=6 --no-owner --no-acl \
      --file="$work_dir/database.dump" 2>"$work_dir/postgres.log"; then
      log 'Database dump failed; last successful backup retained'; return 1
    fi
    if ! pg_restore --list "$work_dir/database.dump" > /dev/null 2>"$work_dir/postgres.log"; then
      log 'Database dump validation failed'; return 1
    fi
  fi
  # Files are streamed from MinIO; no second full file copy on the small data disk.
  # Never use sync: files deleted locally remain recoverable in R2.
  IFS=',' read -ra buckets <<< "${BACKUP_SOURCE_BUCKETS:-${STORAGE_BUCKET:-hquizlet},${MINIO_IMPORT_BUCKET:-hquizlet-imports}}"
  for bucket in "${buckets[@]}"; do
    [[ "$bucket" =~ ^[a-z0-9][a-z0-9.-]+[a-z0-9]$ ]] || { log 'Invalid backup source bucket'; return 1; }
    # Import spreadsheets are temporary; quiz-audio in the same bucket is retained.
    if ! backup_rclone copy "primary:$bucket" "vault:files/$bucket" \
      --backup-dir "vault:versions/$stamp/$bucket" --exclude '/imports/**' \
      >"$work_dir/rclone.log" 2>&1; then
      log 'Object backup failed; retry next cycle (details kept private locally)'
      return 1
    fi
  done
  if [[ "$dump_due" == true ]]; then
    if ! backup_rclone copyto "$work_dir/database.dump" "vault:database/$stamp.dump" \
      >"$work_dir/rclone.log" 2>&1; then
      log 'Database upload failed; retry next cycle'; return 1
    fi
    (cd "$work_dir" && sha256sum database.dump > database.sha256) || return 1
    if ! backup_rclone copyto "$work_dir/database.sha256" "vault:database/$stamp.sha256" \
      >"$work_dir/rclone.log" 2>&1; then
      log 'Database checksum upload failed; retry next cycle'; return 1
    fi
    printf '%s\n' "$now" > "$state_dir/database-success.tmp" || return 1
    mv -- "$state_dir/database-success.tmp" "$state_dir/database-success" || return 1
    rm -f -- "$work_dir/database.dump" || return 1
  fi
  printf '%s\n' "$(date +%s)" > "$state_dir/last-success.tmp" || return 1
  mv -- "$state_dir/last-success.tmp" "$state_dir/last-success" || return 1
  log 'Encrypted object and database backup cycle completed'
}

if [[ "$mode" == check ]]; then
  # Connectivity check only; no objects modified.
  for remote in "primary:${STORAGE_BUCKET:-hquizlet}" "r2:${R2_BUCKET_NAME}"; do
    backup_rclone lsf "$remote" --max-depth 1 > /dev/null 2>"$work_dir/check.log" || { log 'Storage connectivity check failed'; exit 1; }
  done
  pg_isready -q || { log 'PostgreSQL readiness check failed'; exit 1; }
  log 'Storage connectivity and PostgreSQL readiness OK'; exit 0
fi
while true; do
  if run_cycle; then result=0; else
    result=1
    rm -f -- "$work_dir/database.dump" "$work_dir/database.sha256"
    log 'Backup incomplete; user requests are unaffected'
  fi
  [[ "$mode" == once ]] && exit "$result"
  sleep "$interval" & wait $!
done
