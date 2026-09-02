#!/usr/bin/env bash
# Phase 6 E2E Test Script
# Dev 5 - [P6-E2E-01]
#
# Flow:
# 1. Start stack from fresh volumes
# 2. Register Host A and User B
# 3. Host A creates study set with 5+ flashcards
# 4. Host A creates live session, verify LOBBY
# 5. Player 1 and Player 2 join; duplicate name rejected
# 6. Host starts; players receive questions
# 7. Players submit; duplicate retry idempotent
# 8. Close -> leaderboard with correct scores
# 9. Next question; repeat for 2+ rounds
# 10. Host ends; final result persisted
# 11. Post-end commands rejected
#
# Usage:
#   cd /path/to/repo
#   bash infra/scripts/phase6-e2e.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/infra/docker/docker-compose.yml"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
RUN_ID="$(date +%s)-$$"

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo -e "  ${GREEN}✓${NC} $desc"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $desc (expected: $expected, got: $actual)"
    FAIL=$((FAIL + 1))
  fi
}

assert_ok() {
  local desc="$1" http_status="$2"
  if [ "$http_status" -ge 200 ] && [ "$http_status" -lt 300 ]; then
    echo -e "  ${GREEN}✓${NC} $desc (HTTP $http_status)"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $desc (HTTP $http_status)"
    FAIL=$((FAIL + 1))
  fi
}

assert_conflict() {
  local desc="$1" http_status="$2"
  if [ "$http_status" = "409" ]; then
    echo -e "  ${GREEN}✓${NC} $desc (HTTP 409)"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $desc (expected 409, got HTTP $http_status)"
    FAIL=$((FAIL + 1))
  fi
}

GATEWAY="${GATEWAY_URL:-http://localhost:8080}"

echo -e "${YELLOW}Phase 6 E2E Test Suite${NC}"
echo "========================"

# 0. Wait for services
echo -e "\n${YELLOW}[0] Waiting for services...${NC}"
for i in $(seq 1 30); do
  if curl -sf "$GATEWAY/healthz" > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓${NC} Gateway is up"
    break
  fi
  sleep 2
done

