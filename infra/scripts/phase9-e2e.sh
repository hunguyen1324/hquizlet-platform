#!/bin/bash
# infra/scripts/phase9-e2e.sh
# Phase 9 E2E: File Upload flow
set -euo pipefail

echo "=== Phase 9 E2E: File Upload ==="
echo "Rebuilding stack on fresh volumes..."

docker compose -f infra/docker/docker-compose.yml down -v
docker compose -f infra/docker/docker-compose.yml up --build -d
echo "Waiting for services to be healthy..."
sleep 30

GW="http://localhost:8080"

# --- Health check ---
echo "[1] Health check"
HEALTH=$(curl -sf "$GW/healthz/services")
echo "$HEALTH" | python3 -c "
import sys, json
d = json.load(sys.stdin)
services = {s['name']: s['status'] for s in d.get('services', [])}
assert services.get('file') == 'ok', f'file service not ok: {services}'
print('  healthz OK:', list(services.keys()))
"

# --- Register + login ---
echo "[2] Auth"
REG=$(curl -sf -X POST "$GW/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"FileTest","email":"filetest@e2e.local","password":"Password123!"}')
TOKEN=$(echo "$REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
USER_ID=$(echo "$REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  User registered: $USER_ID"

# --- Presign avatar ---
echo "[3] Presign avatar"
PRESIGN=$(curl -sf -X POST "$GW/v1/files/presign" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"upload_type":"avatar","filename":"test.jpg","content_type":"image/jpeg","file_size":1024}')
FILE_ID=$(echo "$PRESIGN" | python3 -c "import sys,json; print(json.load(sys.stdin)['file_id'])")
UPLOAD_URL=$(echo "$PRESIGN" | python3 -c "import sys,json; print(json.load(sys.stdin)['upload_url'])")
echo "  file_id: $FILE_ID"

# --- Upload fake image directly to MinIO ---
echo "[4] Upload to MinIO"
printf '\xff\xd8\xff\xe0\x00\x10JFIF\x00' > /tmp/test-avatar.jpg
curl -sf -X PUT "$UPLOAD_URL" \
  -H "Content-Type: image/jpeg" \
  --data-binary @/tmp/test-avatar.jpg
echo "  Upload OK"

# --- Confirm ---
echo "[5] Confirm upload"
CONFIRM=$(curl -sf -X POST "$GW/v1/files/$FILE_ID/confirm" \
  -H "Authorization: Bearer $TOKEN")
URL=$(echo "$CONFIRM" | python3 -c "import sys,json; print(json.load(sys.stdin)['url'])")
echo "  Confirmed URL: $URL"

# --- Update profile avatar ---
echo "[6] Update profile image"
ME=$(curl -sf -X PATCH "$GW/v1/auth/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"image\":\"$URL\"}")
IMG=$(echo "$ME" | python3 -c "import sys,json; print(json.load(sys.stdin)['image'])")
echo "  Profile image: $IMG"

# --- GET /v1/auth/me verify ---
echo "[7] Verify GET /me"
ME2=$(curl -sf "$GW/v1/auth/me" -H "Authorization: Bearer $TOKEN")
echo "$ME2" | python3 -c "
import sys,json
d=json.load(sys.stdin)
user = d.get('user', {})
assert user.get('image'), 'image missing from /me response'
print('  image OK:', user['image'])
"

# --- GET /v1/files ---
echo "[8] List files + quota"
FILES=$(curl -sf "$GW/v1/files" -H "Authorization: Bearer $TOKEN")
echo "$FILES" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert d['total'] >= 1, 'expected at least 1 file'
assert d['quota']['active_files'] >= 1
print('  total files:', d['total'], '| quota:', d['quota'])
"

# --- Security: confirm another user's file ---
echo "[9] Security: confirm another user's file"
REG2=$(curl -sf -X POST "$GW/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"Attacker","email":"attacker@e2e.local","password":"Password123!"}')
TOKEN2=$(echo "$REG2" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$GW/v1/files/$FILE_ID/confirm" \
  -H "Authorization: Bearer $TOKEN2")
[ "$STATUS" == "403" ] && echo "  403 OK (not_owner)" || (echo "  FAIL: expected 403, got $STATUS" && exit 1)

# --- Invalid MIME ---
echo "[10] Security: invalid MIME type"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$GW/v1/files/presign" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"upload_type":"avatar","filename":"evil.exe","content_type":"application/x-executable","file_size":512}')
[ "$STATUS" == "400" ] && echo "  400 OK (invalid_content_type)" || (echo "  FAIL: expected 400, got $STATUS" && exit 1)

# --- Soft delete ---
echo "[11] Soft delete"
curl -sf -X DELETE "$GW/v1/files/$FILE_ID" -H "Authorization: Bearer $TOKEN" -o /dev/null
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  "$GW/v1/files/$FILE_ID" -H "Authorization: Bearer $TOKEN")
[ "$STATUS" == "404" ] && echo "  404 OK (deleted)" || (echo "  FAIL: expected 404, got $STATUS" && exit 1)

echo ""
echo "==================================================="
echo "  Phase 9 E2E: ALL CHECKS PASSED"
echo "==================================================="
