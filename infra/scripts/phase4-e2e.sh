#!/usr/bin/env bash
# P4-E2E-01 — Full Phase 4 E2E script.
# Covers: flashcards, learn, test, match, progress save/history, ownership, security.
# Usage: GATEWAY_URL=http://localhost:8080 bash phase4-e2e.sh
set -euo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
PASSWORD="phase4-e2e-$(date +%s)"
A_EMAIL="phase4-a-$(date +%s)@test.local"
B_EMAIL="phase4-b-$(date +%s)@test.local"
PASS_COUNT=0
FAIL_COUNT=0
TOTAL_COUNT=0

# ── Helpers ────────────────────────────────────────────────────────────────────

json_field() {
  local field="$1"
  python3 -c "import json,sys; print(json.load(sys.stdin)['$field'])"
}

json_array_len() {
  python3 -c "import json,sys; print(len(json.load(sys.stdin)))"
}

json_array_field() {
  local field="$1" idx="$2"
  python3 -c "import json,sys; print(json.load(sys.stdin)[$idx]['$field'])"
}

expect_status() {
  local expected="$1" actual="$2" label="$3"
  TOTAL_COUNT=$((TOTAL_COUNT + 1))
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL [$label]: expected HTTP $expected, got $actual" >&2
    FAIL_COUNT=$((FAIL_COUNT + 1))
  else
    echo "PASS [$label]: HTTP $actual"
    PASS_COUNT=$((PASS_COUNT + 1))
  fi
}

expect_json() {
  local label="$1" condition="$2"
  TOTAL_COUNT=$((TOTAL_COUNT + 1))
  if ! eval "$condition" >/dev/null 2>&1; then
    echo "FAIL [$label]: condition failed" >&2
    FAIL_COUNT=$((FAIL_COUNT + 1))
  else
    echo "PASS [$label]"
    PASS_COUNT=$((PASS_COUNT + 1))
  fi
}

post_json() {
  local url="$1" token="$2" body="$3"
  curl -sS -w '\n%{http_code}' "$url" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $token" \
    -d "$body"
}

get_json() {
  local url="$1" token="$2"
  curl -sS -w '\n%{http_code}' "$url" \
    -H "Authorization: Bearer $token"
}

extract_status() { echo "${1##*$'\n'}"; }
extract_body() { echo "${1%$'\n'*}"; }

wait_for_health() {
  echo "Waiting for gateway at $BASE_URL ..."
  for i in $(seq 1 60); do
    if curl -fsS "$BASE_URL/healthz" >/dev/null 2>&1; then
      echo "Gateway healthy."
      return 0
    fi
    sleep 2
  done
  echo "FAIL: gateway did not become healthy after 120s" >&2
  exit 1
}

# ── Setup ──────────────────────────────────────────────────────────────────────

wait_for_health

# Register User A
a_resp=$(post_json "$BASE_URL/v1/auth/register" "" "{\"name\":\"Phase4A\",\"email\":\"$A_EMAIL\",\"password\":\"$PASSWORD\"}")
a_status=$(extract_status "$a_resp"); a_body=$(extract_body "$a_resp")
expect_status 201 "$a_status" "register User A"
a_token=$(echo "$a_body" | json_field token)

# Register User B
b_resp=$(post_json "$BASE_URL/v1/auth/register" "" "{\"name\":\"Phase4B\",\"email\":\"$B_EMAIL\",\"password\":\"$PASSWORD\"}")
b_status=$(extract_status "$b_resp"); b_body=$(extract_body "$b_resp")
expect_status 201 "$b_status" "register User B"
b_token=$(echo "$b_body" | json_field token)

# Create study set for User A
set_resp=$(post_json "$BASE_URL/v1/study-sets" "$a_token" '{"title":"Phase 4 E2E","description":"Full E2E test"}')
set_status=$(extract_status "$set_resp"); set_body=$(extract_body "$set_resp")
expect_status 201 "$set_status" "create study set"
set_id=$(echo "$set_body" | json_field id)