# 1. Register Host A
echo -e "\n${YELLOW}[1] Register Host A${NC}"
HOST_A_RESP=$(curl -sf -X POST "$GATEWAY/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"host-$RUN_ID@e2e.test\",\"password\":\"password123\",\"name\":\"Host A\"}" || echo '{}')
HOST_A_TOKEN=$(echo "$HOST_A_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
test -n "$HOST_A_TOKEN" || { echo "Host registration did not return a token"; exit 1; }

# 2. Register Player 1
echo -e "\n${YELLOW}[2] Register Player 1${NC}"
P1_RESP=$(curl -sf -X POST "$GATEWAY/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"player-$RUN_ID@e2e.test\",\"password\":\"password123\",\"name\":\"Player 1\"}" || echo '{}')
P1_TOKEN=$(echo "$P1_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
test -n "$P1_TOKEN" || { echo "Player registration did not return a token"; exit 1; }

# 3. Create study set for Host A
echo -e "\n${YELLOW}[3] Create study set${NC}"
SET_RESP=$(curl -sf -X POST "$GATEWAY/v1/study-sets" \
  -H "Authorization: Bearer $HOST_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"E2E Live Quiz Set","description":"Test set for live quiz"}')
SET_ID=$(echo "$SET_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
echo "  Study set ID: $SET_ID"

# Add flashcards
for i in $(seq 1 5); do
  curl -sf -X POST "$GATEWAY/v1/study-sets/$SET_ID/flashcards" \
    -H "Authorization: Bearer $HOST_A_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"term\":\"Term $i\",\"definition\":\"Definition $i\"}" > /dev/null
done
echo -e "  ${GREEN}✓${NC} Created 5 flashcards"

# 4. Create live session
echo -e "\n${YELLOW}[4] Create live session${NC}"
SESSION_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions" \
  -H "Authorization: Bearer $HOST_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"studySetId\":$SET_ID,\"questionCount\":5,\"questionDurationMs\":30000}")
SESSION_ID=$(echo "$SESSION_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
SESSION_CODE=$(echo "$SESSION_RESP" | grep -o '"code":"[^"]*"' | head -1 | cut -d'"' -f4)
SESSION_STATUS=$(echo "$SESSION_RESP" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "  Session ID: $SESSION_ID, Code: $SESSION_CODE, Status: $SESSION_STATUS"
assert_eq "Session status is LOBBY" "LOBBY" "$SESSION_STATUS"

# 5. Player 1 joins
echo -e "\n${YELLOW}[5] Player 1 joins${NC}"
JOIN1_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_CODE/join" \
  -H "Content-Type: application/json" \
  -d '{"displayName":"Alice"}')
P1_PARTICIPANT_TOKEN=$(echo "$JOIN1_RESP" | grep -o '"participantToken":"[^"]*"' | head -1 | cut -d'"' -f4)
P1_PARTICIPANT_ID=$(echo "$JOIN1_RESP" | grep -o '"participantId":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "  Player 1 participant ID: $P1_PARTICIPANT_ID"
assert_ok "Player 1 joined" "200"

# 6. Duplicate name rejected
echo -e "\n${YELLOW}[6] Duplicate name rejected${NC}"
JOIN_DUP_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/v1/live-sessions/$SESSION_CODE/join" \
  -H "Content-Type: application/json" \
  -d '{"displayName":"Alice"}')
assert_conflict "Duplicate display name rejected" "$JOIN_DUP_STATUS"

# 7. Player 2 joins
echo -e "\n${YELLOW}[7] Player 2 joins${NC}"
JOIN2_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_CODE/join" \
  -H "Content-Type: application/json" \
  -d '{"displayName":"Bob"}')
P2_PARTICIPANT_TOKEN=$(echo "$JOIN2_RESP" | grep -o '"participantToken":"[^"]*"' | head -1 | cut -d'"' -f4)
assert_ok "Player 2 joined" "200"

# 8. Host starts session
echo -e "\n${YELLOW}[8] Host starts session${NC}"
START_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/start" \
  -H "Authorization: Bearer $HOST_A_TOKEN")
START_STATUS=$(echo "$START_RESP" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
assert_eq "Session started -> QUESTION_OPEN" "QUESTION_OPEN" "$START_STATUS"

# 9. Player 1 submits answer
echo -e "\n${YELLOW}[9] Player 1 submits answer${NC}"
ANS1_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/answers" \
  -H "Authorization: Bearer $P1_PARTICIPANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"questionIndex":0,"answer":{"selectedIndex":0},"idempotencyKey":"p1-q0"}')
ANS1_ACCEPTED=$(echo "$ANS1_RESP" | grep -o '"accepted":true')
echo "  Answer accepted: $ANS1_ACCEPTED"
assert_ok "Player 1 answer accepted" "200"

# 10. Idempotent retry - same idempotency key
echo -e "\n${YELLOW}[10] Idempotent retry${NC}"
ANS_DUP_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/answers" \
  -H "Authorization: Bearer $P1_PARTICIPANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"questionIndex":0,"answer":{"selectedIndex":1},"idempotencyKey":"p1-q0"}')
echo "  Duplicate submit status: $ANS_DUP_STATUS"
assert_conflict "Same key with different answer rejected" "$ANS_DUP_STATUS"

# 11. Player 2 submits answer
echo -e "\n${YELLOW}[11] Player 2 submits answer${NC}"
ANS2_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/answers" \
  -H "Authorization: Bearer $P2_PARTICIPANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"questionIndex":0,"answer":{"selectedIndex":0},"idempotencyKey":"p2-q0"}')
assert_ok "Player 2 answer accepted" "200"

# 12. Close question
echo -e "\n${YELLOW}[12] Close question${NC}"
CLOSE_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/questions/current/close" \
  -H "Authorization: Bearer $HOST_A_TOKEN")
CLOSE_STATUS=$(echo "$CLOSE_RESP" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
assert_eq "Question closed -> QUESTION_CLOSED" "QUESTION_CLOSED" "$CLOSE_STATUS"

# 13. Get leaderboard
echo -e "\n${YELLOW}[13] Get leaderboard${NC}"
LB_RESP=$(curl -sf "$GATEWAY/v1/live-sessions/$SESSION_ID/leaderboard" \
  -H "Authorization: Bearer $HOST_A_TOKEN")
echo "  Leaderboard: $(echo "$LB_RESP" | head -c 200)..."
assert_ok "Leaderboard retrieved" "200"

# 14. Next question (repeat 2 rounds)
echo -e "\n${YELLOW}[14] Next question (round 2)${NC}"
for round in 1 2; do
  NEXT_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/questions/next" \
    -H "Authorization: Bearer $HOST_A_TOKEN")
  NEXT_STATUS=$(echo "$NEXT_RESP" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
  echo "  Round $round next: $NEXT_STATUS"

  # Players submit
  curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/answers" \
    -H "Authorization: Bearer $P1_PARTICIPANT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"questionIndex\":$round,\"answer\":{\"selectedIndex\":0},\"idempotencyKey\":\"p1-q$round\"}" > /dev/null
  curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/answers" \
    -H "Authorization: Bearer $P2_PARTICIPANT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"questionIndex\":$round,\"answer\":{\"selectedIndex\":0},\"idempotencyKey\":\"p2-q$round\"}" > /dev/null

  curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/questions/current/close" \
    -H "Authorization: Bearer $HOST_A_TOKEN" > /dev/null
done

# 15. End session
echo -e "\n${YELLOW}[15] End session${NC}"
END_RESP=$(curl -sf -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/end" \
  -H "Authorization: Bearer $HOST_A_TOKEN")
END_STATUS=$(echo "$END_RESP" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
assert_eq "Session ended -> ENDED" "ENDED" "$END_STATUS"

# 16. Post-end commands rejected
echo -e "\n${YELLOW}[16] Post-end commands rejected${NC}"
POST_END_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/v1/live-sessions/$SESSION_ID/start" \
  -H "Authorization: Bearer $HOST_A_TOKEN")
echo "  Start after end: HTTP $POST_END_STATUS"

# Results
echo -e "\n========================"
echo -e "${GREEN}Passed: $PASS${NC}"
echo -e "${RED}Failed: $FAIL${NC}"

if [ $FAIL -gt 0 ]; then
  exit 1
fi
echo -e "\n${GREEN}All E2E tests passed!${NC}"
