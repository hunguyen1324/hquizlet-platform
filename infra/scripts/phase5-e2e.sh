#!/usr/bin/env bash
# Phase 5 Folder API E2E. Run against a fresh Docker stack.
set -euo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
STAMP="$(date +%s)"
PASSWORD="phase5-$STAMP-password"

request() {
  local method="$1" path="$2" token="${3:-}" body="${4:-}"
  local args=(-sS -w $'\n%{http_code}' -X "$method" "$BASE_URL$path")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$body" ]] && args+=(-H "Content-Type: application/json" -d "$body")
  curl "${args[@]}"
}
body() { printf '%s' "${1%$'\n'*}"; }
status() { printf '%s' "${1##*$'\n'}"; }
field() { python3 -c "import json,sys; print(json.load(sys.stdin)['$1'])"; }
expect() { [[ "$(status "$1")" == "$2" ]] || { echo "FAIL: $3 ($(status "$1"), expected $2)" >&2; exit 1; }; echo "PASS: $3"; }

for _ in $(seq 1 60); do curl -fsS "$BASE_URL/healthz" >/dev/null 2>&1 && break; sleep 2; done

a=$(request POST /v1/auth/register "" "{\"name\":\"Phase5 A\",\"email\":\"phase5-a-$STAMP@test.local\",\"password\":\"$PASSWORD\"}"); expect "$a" 201 "register A"; at=$(body "$a" | field token)
b=$(request POST /v1/auth/register "" "{\"name\":\"Phase5 B\",\"email\":\"phase5-b-$STAMP@test.local\",\"password\":\"$PASSWORD\"}"); expect "$b" 201 "register B"; bt=$(body "$b" | field token)

s1=$(request POST /v1/study-sets "$at" '{"title":"Set one","description":"kept after folder delete"}'); expect "$s1" 201 "create set 1"; s1id=$(body "$s1" | field id)
s2=$(request POST /v1/study-sets "$at" '{"title":"Set two","description":"folder member"}'); expect "$s2" 201 "create set 2"; s2id=$(body "$s2" | field id)
f=$(request POST /v1/folders "$at" '{"title":"English","description":"IELTS"}'); expect "$f" 201 "create folder"; fid=$(body "$f" | field id)

expect "$(request POST "/v1/folders/$fid/study-sets" "$at" "{\"studySetId\":$s1id}")" 200 "add set 1"
expect "$(request POST "/v1/folders/$fid/study-sets" "$at" "{\"studySetId\":$s2id}")" 200 "add set 2"
detail=$(request GET "/v1/folders/$fid" "$at"); expect "$detail" 200 "folder detail"
[[ "$(body "$detail" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["studySets"]))')" == 2 ]] || exit 1
expect "$(request DELETE "/v1/folders/$fid/study-sets/$s1id" "$at")" 200 "remove relation"
expect "$(request GET "/v1/folders/$fid" "$bt")" 403 "B cannot read A folder"
expect "$(request PUT "/v1/folders/$fid" "$bt" '{"title":"stolen","description":""}')" 403 "B cannot update A folder"
bf=$(request POST /v1/folders "$bt" '{"title":"B folder","description":"ownership check"}'); expect "$bf" 201 "create B folder"; bfid=$(body "$bf" | field id)
expect "$(request POST "/v1/folders/$bfid/study-sets" "$bt" "{\"studySetId\":$s1id}")" 403 "B cannot add A set"
expect "$(request DELETE "/v1/folders/$fid" "$at")" 200 "delete folder"
expect "$(request GET "/v1/study-sets/$s1id" "$at")" 200 "source set survives folder delete"
expect "$(request GET /v1/folders "$at")" 200 "folder list remains available"
echo "Phase 5 Folder E2E PASS"
