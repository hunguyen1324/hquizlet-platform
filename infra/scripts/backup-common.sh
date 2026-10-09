#!/usr/bin/env bash
# Sourced by backup and recovery commands; never enable shell tracing here.
set -euo pipefail
umask 077

backup_configure() {
  : "${R2_ACCESS_KEY_ID:?R2_ACCESS_KEY_ID is required}"
  : "${R2_SECRET_ACCESS_KEY:?R2_SECRET_ACCESS_KEY is required}"
  : "${R2_BUCKET_NAME:?R2_BUCKET_NAME is required}"
  : "${BACKUP_ENCRYPTION_PASSWORD:?BACKUP_ENCRYPTION_PASSWORD is required}"
  : "${BACKUP_ENCRYPTION_SALT:?BACKUP_ENCRYPTION_SALT is required}"
  : "${MINIO_ROOT_USER:?MINIO_ROOT_USER is required}"
  : "${MINIO_ROOT_PASSWORD:?MINIO_ROOT_PASSWORD is required}"
  local endpoint="${R2_ENDPOINT:-}"
  if [[ -z "$endpoint" ]]; then
    : "${R2_ACCOUNT_ID:?R2_ACCOUNT_ID or R2_ENDPOINT is required}"
    endpoint="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"
  fi
  [[ "$endpoint" == https://* ]] || { echo 'R2 requires HTTPS' >&2; return 1; }
  local prefix="${BACKUP_PREFIX:-hquizlet-backup}"
  [[ "$prefix" =~ ^[a-zA-Z0-9_-]+$ ]] || { echo 'Invalid BACKUP_PREFIX' >&2; return 1; }
  export RCLONE_CONFIG=/dev/null
  export RCLONE_CONFIG_PRIMARY_TYPE=s3 RCLONE_CONFIG_PRIMARY_PROVIDER=Minio
  export RCLONE_CONFIG_PRIMARY_ENDPOINT="${BACKUP_MINIO_ENDPOINT:-http://minio:9000}"
  export RCLONE_CONFIG_PRIMARY_ACCESS_KEY_ID="$MINIO_ROOT_USER"
  export RCLONE_CONFIG_PRIMARY_SECRET_ACCESS_KEY="$MINIO_ROOT_PASSWORD"
  export RCLONE_CONFIG_PRIMARY_REGION=us-east-1 RCLONE_CONFIG_PRIMARY_FORCE_PATH_STYLE=true
  export RCLONE_CONFIG_R2_TYPE=s3 RCLONE_CONFIG_R2_PROVIDER=Cloudflare
  export RCLONE_CONFIG_R2_ENDPOINT="$endpoint" RCLONE_CONFIG_R2_REGION=auto
  export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
  export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
  export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true RCLONE_CONFIG_R2_FORCE_PATH_STYLE=true
  export RCLONE_CONFIG_VAULT_TYPE=crypt
  export RCLONE_CONFIG_VAULT_REMOTE="r2:${R2_BUCKET_NAME}/${prefix}"
  export RCLONE_CONFIG_VAULT_FILENAME_ENCRYPTION=standard
  export RCLONE_CONFIG_VAULT_DIRECTORY_NAME_ENCRYPTION=true
  # Feed secrets through stdin rather than process command-line arguments.
  RCLONE_CONFIG_VAULT_PASSWORD="$(printf '%s' "$BACKUP_ENCRYPTION_PASSWORD" | rclone obscure -)"
  RCLONE_CONFIG_VAULT_PASSWORD2="$(printf '%s' "$BACKUP_ENCRYPTION_SALT" | rclone obscure -)"
  export RCLONE_CONFIG_VAULT_PASSWORD RCLONE_CONFIG_VAULT_PASSWORD2
  export PGHOST="${BACKUP_POSTGRES_HOST:-postgres}" PGPORT="${BACKUP_POSTGRES_PORT:-5432}"
  export PGUSER="${POSTGRES_USER:-hquizlet}" PGDATABASE="${POSTGRES_DB:-hquizlet}"
  export PGPASSWORD="${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
  export PGCONNECT_TIMEOUT=15 PGOPTIONS='-c statement_timeout=0 -c lock_timeout=10000'
}

backup_rclone() {
  rclone "$@" --transfers 1 --checkers 2 --buffer-size 4M \
    --s3-upload-concurrency 1 --s3-chunk-size 8M --multi-thread-streams 0 \
    --max-backlog 1000 --bwlimit "${BACKUP_BWLIMIT:-5M}" --tpslimit "${BACKUP_TPSLIMIT:-10}" \
    --retries 3 --low-level-retries 5 --contimeout 15s --timeout 2m --stats 0
}
