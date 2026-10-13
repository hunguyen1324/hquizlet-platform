#!/bin/bash
# infra/scripts/phase10-e2e.sh
# Phase 1 (dashboard/iam plan) E2E: IAM authorization, privilege escalation, audit log.
set -euo pipefail

PYTHON="${IAM_E2E_PYTHON:-python3}"
if ! "$PYTHON" -c 'import json' >/dev/null 2>&1; then
  PYTHON=python
  "$PYTHON" -c 'import json' >/dev/null 2>&1 || { echo "Python 3 is required (set IAM_E2E_PYTHON)."; exit 1; }
fi

echo "=== Phase 10 E2E: IAM authorization ==="
cd "$(dirname "$0")/../.."
# Unique project names isolate both networks and named volumes on every invocation.
PROJECT="hq-iam-e2e-$(date +%s)-$$"
COMPOSE=(docker compose --project-name "$PROJECT" -f infra/docker/docker-compose.iam-e2e.yml)
cleanup() {
  result=$?
  if [ "$result" -ne 0 ]; then "${COMPOSE[@]}" logs --no-color --tail=60 || true; fi
  if [ "$result" -eq 0 ] && [ "${IAM_E2E_KEEP_STACK:-0}" = 1 ]; then
    echo "Retained isolated QA stack: $PROJECT"
  else
    "${COMPOSE[@]}" down --volumes --remove-orphans || true
  fi
  exit "$result"
}
trap cleanup EXIT
echo "Starting isolated test stack: $PROJECT"
"${COMPOSE[@]}" up --build -d --wait --wait-timeout 240

GW="http://127.0.0.1:18080"
PSQL=("${COMPOSE[@]}" exec -T postgres psql -U hquizlet -d hquizlet -tA)

json_get() { "$PYTHON" -c "import sys,json; d=json.load(sys.stdin); print($1)"; }

# --- Health check ---
echo "[1] Health check"
curl -sf "$GW/healthz/services" | "$PYTHON" -c "
import sys, json
services = {s['name']: s['status'] for s in json.load(sys.stdin).get('services', [])}
assert services.get('auth') == 'ok', f'auth service not ok: {services}'
print('  healthz OK:', list(services.keys()))
"

