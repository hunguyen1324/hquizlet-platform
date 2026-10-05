#!/usr/bin/env bash
# Uses the existing Data server .env. Does not modify passwords or firewall.
set -euo pipefail
cd "${INSTALL_DIR:-/opt/hquizlet}"
test -f .env || { echo "Missing server .env" >&2; exit 1; }
docker compose version >/dev/null
compose=(docker compose -p docker -f infra/docker/docker-compose.data.yml --env-file .env)
"${compose[@]}" config --quiet
"${compose[@]}" pull
"${compose[@]}" up -d --wait --wait-timeout 180 postgres redis nats minio
"${compose[@]}" up -d minio-init
init_id=$("${compose[@]}" ps -a -q minio-init)
test -n "$init_id"
exit_code=$(docker wait "$init_id")
[[ "$exit_code" == 0 ]] || { echo "MinIO initialization failed" >&2; exit 1; }
"${compose[@]}" ps