# Create 5 flashcards
cards=("hello:xin chào" "book:quyển sách" "water:nước" "computer:máy tính" "teacher:giáo viên")
for card in "${cards[@]}"; do
  term="${card%%:*}"
  def="${card##*:}"
  cr=$(post_json "$BASE_URL/v1/study-sets/$set_id/flashcards" "$a_token" "{\"term\":\"$term\",\"definition\":\"$def\"}")
  cs=$(extract_status "$cr")
  expect_status 201 "$cs" "create card: $term"
done

# ── Security checks ───────────────────────────────────────────────────────────

# User B cannot access User A's study set
b_set_resp=$(get_json "$BASE_URL/v1/study-sets/$set_id" "$b_token")
b_set_status=$(extract_status "$b_set_resp")
expect_status 403 "$b_set_status" "ownership: User B blocked from User A set"

# Spoofed X-User-ID header (gateway should strip it)
spoof_resp=$(curl -sS -w '\n%{http_code}' "$BASE_URL/v1/study-sets/$set_id" \
  -H "Authorization: Bearer $b_token" -H "X-User-ID: 1")
spoof_status=$(extract_status "$spoof_resp")
expect_status 403 "$spoof_status" "spoofed X-User-ID rejected"

# ── Flashcards mode ───────────────────────────────────────────────────────────

echo ""
echo "=== Flashcards mode ==="
fc_gen=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"flashcards","seed":42,"limit":5}')
fc_status=$(extract_status "$fc_gen"); fc_body=$(extract_body "$fc_gen")
expect_status 200 "$fc_status" "flashcards generate"
fc_items=$(echo "$fc_body" | json_array_len)
expect_json "flashcards item count" "[ $fc_items -eq 5 ]"

# Evaluate (flashcards always scores all correct)
fc_eval=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/evaluate" "$a_token" \
  "{\"mode\":\"flashcards\",\"seed\":42,\"limit\":5,\"answers\":[{\"flashcardId\":1,\"attempts\":1},{\"flashcardId\":2,\"attempts\":1},{\"flashcardId\":3,\"attempts\":1},{\"flashcardId\":4,\"attempts\":1},{\"flashcardId\":5,\"attempts\":1}]}")
fc_eval_status=$(extract_status "$fc_eval"); fc_eval_body=$(extract_body "$fc_eval")
expect_status 200 "$fc_eval_status" "flashcards evaluate"
fc_score=$(echo "$fc_eval_body" | json_field score)
expect_json "flashcards score=5" "[ $fc_score -eq 5 ]"

# ── Learn mode ────────────────────────────────────────────────────────────────

echo ""
echo "=== Learn mode ==="
learn_gen=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"learn","seed":7,"limit":5}')
learn_status=$(extract_status "$learn_gen"); learn_body=$(extract_body "$learn_gen")
expect_status 200 "$learn_status" "learn generate"

# Get first flashcardId from items
learn_fc_id=$(echo "$learn_body" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['items'][0]['flashcardId'])")
learn_fc_def=$(echo "$learn_body" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['items'][0]['definition'])")

# Evaluate with correct answer
learn_eval_correct=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/evaluate" "$a_token" \
  "{\"mode\":\"learn\",\"seed\":7,\"limit\":5,\"answers\":[{\"flashcardId\":$learn_fc_id,\"submitted\":\"$learn_fc_def\",\"attempts\":1,\"responseTimeMs\":2000}]}")
le_status=$(extract_status "$learn_eval_correct")
expect_status 200 "$le_status" "learn evaluate (correct)"

# Evaluate with wrong answer (normalization test)
learn_eval_wrong=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/evaluate" "$a_token" \
  "{\"mode\":\"learn\",\"seed\":7,\"limit\":5,\"answers\":[{\"flashcardId\":$learn_fc_id,\"submitted\":\"wrong answer\",\"attempts\":2,\"responseTimeMs\":3000}]}")