# --- Register a student and a teacher ---
echo "[2] Register student + teacher"
STUDENT=$(curl -sf -X POST "$GW/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"Student One","email":"iam-student@e2e.local","password":"Password123!","accountType":"student"}')
STUDENT_TOKEN=$(echo "$STUDENT" | json_get "d['token']")
STUDENT_ID=$(echo "$STUDENT" | json_get "d['user']['id']")

TEACHER=$(curl -sf -X POST "$GW/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"Teacher One","email":"iam-teacher@e2e.local","password":"Password123!","accountType":"teacher"}')
TEACHER_TOKEN=$(echo "$TEACHER" | json_get "d['token']")
TEACHER_ID=$(echo "$TEACHER" | json_get "d['user']['id']")
echo "  student=$STUDENT_ID teacher=$TEACHER_ID"

# --- Registration assigns the chosen platform role ---
echo "[3] Permissions reflect the chosen account type"
curl -sf "$GW/v1/me/permissions" -H "Authorization: Bearer $STUDENT_TOKEN" | "$PYTHON" -c "
import sys, json
d = json.load(sys.stdin)
assert 'Student' in d['platformRoles'], d
assert 'iam:AssignRole' not in d['actions'], 'student must not hold iam:AssignRole'
assert 'dashboard:ViewAdmin' not in d['actions'], 'student must not hold dashboard:ViewAdmin'
print('  student roles:', d['platformRoles'])
"
curl -sf "$GW/v1/me/permissions" -H "Authorization: Bearer $TEACHER_TOKEN" | "$PYTHON" -c "
import sys, json
d = json.load(sys.stdin)
assert 'Teacher' in d['platformRoles'], d
assert 'class:Create' in d['actions'], 'teacher must be able to create classes'
assert 'iam:AssignRole' not in d['actions'], 'teacher must not hold iam:AssignRole'
print('  teacher roles:', d['platformRoles'])
"

# --- Non-admins are blocked at the gateway ---
echo "[4] Non-admins cannot reach /v1/admin/*"
for label in student teacher; do
  [ "$label" == "student" ] && TOKEN="$STUDENT_TOKEN" || TOKEN="$TEACHER_TOKEN"
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$GW/v1/admin/users" -H "Authorization: Bearer $TOKEN")
  [ "$STATUS" == "403" ] && echo "  $label -> 403 OK" || (echo "  FAIL: $label expected 403, got $STATUS" && exit 1)
done

# --- Spoofed identity headers from a client are stripped by the gateway ---
echo "[5] Spoofed X-User-Roles / X-User-Role are stripped"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$GW/v1/admin/users" \
  -H "Authorization: Bearer $STUDENT_TOKEN" \
  -H "X-User-ID: 1" \
  -H "X-User-Role: admin" \
  -H "X-User-Roles: PlatformAdmin")
[ "$STATUS" == "403" ] && echo "  403 OK (spoofed headers ignored)" || (echo "  FAIL: expected 403 for spoofed admin, got $STATUS" && exit 1)

echo "[5a] Teacher cannot grant themselves PlatformAdmin"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/admin/assignments" \
  -H "Authorization: Bearer $TEACHER_TOKEN" -H "Content-Type: application/json" \
  -d "{\"userId\": $TEACHER_ID, \"roleKey\": \"PlatformAdmin\", \"scopeType\": \"platform\"}")
[ "$STATUS" == "403" ] || (echo "  FAIL: escalation expected 403, got $STATUS" && exit 1)

echo "[5b] Student cannot create a class, even with spoofed teacher headers"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/classes" \
  -H "Authorization: Bearer $STUDENT_TOKEN" -H "Content-Type: application/json" \
  -H "X-User-Roles: Teacher" -H "X-Class-Role: owner" \
  -d '{"name":"Unauthorized class"}')
[ "$STATUS" == "403" ] || (echo "  FAIL: student class creation expected 403, got $STATUS" && exit 1)

# --- Promote an admin through the database (documented bootstrap path) ---
echo "[6] Bootstrap PlatformAdmin via iam_assignments"
"${PSQL[@]}" -c "INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by)
  VALUES ('user', $TEACHER_ID, 'PlatformAdmin', 'platform', NULL, $TEACHER_ID)
  ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING" > /dev/null
# Re-login so the gateway's identity cache sees the new role immediately.
ADMIN_TOKEN=$(curl -sf -X POST "$GW/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"iam-teacher@e2e.local","password":"Password123!"}' | json_get "d['token']")

echo "[7] Admin console reads"
curl -sf "$GW/v1/admin/roles" -H "Authorization: Bearer $ADMIN_TOKEN" | "$PYTHON" -c "
import sys, json
keys = {r['key'] for r in json.load(sys.stdin)['roles']}
for want in ('PlatformAdmin', 'Teacher', 'Student', 'ClassOwner'):
    assert want in keys, f'missing managed role {want}: {keys}'
print('  roles:', sorted(keys))
"
curl -sf "$GW/v1/admin/users?limit=10" -H "Authorization: Bearer $ADMIN_TOKEN" | "$PYTHON" -c "
import sys, json
d = json.load(sys.stdin)
assert d['total'] >= 2, d
print('  users:', d['total'])
"

# --- Assign + revoke a role, both audited ---
echo "[8] Assign and revoke a role"
ASSIGNMENT_ID=$(curl -sf -X POST "$GW/v1/admin/assignments" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"userId\": $STUDENT_ID, \"roleKey\": \"PlatformSupport\", \"scopeType\": \"platform\"}" | json_get "d['id']")
echo "  assignment=$ASSIGNMENT_ID"

curl -sf -X DELETE "$GW/v1/admin/assignments/$ASSIGNMENT_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
echo "  revoked"

# --- An admin cannot grant an unknown role ---
echo "[9] Unknown roles are rejected"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/admin/assignments" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"userId\": $STUDENT_ID, \"roleKey\": \"SuperRoot\", \"scopeType\": \"platform\"}")
[ "$STATUS" == "422" ] && echo "  422 OK (unknown role)" || (echo "  FAIL: expected 422, got $STATUS" && exit 1)

# --- Dashboard, live, payment and moderation acceptance ---
echo "[9a] Build an assigned class and verify data isolation"
CLASS_ID=$(curl -sf -X POST "$GW/v1/classes" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d '{"name":"IAM Dashboard Class"}' | json_get "d['id']")
INVITE=$(curl -sf "$GW/v1/classes/$CLASS_ID" -H "Authorization: Bearer $ADMIN_TOKEN" | json_get "d['inviteCode']")
curl -sf -X POST "$GW/v1/classes/$INVITE/join" -H "Authorization: Bearer $STUDENT_TOKEN" >/dev/null
SET_ID=$(curl -sf -X POST "$GW/v1/study-sets" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d '{"title":"IAM public learning","visibility":"public","contentType":"flashcard"}' | json_get "d['id']")
curl -sf -X POST "$GW/v1/study-sets/$SET_ID/flashcards" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d '{"term":"hello","definition":"xin chao"}' >/dev/null
curl -sf -X POST "$GW/v1/classes/$CLASS_ID/study-sets" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d "{\"studySetId\":$SET_ID,\"dueAt\":\"2099-01-01T00:00:00Z\"}" >/dev/null
curl -sf -X POST "$GW/v1/study-sets/$SET_ID/progress" -H "Authorization: Bearer $STUDENT_TOKEN" -H "Content-Type: application/json" -d '{"mode":"learn","score":9,"total":10,"startedAt":"2026-10-13T00:00:00Z","completedAt":"2026-10-13T01:00:00Z","idempotencyKey":"iam-dashboard-session","cardResults":[]}' >/dev/null
curl -sf "$GW/v1/classes/$CLASS_ID/dashboard" -H "Authorization: Bearer $STUDENT_TOKEN" | "$PYTHON" -c "
import json,sys
d=json.load(sys.stdin)
assert d['canViewAll'] is False and d['students']==[] and d['recentActivity']==[],d
assert d['myProgress']['userId']==$STUDENT_ID and d['myProgress']['sessions']==1,d
assert d['assignedSets'][0]['dueAt'].startswith('2099-'),d
"
curl -sf "$GW/v1/classes/$CLASS_ID/permissions" -H "Authorization: Bearer $STUDENT_TOKEN" | "$PYTHON" -c "import json,sys;d=json.load(sys.stdin);assert 'class-member:ViewProgress' in d['actions'] and 'live:Host' not in d['actions'],d"
curl -sf "$GW/v1/me/study-stats" -H "Authorization: Bearer $STUDENT_TOKEN" | "$PYTHON" -c "import json,sys;d=json.load(sys.stdin);assert d['sessionsTotal']>=1,d"
OUTSIDER=$(curl -sf -X POST "$GW/v1/auth/register" -H "Content-Type: application/json" -d '{"name":"Outsider","email":"iam-outsider@e2e.local","password":"Password123!","accountType":"student"}')
OUTSIDER_TOKEN=$(echo "$OUTSIDER" | json_get "d['token']")
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$GW/v1/classes/$CLASS_ID/dashboard" -H "Authorization: Bearer $OUTSIDER_TOKEN")
[ "$STATUS" = 403 ] || { echo "FAIL: class isolation $STATUS";exit 1; }
# Completion events are consumed asynchronously, with durable storage and deduplication.
for attempt in $(seq 1 30); do
  EVENT_COUNT=$("${PSQL[@]}" -c "SELECT count(*) FROM activity_events WHERE source_event_id IS NOT NULL AND class_id=$CLASS_ID")
  [ "$EVENT_COUNT" -ge 1 ] && break
  sleep 1
done
[ "$EVENT_COUNT" -ge 1 ] || { echo 'FAIL: completion event not consumed';exit 1; }
echo "  class isolation, deadlines, progress and NATS completion OK"

echo "[9b] Students cannot host live or set prices"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/live-sessions" -H "Authorization: Bearer $STUDENT_TOKEN" -H "Content-Type: application/json" -d "{\"studySetId\":$SET_ID,\"questionCount\":1,\"questionDurationMs\":10000}")
[ "$STATUS" = 403 ] || { echo "FAIL: student hosted live $STATUS";exit 1; }
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$GW/v1/study-sets/$SET_ID/price" -H "Authorization: Bearer $STUDENT_TOKEN" -H "Content-Type: application/json" -d '{"pricingType":"one_time","priceVnd":1000}')
[ "$STATUS" = 403 ] || { echo "FAIL: student set price $STATUS";exit 1; }
curl -sf -X POST "$GW/v1/live-sessions" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d "{\"studySetId\":$SET_ID,\"classId\":$CLASS_ID,\"questionCount\":1,\"questionDurationMs\":10000}" >/dev/null

echo "[9c] Wallet credits require reauthentication and are audited"
for PASSWORD in '' 'wrong-password'; do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/admin/wallet/credit" -H "Authorization: Bearer $ADMIN_TOKEN" -H "X-Admin-Password: $PASSWORD" -H "Content-Type: application/json" -d "{\"userId\":$STUDENT_ID,\"amountVnd\":1000,\"note\":\"IAM test\"}")
  [ "$STATUS" = 401 ] || { echo "FAIL: credit accepted without password $STATUS";exit 1; }
done
curl -sf -X POST "$GW/v1/admin/wallet/credit" -H "Authorization: Bearer $ADMIN_TOKEN" -H "X-Admin-Password: Password123!" -H "Content-Type: application/json" -d "{\"userId\":$STUDENT_ID,\"amountVnd\":1000,\"note\":\"IAM test\"}" >/dev/null
curl -sf "$GW/v1/wallet" -H "Authorization: Bearer $STUDENT_TOKEN" | "$PYTHON" -c "import json,sys;assert json.load(sys.stdin)['balance']==1000"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/admin/assignments" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d "{\"userId\":$STUDENT_ID,\"roleKey\":\"PlatformAdmin\",\"scopeType\":\"platform\"}")
[ "$STATUS" = 401 ] || { echo "FAIL: granting admin bypassed confirmation $STATUS";exit 1; }

echo "[9d] Admin metrics, catalog, moderation, system and filters"
for endpoint in overview users/stats study-stats class-stats payments/stats top-content study-system classes study-sets; do
  curl -sf "$GW/v1/admin/$endpoint" -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null
done
curl -sf "$GW/v1/admin/users?role=Teacher&status=active" -H "Authorization: Bearer $ADMIN_TOKEN" | "$PYTHON" -c "import json,sys;d=json.load(sys.stdin);assert d['total']==1,d"
curl -sf "$GW/v1/admin/users/$STUDENT_ID" -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null
curl -sf "$GW/v1/admin/study-sets/$SET_ID" -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null
curl -sf -X POST "$GW/v1/admin/study-sets/$SET_ID/moderation" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d '{"hidden":true,"reason":"IAM moderation test"}' >/dev/null
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$GW/v1/study-sets/$SET_ID" -H "Authorization: Bearer $STUDENT_TOKEN")
[ "$STATUS" = 404 ] || { echo "FAIL: hidden content still readable $STATUS";exit 1; }
curl -sf -X POST "$GW/v1/admin/study-sets/$SET_ID/moderation" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d '{"hidden":false,"reason":"IAM restore test"}' >/dev/null
curl -sf "$GW/v1/study-sets/$SET_ID" -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null

echo "[9e] Quiz scoring, deduplication and immediate privilege revocation"
IAM_QA_ADMIN_TOKEN="$ADMIN_TOKEN" IAM_QA_LEARNER_TOKEN="$OUTSIDER_TOKEN" IAM_QA_CLASS_ID="$CLASS_ID" IAM_QA_LEARNER_ID="$(echo "$OUTSIDER" | json_get "d['user']['id']")" "$PYTHON" infra/scripts/dashboard-flow.py
IAM_LOAD_TOKEN="$ADMIN_TOKEN" IAM_LOAD_CLASS_ID="$CLASS_ID" "$PYTHON" infra/scripts/dashboard-load.py

# --- Disable an account ---
curl -sf "$GW/v1/admin/assignments?userId=$STUDENT_ID" -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null

echo "[10] Disable a user and verify the session is rejected"
curl -sf -X POST "$GW/v1/admin/users/$STUDENT_ID/disable" \
  -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GW/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"iam-student@e2e.local","password":"Password123!"}')
[ "$STATUS" == "401" ] || (echo "  FAIL: disabled login expected 401, got $STATUS" && exit 1)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$GW/v1/auth/profile" -H "Authorization: Bearer $STUDENT_TOKEN")
[ "$STATUS" == "401" ] && echo "  401 OK (disabled account rejected)" || (echo "  FAIL: expected 401, got $STATUS" && exit 1)
# /auth/me intentionally returns HTTP 200 with authenticated=false for invalid sessions.
curl -sf "$GW/v1/auth/me" -H "Authorization: Bearer $STUDENT_TOKEN" | "$PYTHON" -c "
import sys, json
d = json.load(sys.stdin)
assert d['authenticated'] is False and d['user'] is None, d
"

# --- Audit log captures the privileged writes ---
echo "[10a] Restart migrations preserve managed roles and session revocation"
BEFORE=$("${PSQL[@]}" -c "SELECT count(*) FROM iam_assignments WHERE principal_id=$TEACHER_ID AND role_key='Student'")
"${COMPOSE[@]}" restart auth
"${COMPOSE[@]}" up -d --wait --wait-timeout 60 auth
AFTER=$("${PSQL[@]}" -c "SELECT count(*) FROM iam_assignments WHERE principal_id=$TEACHER_ID AND role_key='Student'")
[ "$BEFORE" = "$AFTER" ] || { echo 'FAIL: restart changed Teacher to Student';exit 1; }
[ "$("${PSQL[@]}" -c "SELECT count(*) FROM sessions WHERE user_id=$STUDENT_ID AND revoked_at IS NULL")" = 0 ] || { echo 'FAIL: disabled sessions retained';exit 1; }
echo "[10b] Backfill dry run and dashboard EXPLAIN"
COMPOSE_PROJECT_NAME="$PROJECT" COMPOSE_FILE="infra/docker/docker-compose.iam-e2e.yml" bash infra/scripts/iam-backfill-dry-run.sh
mkdir -p tmp
"${COMPOSE[@]}" exec -T postgres psql -U hquizlet -d hquizlet -v ON_ERROR_STOP=1 < infra/scripts/dashboard-explain.sql > tmp/dashboard-explain.log

echo "[11] Audit log"
curl -sf "$GW/v1/admin/audit?limit=50" -H "Authorization: Bearer $ADMIN_TOKEN" | "$PYTHON" -c "
import sys, json
entries = json.load(sys.stdin)['entries']
actions = {e['action'] for e in entries}
for want in ('iam:AssignRole', 'iam:RevokeRole', 'user:Disable', 'wallet:Credit', 'study-set:Read', 'study-set:Publish'):
    assert want in actions, f'missing audit action {want}: {actions}'
print('  audited actions:', sorted(actions))
"

echo ""
echo "==================================================="
echo "Phase 10 (IAM) E2E PASSED"
echo "==================================================="
