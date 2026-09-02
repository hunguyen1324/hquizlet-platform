# HQuizlet Platform - Phase 8 Execution Plan (5 Developers)

## 1. Muc Tieu Phase 8

Phase 8 xay dung Payment, Wallet va Entitlement domain end-to-end, port trung
thanh logic tu repo `hquizlet` (TypeScript/tRPC) sang Go, su dung SePay lam
payment gateway (memo-based/chuyen khoan ngan hang, QR VietQR).

Ket qua cuoi phase:

- User nap tien vao vi qua chuyen khoan ngan hang BIDV + QR VietQR.
- SePay gui webhook → payment service credit vi tu dong, idempotent.
- User mua study set bang vi (atomic debit + cap entitlement).
- Study service bao ve noi dung: chi tra flashcard khi user co quyen.
- Admin xem danh sach don nap, giao dich vi, kha nang credit/refund thu cong.
- Vi tinh lai moi lan tu so cai (khong co cot balance), idempotent theo moi
  huong: dedup theo SePay transaction ID, unique index (user, study_set).
- `services/payment` la service doc lap: go.mod rieng, port 8085, health check
  rieng, PostgreSQL rieng, Docker Compose service rieng.
- Gateway route toan bo `/v1/wallet`, `/v1/payments/` va `/v1/entitlements/`
  qua Payment service voi auth va spoofed-header stripping.
- OpenAPI, migrations, backend/frontend tests, security tests va Docker
  fresh-volume E2E deu xanh truoc khi danh dau GO.

---

## 2. Baseline Va Dieu Kien Bat Dau

### 2.1 Trang thai repo hien tai

- Phase 7 GO gate: Phase 8 chi duoc bat dau khi Phase 7 release gate da dong
  (fresh-volume E2E co evidence day du).
- Gateway hien co `services/auth` (8081), `services/study` (8082),
  `services/quiz` (8083), `services/class` (8084).
- `services/payment` da co stub `main.go` voi health check va 1 route
  placeholder. Phase 8 replace toan bo stub nay bang clean architecture.
- PostgreSQL chua co bang payment. Phase 8 them migration moi.
- Frontend chua co trang wallet, deposit, paywall.
- OpenAPI hien o version `1.5.0`; Phase 8 tang len `1.6.0`.
- Repo `hquizlet` (TypeScript) la source of truth cho logic thanh toan:
  `packages/api/src/lib/sepay.ts`, `packages/api/src/router/payment.ts`,
  `packages/api/src/lib/checkAccess.ts`, `packages/api/src/lib/deposit.ts`,
  `apps/nextjs/src/app/api/sepay/webhook/route.ts`.

### 2.2 Prerequisite tu Phase 7

- Dong Phase 7 fresh-volume gate: gan commit SHA + output vao Phase 7 release
  gate report truoc khi merge bat ky production code Phase 8 nao.
- Xac nhan Study service co internal API `/internal/study/study-sets/{id}`
  tra ve `{ owner_user_id, is_public }` de Payment service xac nhan ownership
  khi purchase.
- Xac nhan Auth service internal API `/internal/auth/verify` on dinh (dung
  tu Phase 2).
- Giu toan bo Phase 4–7 golden tests xanh. Phase 8 khong duoc tao regression.

---

## 3. Pham Vi

### 3.1 In scope

- `services/payment`: service Go moi voi clean architecture day du (config,
  http/handler, service, repository, model, migration, middleware).
- PostgreSQL migrations: `wallet_transaction`, `payment_order`,
  `study_set_price`, `entitlement`.
- SePay integration: sinh ma don `DEP_xxx` (memo-based), lay thong tin TK BIDV
  + VA, build QR VietQR URL, verify webhook API key.
- Webhook endpoint: idempotent credit, dedup theo SePay transaction ID,
  amount mismatch alert.
- Rate limit: toi da 5 don PENDING trong 10 phut/user (phong spam don rac).
- Wallet ledger: so du = SUM(credit) - SUM(debit), khong luu balance.
- Purchase: atomic DB transaction (debit + entitlement), kiem tra so du,
  kiem tra ownership, kiem tra da mua chua.
- Entitlement API: check quyen truy cap, lay thong tin mua.
- Study service paywall: gateway/Study service tu choi tra flashcard detail
  khi khong co entitlement (hoac la owner, hoac free).
- Admin: list orders, list transactions, credit/refund thu cong (admin-only).
- Gateway routes cho `/v1/wallet`, `/v1/payments/`, `/v1/entitlements/`.
- Docker Compose: them `payment` service, cap nhat `gateway` env vars va
  `healthz/services`.
- OpenAPI `1.6.0`: them tat ca Payment, Wallet, Entitlement schemas va
  endpoints, kem examples.
- Frontend: trang Wallet (so du + lich su giao dich), trang Deposit (chon so
  tien → QR + polling), Paywall component tren Study Set detail, Admin panel
  don/giao dich.
- Backend tests, frontend tests, security tests, webhook simulation test,
  integration test, E2E test.
- Phase 8 release gate report.
- Environment variables documentation cho SEPAY_*.

### 3.2 Out of scope

- Subscription / recurring payment.
- Refund tu dong qua SePay API (manual refund bang admin panel la du).
- Multi-gateway (PayOS, MoMo, ZaloPay) — chi SePay trong phase nay.
- Invoice PDF generation.
- Mobile payment UI (Phase 9+ sau khi co `apps/mobile`).
- Phan tich doanh thu / bao cao tai chinh nang cao.

---

