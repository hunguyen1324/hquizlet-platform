#!/usr/bin/env bash
# Phase 7 E2E Test Script — Class & Activity End-to-end flow
# Covers: create class, join by invite code, manage members, study sets, activity feed
set -euo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
GATEWAY="$BASE_URL/v1"
PASS=0
FAIL=0

# --- Helpers ---
log()  { echo -e "\033[0;36m[Phase7-E2E]\033[0m $*"; }
ok()   { echo -e "\033[0;32m  ✓ $*\033[0m"; PASS=$((PASS + 1)); }
fail() { echo -e "\033[0;31m  ✗ $*\033[0m"; FAIL=$((FAIL + 1)); }

json() { echo "$1" | head -1; }

auth_header() { echo "Authorization: Bearer $1"; }

assert_status() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then ok "$desc"; else fail "$desc (expected $expected, got $actual)"; fi
}

assert_contains() {
  local desc="$1" haystack="$2" needle="$3"
  if echo "$haystack" | grep -q "$needle"; then ok "$desc"; else fail "$desc (missing: $needle)"; fi
}

# --- Register Users ---
log "=== Step 1: Register User A (owner) and User B (student) ==="

RESP_A=$(curl -sf -X POST "$GATEWAY/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"owner-phase7@test.com","password":"testpass123","name":"Owner A"}' 2>/dev/null || echo '{"user":{"id":0},"token":""}')
TOKEN_A=$(echo "$RESP_A" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")
USER_ID_A=$(echo "$RESP_A" | python3 -c "import sys,json; print(json.load(sys.stdin).get('user',{}).get('id',''))" 2>/dev/null || echo "0")
log "  User A: id=$USER_ID_A token=${TOKEN_A:0:20}..."

RESP_B=$(curl -sf -X POST "$GATEWAY/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"student-phase7@test.com","password":"testpass123","name":"Student B"}' 2>/dev/null || echo '{"user":{"id":0},"token":""}')
TOKEN_B=$(echo "$RESP_B" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")
USER_ID_B=$(echo "$RESP_B" | python3 -c "import sys,json; print(json.load(sys.stdin).get('user',{}).get('id',''))" 2>/dev/null || echo "0")
log "  User B: id=$USER_ID_B token=${TOKEN_B:0:20}..."

# --- Step 2: User A creates class ---
log "=== Step 2: User A creates class ==="

RESP_CREATE=$(curl -sf -w "\n%{http_code}" -X POST "$GATEWAY/classes" \
  -H "$(auth_header "$TOKEN_A")" \
  -H "Content-Type: application/json" \
  -d '{"name":"English A1","description":"Beginner English class","maxMembers":50}' 2>/dev/null)
HTTP_CODE=$(echo "$RESP_CREATE" | tail -1)
BODY=$(echo "$RESP_CREATE" | head -1)
assert_status "Create class returns 201" "201" "$HTTP_CODE"

CLASS_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
INVITE_CODE=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('inviteCode',''))" 2>/dev/null || echo "")
log "  Class ID: $CLASS_ID, Invite Code: $INVITE_CODE"
assert_contains "Invite code is 8 chars" "$INVITE_CODE" ".\{8\}"

# --- Step 3: User B joins class by invite code ---
log "=== Step 3: User B joins class by invite code ==="

RESP_JOIN=$(curl -sf -w "\n%{http_code}" -X POST "$GATEWAY/classes/$INVITE_CODE/join" \
  -H "$(auth_header "$TOKEN_B")" \
  -H "Content-Type: application/json" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_JOIN" | tail -1)
assert_status "Join class returns 200" "200" "$HTTP_CODE"

# --- Step 4: Verify class detail ---
log "=== Step 4: User B sees class detail ==="

RESP_DETAIL=$(curl -sf -w "\n%{http_code}" "$GATEWAY/classes/$CLASS_ID" \
  -H "$(auth_header "$TOKEN_B")" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_DETAIL" | tail -1)
assert_status "Class detail returns 200" "200" "$HTTP_CODE"

# --- Step 5: List members ---
log "=== Step 5: List members ==="

RESP_MEMBERS=$(curl -sf -w "\n%{http_code}" "$GATEWAY/classes/$CLASS_ID/members" \
  -H "$(auth_header "$TOKEN_A")" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_MEMBERS" | tail -1)
assert_status "List members returns 200" "200" "$HTTP_CODE"

# --- Step 6: Add study set (create one first) ---
log "=== Step 6: User A creates study set and adds to class ==="

RESP_SS=$(curl -sf -X POST "$GATEWAY/study-sets" \
  -H "$(auth_header "$TOKEN_A")" \
  -H "Content-Type: application/json" \
  -d '{"title":"Phase7 Test Set","description":"Test"}' 2>/dev/null || echo '{}')
SS_ID=$(echo "$RESP_SS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
log "  Study set ID: $SS_ID"

if [ -n "$SS_ID" ] && [ "$SS_ID" != "0" ]; then
  RESP_ADD_SS=$(curl -sf -w "\n%{http_code}" -X POST "$GATEWAY/classes/$CLASS_ID/study-sets" \
    -H "$(auth_header "$TOKEN_A")" \
    -H "Content-Type: application/json" \
    -d "{\"studySetId\":$SS_ID}" 2>/dev/null)
  HTTP_CODE=$(echo "$RESP_ADD_SS" | tail -1)
  assert_status "Add study set to class returns 200" "200" "$HTTP_CODE"
fi

# --- Step 7: User B views study sets ---
log "=== Step 7: User B views class study sets ==="

RESP_SS_LIST=$(curl -sf -w "\n%{http_code}" "$GATEWAY/classes/$CLASS_ID/study-sets" \
  -H "$(auth_header "$TOKEN_B")" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_SS_LIST" | tail -1)
assert_status "List class study sets returns 200" "200" "$HTTP_CODE"

# --- Step 8: Activity feed ---
log "=== Step 8: Check activity feed ==="

RESP_FEED=$(curl -sf -w "\n%{http_code}" "$GATEWAY/activity?limit=10" \
  -H "$(auth_header "$TOKEN_A")" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_FEED" | tail -1)
assert_status "Activity feed returns 200" "200" "$HTTP_CODE"

BODY=$(echo "$RESP_FEED" | head -1)
ITEM_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('items',[])))" 2>/dev/null || echo "0")
log "  Activity items count: $ITEM_COUNT"

# --- Step 9: Leave class ---
log "=== Step 9: User B leaves class ==="

RESP_LEAVE=$(curl -sf -w "\n%{http_code}" -X DELETE "$GATEWAY/classes/$CLASS_ID/members/me" \
  -H "$(auth_header "$TOKEN_B")" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_LEAVE" | tail -1)
assert_status "Leave class returns 200" "200" "$HTTP_CODE"

# --- Step 10: Delete class ---
log "=== Step 10: User A deletes class ==="

RESP_DEL=$(curl -sf -w "\n%{http_code}" -X DELETE "$GATEWAY/classes/$CLASS_ID" \
  -H "$(auth_header "$TOKEN_A")" 2>/dev/null)
HTTP_CODE=$(echo "$RESP_DEL" | tail -1)
assert_status "Delete class returns 200" "200" "$HTTP_CODE"

# --- Step 11: Verify class gone ---
log "=== Step 11: Verify class is deleted ==="

HTTP_CODE=$(curl -so /dev/null -w "%{http_code}" "$GATEWAY/classes/$CLASS_ID" \
  -H "$(auth_header "$TOKEN_A")" 2>/dev/null || echo "404")
assert_status "Deleted class returns 404" "404" "$HTTP_CODE"

# --- Step 12: Study set still exists in Study service ---
log "=== Step 12: Verify study set still exists in Study service ==="

if [ -n "$SS_ID" ] && [ "$SS_ID" != "0" ]; then
  HTTP_CODE=$(curl -so /dev/null -w "%{http_code}" "$GATEWAY/study-sets/$SS_ID" \
    -H "$(auth_header "$TOKEN_A")" 2>/dev/null || echo "000")
  assert_status "Study set still exists in Study service" "200" "$HTTP_CODE"
fi

# --- Step 13: Security: User B cannot access User A's class ---
log "=== Step 13: Security — User B cannot access deleted class ==="

HTTP_CODE=$(curl -so /dev/null -w "%{http_code}" "$GATEWAY/classes/$CLASS_ID" \
  -H "$(auth_header "$TOKEN_B")" 2>/dev/null || echo "404")
assert_status "User B cannot access deleted class" "404" "$HTTP_CODE"

# --- Summary ---
echo ""
echo "========================================="
echo "  Phase 7 E2E Results: $PASS passed, $FAIL failed"
echo "========================================="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
