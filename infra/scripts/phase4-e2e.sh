#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
PASSWORD="phase4-password-$(date +%s)"
A_EMAIL="phase4-a-$(date +%s)@example.test"
B_EMAIL="phase4-b-$(date +%s)@example.test"

request() {
  curl -sS -w '\n%{http_code}' "$@"
}

json_field() {
  local field="$1"
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field"
}

expect_status() {
  local expected="$1" actual="$2" label="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL [$label]: expected HTTP $expected, got $actual" >&2
    exit 1
  fi
  echo "PASS [$label]: HTTP $actual"
}

wait_for_health() {
  for _ in $(seq 1 60); do
    if curl -fsS "$BASE_URL/healthz" >/dev/null 2>&1; then return 0; fi
    sleep 2
  done
  echo "FAIL: gateway did not become healthy" >&2
  exit 1
}

wait_for_health
curl -fsS "$BASE_URL/healthz/services"

a_response="$(request -X POST "$BASE_URL/v1/auth/register" -H 'Content-Type: application/json' -d "{\"name\":\"Phase4 A\",\"email\":\"$A_EMAIL\",\"password\":\"$PASSWORD\"}")"
a_status="${a_response##*$'\n'}"; a_body="${a_response%$'\n'*}"
expect_status 201 "$a_status" "register A"
a_token="$(printf '%s' "$a_body" | json_field token)"

b_response="$(request -X POST "$BASE_URL/v1/auth/register" -H 'Content-Type: application/json' -d "{\"name\":\"Phase4 B\",\"email\":\"$B_EMAIL\",\"password\":\"$PASSWORD\"}")"
b_status="${b_response##*$'\n'}"; b_body="${b_response%$'\n'*}"
expect_status 201 "$b_status" "register B"
b_token="$(printf '%s' "$b_body" | json_field token)"

set_response="$(request -X POST "$BASE_URL/v1/study-sets" -H "Authorization: Bearer $a_token" -H 'Content-Type: application/json' -H 'X-Request-ID: phase4-e2e-create' -d '{"title":"Phase 4 E2E","description":"Dev5 gate"}')"
set_status="${set_response##*$'\n'}"; set_body="${set_response%$'\n'*}"
expect_status 201 "$set_status" "create study set"
set_id="$(printf '%s' "$set_body" | json_field id)"

card_response="$(request -X POST "$BASE_URL/v1/study-sets/$set_id/flashcards" -H "Authorization: Bearer $a_token" -H 'Content-Type: application/json' -d '{"term":"hello","definition":"xin chào"}')"
card_status="${card_response##*$'\n'}"; card_body="${card_response%$'\n'*}"
expect_status 201 "$card_status" "create card 1"
card1="$(printf '%s' "$card_body" | json_field id)"

card2_response="$(request -X POST "$BASE_URL/v1/study-sets/$set_id/flashcards" -H "Authorization: Bearer $a_token" -H 'Content-Type: application/json' -d '{"term":"book","definition":"quyển sách"}')"
card2_status="${card2_response##*$'\n'}"
expect_status 201 "$card2_status" "create card 2"

spoof_status="$(curl -sS -o /tmp/phase4-spoof.json -w '%{http_code}' "$BASE_URL/v1/study-sets/$set_id" -H "Authorization: Bearer $b_token" -H 'X-User-ID: 1')"
if [[ "$spoof_status" != "403" && "$spoof_status" != "404" ]]; then
  echo "FAIL [ownership/spoofing]: User B received HTTP $spoof_status" >&2
  exit 1
fi
echo "PASS [ownership/spoofing]: User B blocked with HTTP $spoof_status"

generate_response="$(request -X POST "$BASE_URL/v1/study-sets/$set_id/quiz/generate" -H "Authorization: Bearer $a_token" -H 'Content-Type: application/json' -H 'X-Request-ID: phase4-generate' -d '{"mode":"test","seed":42,"limit":2,"options":{}}')"
generate_status="${generate_response##*$'\n'}"; generate_body="${generate_response%$'\n'*}"
expect_status 200 "$generate_status" "quiz generate"
seed="$(printf '%s' "$generate_body" | json_field seed)"

# Extract the first generated question identity and choice. The server remains
# the sole source of truth; this script intentionally never sends a correct answer.
read -r first_card first_choice < <(printf '%s' "$generate_body" | python3 -c 'import json,sys; d=json.load(sys.stdin); q=next(x for x in d["items"] if x.get("kind")=="question" or x.get("choices")); print(q["flashcardId"], q["choices"][0])')

evaluate_body="$(python3 - "$first_card" "$first_choice" "$seed" <<'PY'
import json,sys
print(json.dumps({"mode":"test","seed":int(sys.argv[3]),"answers":[{"flashcardId":int(sys.argv[1]),"answer":sys.argv[2],"attempts":1,"responseTimeMs":25}]}))
PY
)"
evaluate_response="$(request -X POST "$BASE_URL/v1/study-sets/$set_id/quiz/evaluate" -H "Authorization: Bearer $a_token" -H 'Content-Type: application/json' -H 'X-Request-ID: phase4-evaluate' -d "$evaluate_body")"
evaluate_status="${evaluate_response##*$'\n'}"
expect_status 200 "$evaluate_status" "quiz evaluate"

echo "PASS [P4-E2E]: Test generate → evaluate"
echo "NOTE: full four-mode/progress/history assertions are enabled after the backend contract is present; this gate fails closed if any required endpoint is unavailable."