## 4. Kien Truc Payment Service

### 4.1 Cau truc thu muc

```
services/payment/
  cmd/server/main.go          -- entry point
  internal/
    config/
      config.go               -- doc env, validate SEPAY_*
    http/
      handler/
        wallet.go             -- GET /v1/wallet, GET /v1/wallet/transactions
        order.go              -- POST /v1/payments/orders, GET /v1/payments/orders/{id}
        webhook.go            -- POST /v1/payments/webhooks/sepay
        purchase.go           -- POST /v1/entitlements/purchase
        entitlement.go        -- GET /v1/entitlements/check, GET /v1/entitlements
        price.go              -- PUT /v1/study-sets/{id}/price (owner)
        admin.go              -- GET /v1/admin/payments/orders (admin only)
      middleware/
        auth.go               -- doc X-User-Id, X-User-Role tu gateway
        request_id.go
        logging.go
      router.go               -- wiring tat ca routes
    model/
      wallet.go               -- WalletTransaction, PaymentOrder, Entitlement, StudySetPrice
    repository/
      wallet_repo.go          -- GetBalance, ListTransactions, InsertTransaction
      order_repo.go           -- CreateOrder, GetOrderByCode, GetOrderByID, UpdateStatus
      entitlement_repo.go     -- GetEntitlement, InsertEntitlement, CheckAccess
      price_repo.go           -- UpsertPrice, GetPrice
    service/
      wallet_svc.go           -- business logic cho vi
      order_svc.go            -- tao don, rate limit, SePay call
      webhook_svc.go          -- xu ly webhook, idempotency, credit
      purchase_svc.go         -- atomic debit + entitlement
      access_svc.go           -- kiem tra quyen truy cap study set
    sepay/
      client.go               -- SePay REST client (get bank account, VA)
      qr.go                   -- buildVietQrUrl
      code.go                 -- generateOrderCode("DEP")
      verify.go               -- verifyWebhookApiKey
    store/
      db.go                   -- postgres pool init
      redis.go                -- (optional, rate limit backup)
  migrations/
    001_payment_tables.sql
    002_dedup_index.sql
  go.mod
  Dockerfile
```

### 4.2 Port map

| Service   | Port |
|-----------|------|
| gateway   | 8080 |
| auth      | 8081 |
| study     | 8082 |
| quiz      | 8083 |
| class     | 8084 |
| payment   | 8085 |

---

## 5. PostgreSQL Migrations

### 5.1 `001_payment_tables.sql`

```sql
-- So cai bat bien: KHONG co cot balance, so du = SUM(credit) - SUM(debit)
CREATE TABLE IF NOT EXISTS "wallet_transaction" (
  "id"         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id"    uuid        NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
  "type"       text        NOT NULL
                           CHECK ("type" IN ('deposit','purchase','refund','adjustment')),
  "amount_vnd" integer     NOT NULL,
  "direction"  text        NOT NULL
                           CHECK ("direction" IN ('credit','debit')),
  -- deposit: SePay transaction id (dedup key, partial unique index bên dưới)
  -- purchase: study_set_id
  "ref_id"     text,
  "note"       text,
  "created_at" timestamp   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS "idx_wt_user_created"
  ON "wallet_transaction" ("user_id", "created_at" DESC);

-- PARTIAL UNIQUE INDEX: moi SePay tx chi duoc credit 1 lan
CREATE UNIQUE INDEX IF NOT EXISTS "uq_wt_deposit_ref"
  ON "wallet_transaction" ("ref_id")
  WHERE "type" = 'deposit';

-- Moi lan nap tien qua SePay (memo-based)
CREATE TABLE IF NOT EXISTS "payment_order" (
  "id"                   uuid      PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id"              uuid      NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
  "sepay_order_code"     text      NOT NULL UNIQUE,   -- DEP_xxx (memo)
  "amount_vnd"           integer   NOT NULL,
  "status"               text      NOT NULL DEFAULT 'PENDING'
                                   CHECK ("status" IN ('PENDING','PAID','CANCELLED','EXPIRED')),
  "qr_code_url"          text,
  "expired_at"           timestamp,
  "webhook_received_at"  timestamp,
  "created_at"           timestamp NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS "idx_po_status" ON "payment_order" ("status");
CREATE INDEX IF NOT EXISTS "idx_po_user"   ON "payment_order" ("user_id");

-- Gia cua tung study set (khong co ban ghi = mac dinh free)
CREATE TABLE IF NOT EXISTS "study_set_price" (
  "study_set_id"  uuid    PRIMARY KEY,  -- no FK (cross-service), validate qua internal API
  "pricing_type"  text    NOT NULL DEFAULT 'free'
                          CHECK ("pricing_type" IN ('free','one_time')),
  "price_vnd"     integer NOT NULL DEFAULT 0,
  "updated_at"    timestamp NOT NULL DEFAULT now()
);

-- Quyen truy cap noi dung tra phi
CREATE TABLE IF NOT EXISTS "entitlement" (
  "id"           uuid      PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id"      uuid      NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
  "study_set_id" uuid      NOT NULL,  -- no FK (cross-service)
  "granted_via"  text      NOT NULL
                           CHECK ("granted_via" IN ('purchase','free','admin_grant')),
  "tx_id"        uuid      REFERENCES "wallet_transaction"("id"),
  "expires_at"   timestamp,
  "created_at"   timestamp NOT NULL DEFAULT now(),
  UNIQUE ("user_id", "study_set_id")
);

CREATE INDEX IF NOT EXISTS "idx_ent_user" ON "entitlement" ("user_id");
```

