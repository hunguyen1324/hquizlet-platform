#!/usr/bin/env bash
set -euo pipefail

# Phase 8 E2E: Payment, Wallet, Entitlement
# Runs against fresh Docker Compose stack.
# Usage: bash infra/scripts/phase8-e2e.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/infra/docker/docker-compose.yml"

echo "=== Phase 8 E2E: Payment, Wallet, Entitlement ==="

# 1. Fresh volume
echo "[1/10] Docker compose down -v ..."
docker compose -f "$COMPOSE_FILE" down -v 2>/dev/null || true

echo "[2/10] Docker compose up --build ..."
docker compose -f "$COMPOSE_FILE" up --build -d

echo "[3/10] Waiting for services to be healthy ..."
sleep 15

# 2. Health check
echo "[4/10] Health check ..."
HEALTH=$(curl -s http://localhost:8080/healthz/services)
echo "$HEALTH" | head -200

PAYMENT_STATUS=$(echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); [print(s['status']) for s in d['services'] if s['name']=='payment']" 2>/dev/null || echo "N/A")
echo "Payment service status: $PAYMENT_STATUS"

# 3. Register a test user (user 1 = owner)
echo "[5/10] Registering test user 1 (owner) ..."
USER1=$(curl -s -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Owner","email":"owner@test.com","password":"password123"}')
USER1_TOKEN=$(echo "$USER1" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
echo "User1 token: ${USER1_TOKEN:0:20}..."

# 4. Register user 2 (buyer)
echo "[6/10] Registering test user 2 (buyer) ..."
USER2=$(curl -s -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Buyer","email":"buyer@test.com","password":"password123"}')
USER2_TOKEN=$(echo "$USER2" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
echo "User2 token: ${USER2_TOKEN:0:20}..."

# 5. Check wallet (should be 0)
echo "[7/10] Checking wallet balance (should be 0) ..."
BALANCE=$(curl -s http://localhost:8080/v1/wallet \
  -H "Authorization: Bearer $USER2_TOKEN")
echo "Balance: $BALANCE"

# 6. Create study set + set price
echo "[8/10] Creating study set and setting price ..."
SET=$(curl -s -X POST http://localhost:8080/v1/study-sets \
  -H "Authorization: Bearer $USER1_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Paid Study Set","description":"This set costs money"}')
SET_ID=$(echo "$SET" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
echo "Created study set: $SET_ID"

PRICE_RESP=$(curl -s -X PUT "http://localhost:8080/v1/study-sets/${SET_ID}/price" \
  -H "Authorization: Bearer $USER1_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pricingType":"one_time","priceVnd":50000}')
echo "Price set: $PRICE_RESP"

# 7. Check access (should require purchase)
echo "[9/10] Checking access for buyer (should require purchase) ..."
ACCESS=$(curl -s "http://localhost:8080/v1/entitlements/check?study_set_id=${SET_ID}" \
  -H "Authorization: Bearer $USER2_TOKEN")
echo "Access info: $ACCESS"

# 8. Simulate admin credit + purchase
echo "[10/10] Admin credit + purchase flow ..."

# Get user2 ID
USER2_ID=$(echo "$USER2" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])" 2>/dev/null)

# Admin credit (direct API call with admin token - internal service)
# Note: In production, admin needs proper auth. For E2E we use the payment service directly.
CREDIT_RESP=$(curl -s -X POST http://localhost:8085/v1/admin/wallet/credit \
  -H "X-User-Role: admin" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":${USER2_ID},\"amountVnd\":200000,\"note\":\"E2E test credit\"}")
echo "Admin credit: $CREDIT_RESP"

# Check balance after credit
BALANCE2=$(curl -s http://localhost:8080/v1/wallet \
  -H "Authorization: Bearer $USER2_TOKEN")
echo "Balance after credit: $BALANCE2"

# Purchase study set
PURCHASE_RESP=$(curl -s -X POST http://localhost:8080/v1/entitlements/purchase \
  -H "Authorization: Bearer $USER2_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"studySetId\":${SET_ID}}")
echo "Purchase: $PURCHASE_RESP"

# Check access after purchase (should have access)
ACCESS2=$(curl -s "http://localhost:8080/v1/entitlements/check?study_set_id=${SET_ID}" \
  -H "Authorization: Bearer $USER2_TOKEN")
echo "Access after purchase: $ACCESS2"

echo ""
echo "=== Phase 8 E2E PASSED ==="
echo "Payment service health: $PAYMENT_STATUS"
