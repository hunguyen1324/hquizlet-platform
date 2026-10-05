#!/usr/bin/env bash
# Usage: bash setup-nat-mini.sh [IMAGE_TAG]. Uses the existing server .env.
set -euo pipefail
cd "${INSTALL_DIR:-/opt/hquizlet}"
test -f .env || { echo "Missing server .env" >&2; exit 1; }
docker compose version >/dev/null
compose=(docker compose -p docker -f infra/docker/docker-compose.app.yml)
for override in infra/docker/docker-compose.data-hosts.yml infra/docker/docker-compose.quicktunnel.yml; do
  if [[ -f "$override" ]]; then compose+=(-f "$override"); fi
done
compose+=(--env-file .env)
if [[ -f .image_tag.env ]]; then compose+=(--env-file .image_tag.env); fi
if [[ -n "${1:-}" ]]; then
  [[ "$1" =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$ ]] || { echo "Invalid image tag" >&2; exit 1; }
  export IMAGE_TAG="$1"
fi
"${compose[@]}" config --quiet
"${compose[@]}" pull nginx gateway auth study quiz class payment file
"${compose[@]}" up -d --wait --wait-timeout 180 nginx gateway auth study quiz class payment file
"${compose[@]}" exec -T nginx nginx -t
"${compose[@]}" exec -T nginx nginx -s reload
# Keep the existing Quick Tunnel and its URL.
if [[ -f infra/docker/docker-compose.quicktunnel.yml ]]; then
  "${compose[@]}" up -d --no-recreate cloudflared-quick
fi
if [[ -n "${1:-}" ]]; then printf 'IMAGE_TAG=%s\n' "$1" > .image_tag.env; fi
"${compose[@]}" ps
"${compose[@]}" exec -T nginx wget -T 10 -qO- http://127.0.0.1/api/healthz/services
echo