### 5.2 `002_dedup_index.sql`

```sql
-- Dedup index bo sung neu can rollback + re-apply
CREATE UNIQUE INDEX IF NOT EXISTS "uq_po_sepay_order_code"
  ON "payment_order" ("sepay_order_code");
```

---

## 6. SePay Integration (Go Port)

Port trung thanh tu `packages/api/src/lib/sepay.ts`.

### 6.1 `internal/sepay/client.go`

```go
package sepay

import (
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "sync"
)

const defaultBaseURL = "https://userapi.sepay.vn/v2"

type Config struct {
    APIToken       string
    BankAccountID  string
    WebhookAPIKey  string
    BaseURL        string
}

var (
    once   sync.Once
    config *Config
)

func GetConfig() *Config {
    once.Do(func() {
        token := mustEnv("SEPAY_API_TOKEN")
        bankID := mustEnv("SEPAY_BIDV_BANK_ACCOUNT_ID")
        webhookKey := mustEnv("SEPAY_WEBHOOK_API_KEY")
        base := strings.TrimRight(os.Getenv("SEPAY_API_BASE_URL"), "/")
        if base == "" {
            base = defaultBaseURL
        }
        config = &Config{
            APIToken:      token,
            BankAccountID: bankID,
            WebhookAPIKey: webhookKey,
            BaseURL:       base,
        }
    })
    return config
}

type BankAccount struct {
    ID                string
    AccountNumber     string
    AccountHolderName string
    BankShortName     string
    BankBin           string
    VANumber          *string
    VAHolderName      *string
}

var (
    bankAccountMu    sync.Mutex
    bankAccountCache *BankAccount
)

// GetBankAccount lay TK ngan hang tu SePay, cache trong process.
func GetBankAccount() (*BankAccount, error) {
    bankAccountMu.Lock()
    defer bankAccountMu.Unlock()
    if bankAccountCache != nil {
        return bankAccountCache, nil
    }
    cfg := GetConfig()
    resp, err := doGet(cfg, fmt.Sprintf("%s/bank-accounts", cfg.BaseURL))
    if err != nil {
        return nil, err
    }
    // ... parse JSON, find by BankAccountID, fill VA
    // (chi tiet implementation trong PR)
    bankAccountCache = &BankAccount{ /* ... */ }
    return bankAccountCache, nil
}

func headers(cfg *Config) map[string]string {
    return map[string]string{
        "Authorization": "Bearer " + cfg.APIToken,
        "Content-Type":  "application/json",
    }
}

func doGet(cfg *Config, url string) (*http.Response, error) {
    req, _ := http.NewRequest(http.MethodGet, url, nil)
    for k, v := range headers(cfg) {
        req.Header.Set(k, v)
    }
    return http.DefaultClient.Do(req)
}

func mustEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        panic("missing required env: " + key)
    }
    return v
}
```

### 6.2 `internal/sepay/code.go`

```go
package sepay

import (
    "crypto/rand"
    "encoding/hex"
    "strings"
)

// GenerateOrderCode sinh ma DEP_xxx: "DEP" + 16 hex chars = 19 ky tu.
// Prefix phai khop Payment code structure tren my.sepay.vn (prefix=DEP).
func GenerateOrderCode() string {
    b := make([]byte, 8)
    _, _ = rand.Read(b)
    return "DEP" + strings.ToUpper(hex.EncodeToString(b))
}
```

### 6.3 `internal/sepay/qr.go`

```go
package sepay

import "fmt"

// BuildVietQrURL tra URL anh QR VietQR (khong can key).
// Tham so noi dung la "des" (KHONG phai "note") — xem docs vietqr.app.
func BuildVietQrURL(accountNumber, bankBin string, amountVnd int, note string) string {
    return fmt.Sprintf(
        "https://vietqr.app/img?acc=%s&bank=%s&amount=%d&des=%s&template=compact",
        accountNumber, bankBin, amountVnd, note,
    )
}
```

### 6.4 `internal/sepay/verify.go`

```go
package sepay

import "fmt"

// VerifyWebhook kiem tra header "Authorization: Apikey <key>".
func VerifyWebhook(authHeader string) bool {
    if authHeader == "" {
        return false
    }
    return authHeader == fmt.Sprintf("Apikey %s", GetConfig().WebhookAPIKey)
}
```

---

## 7. API Contract (OpenAPI 1.6.0)

### 7.1 Wallet

| Method | Route                          | Auth     | Mo ta                              |
|--------|--------------------------------|----------|------------------------------------|
| GET    | `/v1/wallet`                   | required | So du hien tai (tinh tu ledger)    |
| GET    | `/v1/wallet/transactions`      | required | Lich su giao dich (phan trang)     |

**GET /v1/wallet response:**
```json
{
  "balance": 250000
}
```

**GET /v1/wallet/transactions query params:** `limit` (1–100, default 20), `offset` (default 0)

**GET /v1/wallet/transactions response:**
```json
{
  "items": [
    {
      "id": "uuid",
      "type": "deposit",
      "direction": "credit",
      "amount_vnd": 100000,
      "label": "Nạp tiền",
      "note": null,
      "created_at": "2026-09-02T10:00:00Z"
    }
  ],
  "total": 1
}
```

### 7.2 Payment Orders

| Method | Route                           | Auth     | Mo ta                                      |
|--------|----------------------------------|----------|--------------------------------------------|
| POST   | `/v1/payments/orders`            | required | Tao don nap tien qua SePay                 |
| GET    | `/v1/payments/orders/{id}`       | required | Trang thai don nap tien (poll)             |
| POST   | `/v1/payments/webhooks/sepay`    | none     | Webhook SePay (Apikey header)              |

