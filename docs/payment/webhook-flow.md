# Webhook Flow & Idempotency

## Luong Webhook SePay

```
SePay Server
    │
    │  POST /v1/payments/webhooks/sepay
    │  Header: Authorization: Apikey <key>
    │  Body: { id, transferType, code, transferAmount, ... }
    ▼
Gateway (forward raw, KHONG strip/add headers)
    │
    ▼
Payment Service: POST /v1/payments/webhooks/sepay
    │
    ├─ 1. Verify Apikey header
    │       Sai → {"success":false} HTTP 200 (khong credit)
    │
    ├─ 2. Parse JSON body
    │       Loi parse → {"success":false} HTTP 200
    │
    ├─ 3. Chi xu ly transferType = "in"
    │       "out" → {"success":true} HTTP 200 (bo qua)
    │
    ├─ 4. Kiem tra field "code" co ma DEP_xxx khong
    │       null/empty → {"success":true} HTTP 200 (giao dich khong co ma)
    │
    ├─ 5. Tim PaymentOrder WHERE sepay_order_code = code
    │       Khong tim thay → {"success":true} HTTP 200 (giao dich la, bo qua)
    │
    ├─ 6. CreditDepositIfPaid (idempotent, DB transaction)
    │       │
    │       ├─ SELECT ... FOR UPDATE (lock order row)
    │       ├─ IF status != PENDING → "already_processed" (da xu ly roi)
    │       ├─ IF amount != transferAmount → "amount_mismatch" (sai tien)
    │       │       → Log ERROR, gui admin alert
    │       │       → RETURN, khong credit
    │       ├─ INSERT wallet_transaction (credit, ref_id=payload.id)
    │       │       ON CONFLICT (ref_id WHERE type='deposit') DO NOTHING
    │       │       → Neu conflict: da credit roi, bo qua
    │       └─ UPDATE payment_order SET status='PAID', webhook_received_at=NOW()
    │
    └─ 7. Return {"success":true} HTTP 200 LUON LUON
            SePay co auto-retry → tra 200 de SePay khong retry vo han
```

## Idempotency: 3 Lop Bao Ve

### Lop 1: Status Check (FOR UPDATE)
```sql
SELECT * FROM payment_order WHERE id = $1 FOR UPDATE;
-- Neu status != 'PENDING' → return "already_processed"
```
Bao ve khi 2 webhook den gan cung luc.

### Lop 2: Partial Unique Index (DB constraint)
```sql
CREATE UNIQUE INDEX "uq_wt_deposit_ref"
  ON "wallet_transaction" ("ref_id")
  WHERE "type" = 'deposit';
```
SePay transaction ID (`payload.id`) lam dedup key.
INSERT thu 2 cung ref_id → conflict → DO NOTHING → khong double credit.

### Lop 3: Amount Check
```go
if order.AmountVnd != payload.TransferAmount {
    // Log ERROR + alert admin
    return "amount_mismatch"
}
```
Khach chuyen sai so tien → KHONG credit → admin xu ly thu cong.

## Amount Mismatch Flow

```
Khach chuyen 80,000₫ nhung don la 100,000₫
    │
    ▼
SePay gui webhook: transferAmount = 80000
    │
    ▼
Payment service: 80000 != 100000 → amount_mismatch
    │
    ├─ Log ERROR chi tiet:
    │     orderId, orderCode, userId
    │     expected: 100,000₫
    │     actual: 80,000₫
    │     sepayTxId: 123456
    │
    ├─ Gui admin alert (Slack/email)
    │
    └─ Tra {"success":true} HTTP 200 (de SePay khong retry)
         → Admin xu ly thu cong: refund hoac credit man
```

## Admin Manual Credit

Khi can credit thu cong (refund, dieu chinh, amount mismatch):

```bash
POST /v1/admin/wallet/credit
Authorization: Bearer <admin-token>
{
  "user_id": "uuid",
  "amount_vnd": 80000,
  "note": "Hoan tien don DEP1A2B... (chuyen sai so tien 100k → 80k)"
}
```

Insert 1 wallet_transaction (type=refund, direction=credit) vao ledger.
Khong lien ket voi payment_order (xu ly thu cong, audit trail rieng).

## SePay Auto-Retry Policy

SePay gui lai webhook neu nhan duoc:
- HTTP status != 200
- Timeout (> 30 giay)
- Network error

Vi vay: LUON tra HTTP 200 + `{"success":true}`. Xu ly loi noi bo, khong
throw exception ra ngoai. Idempotency phong double-credit khi SePay retry.