lew_status=$(extract_status "$learn_eval_wrong"); lew_body=$(extract_body "$learn_eval_wrong")
expect_status 200 "$lew_status" "learn evaluate (wrong)"
lew_score=$(echo "$lew_body" | json_field score)
expect_json "learn wrong=0" "[ $lew_score -eq 0 ]"

# ── Test mode ─────────────────────────────────────────────────────────────────

echo ""
echo "=== Test mode ==="
test_gen=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"test","seed":42,"limit":5}')
test_status=$(extract_status "$test_gen"); test_body=$(extract_body "$test_gen")
expect_status 200 "$test_status" "test generate"

# Verify test items have choices and NO correctIndex
test_has_correct_idx=$(echo "$test_body" | python3 -c "
import json,sys
d=json.load(sys.stdin)
items=d.get('items',[])
has_ci = any('correctIndex' in item for item in items)
print('yes' if has_ci else 'no')
")
expect_json "test no leaked correctIndex" "[ $test_has_correct_idx = 'no' ]"

test_has_choices=$(echo "$test_body" | python3 -c "
import json,sys
d=json.load(sys.stdin)
items=d.get('items',[])
all_have = all(len(item.get('choices',[])) >= 2 for item in items)
print('yes' if all_have else 'no')
")
expect_json "test all items have choices" "[ $test_has_choices = 'yes' ]"

# Evaluate test — pick first choice for all
test_answers=$(echo "$test_body" | python3 -c "
import json,sys
d=json.load(sys.stdin)
answers=[{'flashcardId':item['flashcardId'],'selectedIndex':0,'attempts':1,'responseTimeMs':2000} for item in d['items']]
print(json.dumps(answers))
")
test_eval=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/evaluate" "$a_token" \
  "{\"mode\":\"test\",\"seed\":42,\"limit\":5,\"answers\":$test_answers}")
te_status=$(extract_status "$test_eval")
expect_status 200 "$te_status" "test evaluate"

# ── Match mode ────────────────────────────────────────────────────────────────

echo ""
echo "=== Match mode ==="
match_gen=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"match","seed":42,"limit":4}')
match_status=$(extract_status "$match_gen"); match_body=$(extract_body "$match_gen")
expect_status 200 "$match_status" "match generate"

# Verify match items are flat array (not { subset, pairs })
match_is_array=$(echo "$match_body" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('yes' if isinstance(d.get('items'), list) else 'no')
")
expect_json "match items is flat array" "[ $match_is_array = 'yes' ]"

match_item_count=$(echo "$match_body" | json_array_len)
expect_json "match has 8 items (4 pairs)" "[ $match_item_count -eq 8 ]"

# Evaluate match with correct pair identity
match_answers=$(echo "$match_body" | python3 -c "
import json,sys
d=json.load(sys.stdin)
answers=[]
for item in d['items']:
    if item.get('kind')=='term':
        answers.append({'flashcardId':item['flashcardId'],'pairId':item['pairId'],'matchedFlashcardId':item['flashcardId'],'attempts':1,'responseTimeMs':2000})
print(json.dumps(answers))
")
match_eval=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/evaluate" "$a_token" \
  "{\"mode\":\"match\",\"seed\":42,\"limit\":4,\"answers\":$match_answers}")
me_status=$(extract_status "$match_eval"); me_body=$(extract_body "$match_eval")
expect_status 200 "$me_status" "match evaluate"
me_score=$(echo "$me_body" | json_field score)
expect_json "match score=4" "[ $me_score -eq 4 ]"

# ── Progress save ─────────────────────────────────────────────────────────────

echo ""
echo "=== Progress save ==="
progress_resp=$(post_json "$BASE_URL/v1/study-sets/$set_id/progress" "$a_token" \
  "{\"mode\":\"learn\",\"score\":3,\"total\":5,\"startedAt\":\"2026-09-01T00:00:00Z\",\"completedAt\":\"2026-09-01T00:01:00Z\",\"idempotencyKey\":\"$set_id:learn:2026-09-01T00:00:00Z\",\"cardResults\":[{\"flashcardId\":1,\"correct\":true,\"attempts\":1},{\"flashcardId\":2,\"correct\":false,\"attempts\":2},{\"flashcardId\":3,\"correct\":true,\"attempts\":1}]}")
progress_status=$(extract_status "$progress_resp")
expect_status 201 "$progress_status" "save progress"

# Verify history shows the new session
history_resp=$(get_json "$BASE_URL/v1/study-sets/$set_id/progress?page=1&per_page=10" "$a_token")
history_status=$(extract_status "$history_resp"); history_body=$(extract_body "$history_resp")
expect_status 200 "$history_status" "get progress history"
history_total=$(echo "$history_body" | python3 -c "import json,sys; print(json.load(sys.stdin)['totalSessions'])")
expect_json "progress history has 1 session" "[ $history_total -ge 1 ]"

# Latest progress
latest_resp=$(get_json "$BASE_URL/v1/study-sets/$set_id/progress/latest" "$a_token")
latest_status=$(extract_status "$latest_resp")
expect_status 200 "$latest_status" "get latest progress"

# ── Idempotency ───────────────────────────────────────────────────────────────

echo ""
echo "=== Idempotency ==="
idem_resp=$(post_json "$BASE_URL/v1/study-sets/$set_id/progress" "$a_token" \
  "{\"mode\":\"learn\",\"score\":3,\"total\":5,\"startedAt\":\"2026-09-01T00:00:00Z\",\"completedAt\":\"2026-09-01T00:01:00Z\",\"idempotencyKey\":\"$set_id:learn:2026-09-01T00:00:00Z\",\"cardResults\":[{\"flashcardId\":1,\"correct\":true,\"attempts\":1}]}")
idem_status=$(extract_status "$idem_resp")
# 409 = idempotency conflict (already saved) is acceptable
TOTAL_COUNT=$((TOTAL_COUNT + 1))
if [[ "$idem_status" == "409" || "$idem_status" == "201" ]]; then
  echo "PASS [idempotency: duplicate save handled]: HTTP $idem_status"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo "FAIL [idempotency: duplicate save]: expected 201 or 409, got HTTP $idem_status" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# ── Error cases ───────────────────────────────────────────────────────────────

echo ""
echo "=== Error matrix ==="
# Missing token
no_auth=$(curl -sS -w '\n%{http_code}' "$BASE_URL/v1/study-sets/$set_id/quiz/generate" \
  -H "Content-Type: application/json" -d '{"mode":"flashcards","seed":1,"limit":10}')
no_auth_status=$(extract_status "$no_auth")
expect_status 401 "$no_auth_status" "error: missing token"

# Invalid mode
bad_mode=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"bogus","seed":1,"limit":10}')
bad_mode_status=$(extract_status "$bad_mode")
expect_status 422 "$bad_mode_status" "error: invalid mode"

# Quiz service deterministic after restart (simulate by using same seed twice)
gen1=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"test","seed":7777,"limit":3}')
gen1_body=$(extract_body "$gen1")
gen2=$(post_json "$BASE_URL/v1/study-sets/$set_id/quiz/generate" "$a_token" \
  '{"mode":"test","seed":7777,"limit":3}')
gen2_body=$(extract_body "$gen2")
same_output=$(python3 -c "import json,sys; a=json.loads(sys.argv[1]); b=json.loads(sys.argv[2]); print('true' if a['items']==b['items'] else 'false')" "$gen1_body" "$gen2_body")
expect_json "deterministic: same seed same output" "[ $same_output = 'true' ]"

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  P4-E2E RESULTS: $PASS_COUNT/$TOTAL_COUNT PASS, $FAIL_COUNT FAIL"
echo "═══════════════════════════════════════════════════════════"

if [[ $FAIL_COUNT -gt 0 ]]; then
  echo "RESULT: FAIL"
  exit 1
else
  echo "RESULT: PASS"
  exit 0
fi