**POST /v1/payments/orders request:**
```json
{ "amount_vnd": 100000 }
```
Validation: `amount_vnd` >= 10000, <= 50000000, integer.

**POST /v1/payments/orders response:**
```json
{
  "order_id": "uuid",
  "order_code": "DEP1A2B3C4D5E6F7G",
  "bank_account_number": "12345678901",
  "bank_account_holder": "CONG TY ABC",
  "bank_name": "BIDV",
  "amount_vnd": 100000,
  "qr_code_url": "https://vietqr.app/img?..."
}
```

**GET /v1/payments/orders/{id} response:**
```json
{
  "status": "PENDING",
  "amount_vnd": 100000,
  "created_at": "2026-09-02T10:00:00Z",
  "qr_code_url": "https://vietqr.app/img?..."
}
```

**POST /v1/payments/webhooks/sepay request (tu SePay):**
```json
{
  "id": 123456,
  "gateway": "BIDV",
  "transactionDate": "2026-09-02 10:05:00",
  "code": "DEP1A2B3C4D5E6F7G",
  "transferType": "in",
  "transferAmount": 100000,
  "content": "DEP1A2B3C4D5E6F7G",
  "referenceCode": "..."
}
```

**POST /v1/payments/webhooks/sepay response:** HTTP 200 `{ "success": true }` luon luon (du loi).

### 7.3 Entitlements & Purchase

| Method | Route                                      | Auth     | Mo ta                                     |
|--------|--------------------------------------------|----------|-------------------------------------------|
| POST   | `/v1/entitlements/purchase`                | required | Mua study set bang vi (atomic)            |
| GET    | `/v1/entitlements/check?study_set_id=uuid` | optional | Kiem tra quyen truy cap study set         |
| GET    | `/v1/entitlements`                         | required | Danh sach study set user da mua           |

**POST /v1/entitlements/purchase request:**
```json
{ "study_set_id": "uuid" }
```

**POST /v1/entitlements/purchase response:**
```json
{ "balance": 150000, "price_vnd": 100000 }
```

**GET /v1/entitlements/check response:**
```json
{
  "pricing_type": "one_time",
  "price_vnd": 100000,
  "has_access": true,
  "requires_purchase": false,
  "is_owner": false,
  "granted_via": "purchase"
}
```

### 7.4 Study Set Price (Owner)

| Method | Route                         | Auth       | Mo ta                        |
|--------|-------------------------------|------------|------------------------------|
| PUT    | `/v1/study-sets/{id}/price`   | owner only | Gan gia cho study set        |

**PUT /v1/study-sets/{id}/price request:**
```json
{ "pricing_type": "one_time", "price_vnd": 50000 }
```

### 7.5 Admin

| Method | Route                           | Auth      | Mo ta                                |
|--------|---------------------------------|-----------|--------------------------------------|
| GET    | `/v1/admin/payments/orders`     | admin     | List tat ca don nap tien             |
| GET    | `/v1/admin/wallet/transactions` | admin     | List tat ca giao dich vi             |
| POST   | `/v1/admin/wallet/credit`       | admin     | Credit thu cong (refund, dieu chinh) |

---

## 8. Webhook Idempotency Logic

Port trung thanh tu `packages/api/src/lib/deposit.ts`. Logic:

```
func CreditDepositIfPaid(db, paymentOrderID, webhookTxID string, transferAmount int) string:
  BEGIN TRANSACTION
    SELECT * FROM payment_order WHERE id = paymentOrderID FOR UPDATE
    IF order.status != 'PENDING' → RETURN "already_processed"
    IF order.amount_vnd != transferAmount → RETURN "amount_mismatch"
    -- Dedup: partial unique index (ref_id WHERE type='deposit')
    INSERT INTO wallet_transaction (user_id, type='deposit', direction='credit',
      amount_vnd=transferAmount, ref_id=webhookTxID, note='Nạp tiền qua SePay')
    -- On conflict do nothing → da credit roi
    UPDATE payment_order SET status='PAID', webhook_received_at=NOW()
      WHERE id=paymentOrderID
  COMMIT
  RETURN "credited"
```

Neu `amount_mismatch`: ghi log + gui admin alert (Slack webhook hoac email).
Tra HTTP 200 de SePay khong retry vo han.

---

## 9. Phan Chia Cong Viec 5 Developer

### Dev 1: Payment Service Core + SePay (4 ngay)

**Ngay 1-2: Service skeleton + SePay client**
- [ ] Tao `services/payment/cmd/server/main.go` (replace stub)
- [ ] `internal/config/config.go`: doc + validate SEPAY_*, DB_URL, PORT
- [ ] `internal/store/db.go`: pgx pool, ping health
- [ ] `internal/sepay/`: port toan bo 4 file (client, code, qr, verify)
- [ ] `internal/http/middleware/auth.go`: doc X-User-Id, X-User-Role
- [ ] `internal/http/middleware/request_id.go` + `logging.go`
- [ ] `internal/http/router.go`: wiring + CORS
- [ ] Chay duoc: `GET /healthz` → 200

**Ngay 3-4: Order handler + SePay integration test**
- [ ] `internal/model/wallet.go`: struct PaymentOrder, WalletTransaction,
  Entitlement, StudySetPrice
- [ ] `internal/repository/order_repo.go`: CreateOrder, GetByCode, GetByID,
  UpdateStatus, CountPendingByUser
