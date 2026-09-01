#!/usr/bin/env bash
set -euo pipefail

COMPOSE=(docker compose -f infra/docker/docker-compose.yml)
BASE_URL="${GATEWAY_URL:-http://localhost:8080}"

cleanup() {
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

json_value() {
  python3 - "$1" <<'PY'
import json, sys
print(json.load(sys.stdin)[sys.argv[1]])
PY
}

wait_for() {
  local url="$1"
  local attempts="${2:-60}"
  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    sleep 2
  done
  echo "Timed out waiting for $url" >&2
  return 1
}

echo "[Dev5] Fresh Phase 3 Docker gate"
cleanup
"${COMPOSE[@]}" up -d --build
wait_for "$BASE_URL/healthz"
wait_for "$BASE_URL/healthz/services"

health="$(curl -fsS "$BASE_URL/healthz/services")"
echo "$health"

register() {
  local name="$1"
  local email="$2"
  local password="$3"
  curl -fsS -X POST "$BASE_URL/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"name\":\"$name\",\"email\":\"$email\",\"password\":\"$password\"}"
}

A_JSON="$(register 'Phase3 User A' "phase3-a-$(date +%s)@example.test" 'phase3-password')"
B_JSON="$(register 'Phase3 User B' "phase3-b-$(date +%s)@example.test" 'phase3-password')"
A_TOKEN="$(printf '%s' "$A_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"
B_TOKEN="$(printf '%s' "$B_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"

SET_JSON="$(curl -fsS -X POST "$BASE_URL/v1/study-sets" -H "Authorization: Bearer $A_TOKEN" -H 'Content-Type: application/json' -d '{"title":"Phase 3 ownership gate","description":"temporary gate data"}')"
SET_ID="$(printf '%s' "$SET_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"

echo "Created study set $SET_ID as User A"

status="$(curl -sS -o /tmp/phase3-owner-response.json -w '%{http_code}' "$BASE_URL/v1/study-sets/$SET_ID" -H "Authorization: Bearer $B_TOKEN")"
if [[ "$status" != "403" && "$status" != "404" ]]; then
  echo "FAIL: User B accessed User A study set: HTTP $status" >&2
  cat /tmp/phase3-owner-response.json >&2
  exit 1
fi

echo "Ownership A/B gate: PASS (User B received $status)"

echo "Progress endpoint smoke check requires Dev2 progress implementation and is intentionally executed after backend progress merge."

echo "[Dev5] Fresh-volume Phase 3 gate: PASS"
