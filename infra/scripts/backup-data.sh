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
# A killed container may leave temporary logs; remove only this worker's old run dirs.
for stale in "$state_dir"/run.*; do
  [[ -d "$stale" && ! -L "$stale" ]] || continue
  rm -rf -- "$stale"
done
interval="${BACKUP_INTERVAL_SECONDS:-900}"
[[ "$interval" =~ ^[0-9]+$ ]] && (( interval >= 60 )) || { echo 'Backup intervals must be >= 60 seconds' >&2; exit 1; }
work_dir="$(mktemp -d "$state_dir/run.XXXXXXXX")"
cleanup() { rm -rf -- "$work_dir"; }
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT

log() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*"; }
run_cycle() {
  local stamp
  stamp="$(date -u +%Y%m%dT%H%M%SZ)-${RANDOM}"
  if ! python3 "$script_dir/backup-quota.py" "$stamp"; then
    log 'Backup incomplete or storage limit reached; retry next cycle'
    return 1
  fi
  printf '%s\n' "$(date +%s)" > "$state_dir/last-success.tmp" || return 1
  mv -- "$state_dir/last-success.tmp" "$state_dir/last-success" || return 1
  log 'Encrypted MinIO backup cycle completed'
}

if [[ "$mode" == check ]]; then
  # Connectivity check only; no objects modified.
  for remote in "primary:${STORAGE_BUCKET:-hquizlet}" "r2:${R2_BUCKET_NAME}"; do
    backup_rclone lsf "$remote" --max-depth 1 > /dev/null 2>"$work_dir/check.log" || { log 'Storage connectivity check failed'; exit 1; }
  done
  log 'MinIO and R2 connectivity OK'; exit 0
fi
while true; do
  if run_cycle; then result=0; else
    result=1
    log 'Backup incomplete; user requests are unaffected'
  fi
  [[ "$mode" == once ]] && exit "$result"
  sleep "$interval" & wait $!
done