- [ ] `internal/service/order_svc.go`: rate limit (5 PENDING/10min),
  GenerateOrderCode, GetBankAccount, BuildVietQrURL, insert order
- [ ] `internal/http/handler/order.go`: POST /v1/payments/orders,
  GET /v1/payments/orders/{id}
- [ ] Test: `TestCreateDepositOrder_Success`, `TestCreateDepositOrder_RateLimit`
- [ ] `.env.example` them SEPAY_API_TOKEN, SEPAY_BIDV_BANK_ACCOUNT_ID,
  SEPAY_WEBHOOK_API_KEY, SEPAY_API_BASE_URL

---

### Dev 2: Webhook + Wallet Ledger (4 ngay)

**Ngay 1-2: Webhook handler (idempotent credit)**
- [ ] `internal/repository/wallet_repo.go`: InsertTransaction (upsert-safe),
  GetBalance (SUM credit/debit), ListTransactions
- [ ] `internal/service/webhook_svc.go`: port `creditDepositIfPaid`:
  DB transaction, FOR UPDATE, amount check, partial unique index dedup,
  admin alert khi mismatch
- [ ] `internal/http/handler/webhook.go`:
  - Verify `Authorization: Apikey <key>`
  - Parse payload
  - Chi xu ly `transferType = "in"`
  - Tra 200 + `{"success":true}` luon
  - Log tat ca cases
- [ ] Test webhook voi cURL:
  ```bash
  curl -X POST http://localhost:8085/v1/payments/webhooks/sepay \
    -H "Authorization: Apikey <key>" \
    -H "Content-Type: application/json" \
    -d '{"id":1,"transferType":"in","code":"DEP...","transferAmount":100000}'
  ```
- [ ] Test idempotency: gui 3 lan cung payload → chi credit 1 lan

**Ngay 3-4: Wallet API + Admin**
- [ ] `internal/service/wallet_svc.go`: GetBalance, GetTransactions
- [ ] `internal/http/handler/wallet.go`:
  GET /v1/wallet, GET /v1/wallet/transactions
- [ ] `internal/http/handler/admin.go`:
  GET /v1/admin/payments/orders, GET /v1/admin/wallet/transactions,
  POST /v1/admin/wallet/credit (admin role check)
- [ ] Test: `TestGetBalance`, `TestListTransactions`, `TestAdminCredit_Forbidden`

---

### Dev 3: Purchase + Entitlement + Study Paywall (4 ngay)

**Ngay 1-2: Purchase (atomic debit + entitlement)**
- [ ] `internal/repository/entitlement_repo.go`:
  GetEntitlement, InsertEntitlement, ListByUser
- [ ] `internal/repository/price_repo.go`: UpsertPrice, GetPrice
- [ ] `internal/service/access_svc.go`: port `getStudySetAccessInfo`:
  - GetPrice (khong co ban ghi = free)
  - IsOwner: goi Study internal API `/internal/study/study-sets/{id}` → owner_user_id
  - HasEntitlement: query entitlement table
  - Tra StudySetAccessInfo struct
- [ ] `internal/service/purchase_svc.go`: port `purchaseStudySet`:
  - GetPrice → kiem tra paid
  - IsOwner check
  - HasEntitlement check
  - GetBalance check
  - DB transaction: INSERT wallet_transaction (debit) + INSERT entitlement
- [ ] `internal/http/handler/purchase.go`:
  POST /v1/entitlements/purchase
- [ ] `internal/http/handler/entitlement.go`:
  GET /v1/entitlements/check, GET /v1/entitlements
- [ ] `internal/http/handler/price.go`:
  PUT /v1/study-sets/{id}/price (owner check qua Study internal API)
- [ ] Test: `TestPurchase_Success`, `TestPurchase_InsufficientBalance`,
  `TestPurchase_AlreadyOwned`, `TestPurchase_IsOwner`

**Ngay 3-4: Study service paywall integration**
- [ ] Study service: them `GET /internal/study/entitlement-check?user_id=&study_set_id=`
  → goi Payment internal API `/internal/payment/entitlements/check`
- [ ] Study service: bao ve `GET /v1/study-sets/{id}/flashcards`:
  - Neu study set la `one_time` va viewer khong co quyen → 402/403
  - Owner va free set → khong can check
- [ ] Gateway: them route `/internal/payment/` forward den payment service
  (chi tu internal services, khong expose ra ngoai)
- [ ] Test: `TestStudySetFlashcards_Paywall`, `TestStudySetFlashcards_Owner_Bypass`

---

### Dev 4: Gateway Routes + Docker + Infrastructure (4 ngay)

**Ngay 1: Gateway routes**
- [ ] Them Payment service URL vao gateway config (PAYMENT_SERVICE_URL=http://payment:8085)
- [ ] Gateway routes (authenticatedProxy pattern):
  - `GET /v1/wallet` → payment
  - `GET /v1/wallet/transactions` → payment
  - `POST /v1/payments/orders` → payment
  - `GET /v1/payments/orders/{id}` → payment
  - `POST /v1/entitlements/purchase` → payment
  - `GET /v1/entitlements/check` → payment (unauthenticated ok)
  - `GET /v1/entitlements` → payment
  - `PUT /v1/study-sets/{id}/price` → payment
  - `GET /v1/admin/payments/orders` → payment (admin only)
  - `GET /v1/admin/wallet/transactions` → payment (admin only)
  - `POST /v1/admin/wallet/credit` → payment (admin only)
- [ ] Webhook route KHONG qua authenticatedProxy:
  - `POST /v1/payments/webhooks/sepay` → payment (forward raw, khong strip/add headers)
  - SePay tu xac thuc bang Apikey header
- [ ] Cap nhat `GET /healthz/services` them payment status

**Ngay 2: Docker Compose**
- [ ] Them `payment` service vao `docker-compose.yml`:
  ```yaml
  payment:
    build: ./services/payment
    ports:
      - "8085:8085"
    environment:
      - PORT=8085
      - DATABASE_URL=postgres://...
      - SEPAY_API_TOKEN=${SEPAY_API_TOKEN}
      - SEPAY_BIDV_BANK_ACCOUNT_ID=${SEPAY_BIDV_BANK_ACCOUNT_ID}
      - SEPAY_WEBHOOK_API_KEY=${SEPAY_WEBHOOK_API_KEY}
      - SEPAY_API_BASE_URL=${SEPAY_API_BASE_URL:-}
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8085/healthz"]
      interval: 10s
      timeout: 5s
      retries: 5
  ```
- [ ] Them SEPAY_* vao `.env.example` va `infra/env.template`
- [ ] `services/payment/Dockerfile`: multi-stage, non-root user
- [ ] Migration runner: tich hop vao payment service startup
  (tuong tu auth/study)

**Ngay 3-4: OpenAPI + Internal API**
- [ ] Cap nhat `redocly.yaml` + mo ta OpenAPI len version `1.6.0`
- [ ] Them tat ca Payment/Wallet/Entitlement schemas:
  - `WalletBalanceResponse`, `WalletTransactionItem`, `WalletTransactionList`
  - `CreateDepositOrderRequest`, `DepositOrderCreated`, `DepositOrderStatus`
  - `SePayWebhookPayload`, `WebhookResponse`
  - `PurchaseRequest`, `PurchaseResponse`
  - `StudySetAccessInfo`, `EntitlementList`
  - `SetStudySetPriceRequest`
  - `AdminCreditRequest`
- [ ] Them examples day du cho moi endpoint
- [ ] `GET /internal/payment/entitlements/check`: internal endpoint cho Study
  service goi (khong qua gateway)
- [ ] Test: `TestGatewayRoutes_Payment`, `TestHealthzServices_Payment`

---

### Dev 5: Frontend UI (4 ngay)

**Ngay 1: Wallet page + Transaction list**

Port tu `apps/nextjs/src/app/wallet/page.tsx`.

- [ ] `apps/web/src/app/wallet/page.tsx`:
  - So du hien tai (fetch `GET /v1/wallet`)
  - Nut "Nap tien" → `/payment/deposit`
  - Lich su giao dich (fetch `GET /v1/wallet/transactions`)
  - Credit: mu xanh la `+`, debit: mu do la `-`
  - Format VND: `xxx.xxx₫`
  - Loading skeleton, empty state ("Chua co giao dich nao")
  - Phan trang (Load more hoac offset)

**Ngay 2: Deposit page (QR + polling)**

Port tu `apps/nextjs/src/app/payment/deposit/page.tsx`.

- [ ] `apps/web/src/app/payment/deposit/page.tsx`:
  - Chon so tien (preset: 50k, 100k, 200k, 500k, 1tr)
  - Hien thi so du hien tai
  - Nut "Thanh toan" → POST /v1/payments/orders
  - Loading state trong khi tao don
  - Sau khi tao don: hien thi WaitingCard
- [ ] `WaitingCard` component:
  - QR VietQR (img src = qrCodeUrl)
  - Thong tin tai khoan: STK, chu TK, ngan hang, so tien
  - Noi dung chuyen khoan (noi bat, copy button)
  - Poll GET /v1/payments/orders/{id} moi 3 giay
  - Khi status = PAID → hien thi thanh cong + nut "Xem vi"
  - Spinner "Dang cho thanh toan..."

**Ngay 3: Paywall component + Study Set detail**
- [ ] `StudySetPaywall` component (port tu `apps/nextjs/src/components/payment/study-set-paywall.tsx`):
  - Fetch `GET /v1/entitlements/check?study_set_id=...`
  - Neu `has_access = true` → render noi dung binh thuong
  - Neu `requires_purchase = true`:
    - Hien thi gia + nut "Mua ngay (xxx₫)"
    - Hien thi so du hien tai
    - Khi bam → POST /v1/entitlements/purchase
    - Thanh cong → reload + hien thi noi dung
    - So du khong du → "So du khong du, Nap tien"
  - Neu `pricing_type = free` → khong hien thi gi
- [ ] Tich hop `StudySetPaywall` vao Study Set detail page
  (bao ve phan flashcard list)
- [ ] Owner controls: "Gan gia" button → PUT /v1/study-sets/{id}/price
  (chi hien thi voi owner)

**Ngay 4: Admin panel + Navigation**
- [ ] `apps/web/src/app/admin/payments/page.tsx`:
  - List tat ca don nap tien (admin only)
  - Cot: user, ma don, so tien, trang thai, ngay tao
  - Filter theo status
- [ ] `apps/web/src/app/admin/wallet/page.tsx`:
  - List tat ca giao dich vi (admin only)
  - Form credit thu cong: user_id, amount, note
- [ ] Navigation: them "Vi" link vao header/sidebar user menu
- [ ] Guard: redirect /wallet, /payment/deposit → login neu chua dang nhap
- [ ] Error states: 402, 403, network error
- [ ] Test: `TestWalletPage`, `TestDepositPage_QRDisplay`, `TestPaywall_RequiresPurchase`

---

## 10. Test Plan

### 10.1 Unit Tests (backend)

| File | Test |
|------|------|
| `sepay/code_test.go` | GenerateOrderCode bat dau DEP, 19 ky tu, unique |
| `sepay/qr_test.go` | BuildVietQrURL chua `acc=`, `bank=`, `amount=`, `des=` |
| `sepay/verify_test.go` | VerifyWebhook: valid key, invalid key, empty |
| `service/order_svc_test.go` | Rate limit 5 PENDING/10min, tao order thanh cong |
| `service/webhook_svc_test.go` | Credit thanh cong, idempotency (2 lan cung tx_id), amount mismatch |
| `service/purchase_svc_test.go` | Success, insufficient balance, already owned, is_owner |
| `service/access_svc_test.go` | Free set, paid set + has access, paid set + no access |
| `repository/wallet_repo_test.go` | GetBalance SUM logic, partial unique index enforce |

### 10.2 Integration Tests

- `TestWebhook_FullFlow`: tao order → gia lap webhook → kiem tra balance tang
- `TestPurchase_FullFlow`: nap tien (admin credit) → mua study set → kiem tra entitlement
- `TestStudySet_Paywall`: study set paid → user khong quyen → 402; sau mua → 200
- `TestWebhook_Idempotency`: goi webhook 5 lan cung payload → balance chi tang 1 lan
- `TestWebhook_AmountMismatch`: transferAmount != order.amount_vnd → "amount_mismatch", balance khong tang

### 10.3 Security Tests

- Webhook khong co Apikey → tra `{"success":false}` (HTTP 200, khong credit)
- Webhook sai Apikey → tra `{"success":false}`
- User A khong the xem don nap tien cua User B (GET /v1/payments/orders/{id})
- User binh thuong goi /v1/admin/* → 403
- Purchase study set cua chinh minh → 400 "Ban la chu so huu"
- Purchase study set free → 400 "Set nay mien phi"

### 10.4 Frontend Tests

- `WalletPage`: render so du, danh sach giao dich, empty state
- `DepositPage`: chon so tien, tao don, hien QR, poll status
- `StudySetPaywall`: free set (khong hien paywall), paid + no access (hien gia), paid + has access (hien noi dung)
- `WaitingCard`: poll 3s, PAID → success screen

### 10.5 E2E (Docker fresh-volume)

```bash
# infra/scripts/phase8-e2e.sh
docker compose down -v
docker compose up --build -d
sleep 15

# Health check
curl http://localhost:8080/healthz/services | jq .payment

# Dang ky user
USER=$(curl -s -X POST http://localhost:8080/v1/auth/register ...)

# Admin credit vi (de test, khong qua SePay)
curl -X POST http://localhost:8080/v1/admin/wallet/credit \
  -H "X-Admin-Token: ..." \
  -d '{"user_id":"...","amount_vnd":500000,"note":"E2E test credit"}'

# Tao study set + gan gia
SET_ID=$(curl -s -X POST http://localhost:8080/v1/study-sets ...)
curl -X PUT http://localhost:8080/v1/study-sets/$SET_ID/price \
  -d '{"pricing_type":"one_time","price_vnd":100000}'

# User 2 kiem tra quyen → requires_purchase=true
curl http://localhost:8080/v1/entitlements/check?study_set_id=$SET_ID

# User 2 mua → balance giam, entitlement duoc cap
curl -X POST http://localhost:8080/v1/entitlements/purchase \
  -d '{"study_set_id":"'"$SET_ID"'"}'

# User 2 kiem tra quyen → has_access=true
curl http://localhost:8080/v1/entitlements/check?study_set_id=$SET_ID

# Simulate SePay webhook
curl -X POST http://localhost:8080/v1/payments/webhooks/sepay \
  -H "Authorization: Apikey $SEPAY_WEBHOOK_API_KEY" \
  -d '{"id":999,"transferType":"in","code":"DEP...","transferAmount":100000}'

echo "Phase 8 E2E PASSED"
```

---

## 11. Environment Variables

Them vao `.env.example` va `infra/env.template`:

```env
# SePay Payment Gateway
# Lay tai my.sepay.vn → API → Token
SEPAY_API_TOKEN=

# UUID cua tai khoan BIDV tren SePay (Company → Bank accounts → ID)
SEPAY_BIDV_BANK_ACCOUNT_ID=

# API Key de SePay xac thuc khi gui webhook den he thong
# Lay tai my.sepay.vn → Webhooks → API Key
SEPAY_WEBHOOK_API_KEY=

# (Optional) Ghi de base URL cho sandbox/dev
# De trong = dung production: https://userapi.sepay.vn/v2
# Sandbox: https://userapi-sandbox.sepay.vn/v2
SEPAY_API_BASE_URL=

# (Optional) IP whitelist tu SePay (ngan chan webhook gia)
# Lien he SePay support de lay IP list chinh thuc
# SEPAY_ALLOWED_IPS=103.x.x.x,103.x.x.y
```

**Huong dan cau hinh SePay (them vao docs/payment/sepay-setup.md):**

1. Dang nhap my.sepay.vn → tao tai khoan tich hop.
2. Them tai khoan ngan hang BIDV → sao chep UUID vao `SEPAY_BIDV_BANK_ACCOUNT_ID`.
3. Tao VA neu dung BIDV (Company → Bank accounts → VA tab).
4. Cau hinh Payment code structure: prefix = `DEP`, loai = "So va chu", do dai 6-30.
5. Tao Webhook URL → tro ve `https://yourdomain/v1/payments/webhooks/sepay`.
6. Sao chep Webhook API Key → `SEPAY_WEBHOOK_API_KEY`.
7. Tao API Token → `SEPAY_API_TOKEN`.
8. Test voi sandbox truoc khi production.

---

## 12. Definition of Done

Phase 8 chi duoc danh dau GO khi tat ca dieu kien sau deu dat:

### Backend
- [ ] `GET /healthz/services` tra ve payment: ok
- [ ] Tao order → tra ve orderCode + QR URL + thong tin TK
- [ ] Webhook idempotency: 5 lan cung payload → balance chi tang 1 lan
- [ ] Amount mismatch → khong credit, log warning, admin alert
- [ ] Purchase: atomic debit + entitlement trong 1 DB transaction
- [ ] Study set paid → user khong quyen → 402/403
- [ ] Admin API co role guard (non-admin → 403)
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] All security tests pass

### Frontend
- [ ] Wallet page hien thi so du + lich su giao dich chinh xac
- [ ] Deposit page tao don + hien QR + poll → success khi PAID
- [ ] Paywall component chay dung (free/owned/requires purchase)
- [ ] Owner co the gan gia cho study set cua minh
- [ ] Admin panel xem duoc don + giao dich

### Infrastructure
- [ ] `docker compose up --build` chay sach tren fresh volume
- [ ] Phase 8 E2E script xanh (gan commit SHA + output vao release gate)
- [ ] OpenAPI 1.6.0 validated (redocly lint pass)
- [ ] SEPAY_* documented trong `.env.example`
- [ ] Dockerfile cho payment service: multi-stage, non-root

### Documentation
- [ ] `docs/payment/sepay-setup.md`: huong dan cau hinh SePay step-by-step
- [ ] `docs/payment/webhook-flow.md`: mo ta luong webhook + idempotency
- [ ] Phase 8 release gate report duoc luu vao `docs/phase/phase-8-release-gate-YYYY-MM-DD.md`
- [ ] README.md cap nhat: them Payment service vao service list

---

## 13. Rui Ro Va Giam Thieu

| Rui ro | Giam thieu |
|--------|------------|
| SePay khong gui webhook (loi mang) | SePay co auto-retry; log tat ca; manual credit qua admin API khi can |
| Amount mismatch (user chuyen sai so tien) | Admin alert ngay; manual credit/refund qua admin panel |
| Replay attack (webhook gia) | Verify Apikey; dedup theo SePay transaction ID; (optionally) IP whitelist |
| Race condition double-credit | DB transaction + FOR UPDATE + partial unique index phong double insert |
| Spam tao don rac | Rate limit 5 PENDING/10min/user trong DB |
| SEPAY_* bi lo | `.env.example` khong chua gia tri that; `.env` trong `.gitignore` |
| Payment service sap anh huong toan bo app | Gateway co timeout + circuit breaker; cac service khac khong phu thuoc payment trong happy path |

---

## 14. Timeline Tong Hop (4 Ngay, 5 Dev Song Song)

```
Ngay 1: [Dev1] service skeleton + SePay client
         [Dev2] wallet repo + webhook handler skeleton
         [Dev3] entitlement repo + purchase service
         [Dev4] gateway routes + Docker Compose payment service
         [Dev5] wallet page UI

Ngay 2: [Dev1] order handler + rate limit
         [Dev2] webhook idempotency + admin API
         [Dev3] access service + study paywall integration
         [Dev4] OpenAPI 1.6.0 + internal API
         [Dev5] deposit page + QR + polling

Ngay 3: [Dev1] unit tests + .env.example + Dockerfile
         [Dev2] wallet API + integration tests
         [Dev3] purchase handler + entitlement API + tests
         [Dev4] E2E script + healthz update
         [Dev5] paywall component + study set detail integration

Ngay 4: [All]  security tests + bug fix + release gate report
         [All]  docker fresh-volume E2E
         [All]  code review + merge
         [All]  cap nhat docs/payment/
         [All]  danh dau Phase 8 GO
```

---

## 15. Ghi Chu Quan Trong

### Tai sao SePay memo-based (khong dung VA theo don hang)?

SePay Virtual Account (VA) theo tung don hang chi ho tro BIDV Enterprise.
Tai khoan BIDV ca nhan/doanh nghiep thuong chi dung VA chung hoac VA co dinh.
Giai phap memo-based: moi don nap sinh ma `DEP_xxx` duy nhat, khach ghi vao
noi dung chuyen khoan, SePay extract ma tu memo theo "Payment code structure"
(prefix = DEP) va gui kem trong webhook (field `code`). Khong can them SDK,
goi REST thuan.

### Tai sao khong luu balance truc tiep?

So cai bat bien (append-only ledger) la mo hinh chuan cho he thong tai chinh:
- Audit trail day du (biet moi thay doi den tu dau).
- Khong bao gio co race condition cap nhat balance.
- De rollback/recompute khi co loi.
- Phat hien gian lan: so du luon tinh lai duoc tu raw data.

So du = `SUM(amount_vnd WHERE direction='credit') - SUM(amount_vnd WHERE direction='debit')`.

### Cross-service foreign key?

`entitlement.study_set_id` va `study_set_price.study_set_id` KHONG co FK toi
bang `study_set` (o service khac). Thay vao do, Payment service goi Study
internal API de validate khi can. Day la pattern chuan trong microservices:
moi service so huu data cua minh, trao doi qua API.
