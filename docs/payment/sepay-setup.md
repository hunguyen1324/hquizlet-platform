# Huong Dan Cau Hinh SePay (Memo-Based)

## Tong Quan Mo Hinh

HQuizlet dung SePay theo mo hinh **memo-based** (chuyen khoan co noi dung):

1. He thong sinh ma don `DEP_xxx` duy nhat cho moi lan nap tien.
2. Khach chuyen khoan dung so tien + ghi ma `DEP_xxx` vao noi dung.
3. SePay phat hien giao dich khop prefix, extract ma, gui webhook.
4. He thong nhan webhook → credit vi tu dong.

Mo hinh nay khong can BIDV Enterprise, hoat dong voi tai khoan BIDV ca nhan/
doanh nghiep thuong, khong can SDK.

---

## Buoc 1: Tao Tai Khoan SePay

1. Vao [my.sepay.vn](https://my.sepay.vn) → Dang ky tai khoan doanh nghiep.
2. Xac minh email va thong tin doanh nghiep.
3. Vao **Company → General settings** → ghi nho Account ID.

---

## Buoc 2: Them Tai Khoan Ngan Hang BIDV

1. Vao **Company → Bank accounts → Add bank account**.
2. Chon ngan hang: **BIDV**.
3. Nhap so tai khoan + ten chu tai khoan.
4. Xac minh bang OTP (neu yeu cau).
5. Sau khi them thanh cong: click vao tai khoan → sao chep **UUID** (dang `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`).
6. Set gia tri nay vao: `SEPAY_BIDV_BANK_ACCOUNT_ID=<UUID>`.

### Tuy chon: Them VA (Virtual Account)

Neu BIDV yeu cau chuyen qua VA (mot so truong hop):

1. Trong chi tiet tai khoan → tab **VA → Create VA**.
2. Ghi nho so VA duoc cap.
3. SePay client se tu dong phat hien VA va dung lam dich chuyen khoan.

---

## Buoc 3: Cau Hinh Payment Code Structure

Day la buoc **bat buoc** de SePay extract ma don tu noi dung chuyen khoan.

1. Vao **Company → General settings → Payment code structure**.
2. Cau hinh:
   - **Prefix**: `DEP`
   - **Loai ky tu**: So va chu (alphanumeric)
   - **Do dai**: 16–30 ky tu
3. Luu lai.

Sau khi cau hinh, khi khach ghi `DEP1A2B3C4D5E6F7G` vao noi dung chuyen
khoan, SePay se extract `DEP1A2B3C4D5E6F7G` va tra ve field `code` trong
webhook.

> **Quan trong:** Ky tu ngay sau prefix PHAI la chu/so (khong dung dau `_`
> hoac cac ky tu dac biet).

---

## Buoc 4: Tao API Token

1. Vao **API → Tokens → Create token**.
2. Dat ten (vi du: `hquizlet-production`).
3. Sao chep token → set `SEPAY_API_TOKEN=<token>`.

> Token chi hien thi 1 lan. Luu vao noi an toan.

---

## Buoc 5: Cau Hinh Webhook

1. Vao **Webhooks → Add webhook**.
2. **URL**: `https://yourdomain.com/v1/payments/webhooks/sepay`
   - Local dev: dung ngrok hoac `https://yourapp.fly.dev/v1/payments/webhooks/sepay`
3. **Events**: chon "Transaction created" (hoac tat ca events).
4. **API Key**: SePay tu sinh hoac cho phep dat. Sao chep → set
   `SEPAY_WEBHOOK_API_KEY=<api_key>`.
5. Test webhook bang nut "Send test".

> SePay gui header: `Authorization: Apikey <SEPAY_WEBHOOK_API_KEY>`
> He thong verify header nay truoc khi xu ly.

---

## Buoc 6: Bien Moi Truong

```env
# .env (KHONG commit file nay)
SEPAY_API_TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
SEPAY_BIDV_BANK_ACCOUNT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
SEPAY_WEBHOOK_API_KEY=your-webhook-api-key-here

# De trong = dung production URL
# Dat sandbox URL khi test: https://userapi-sandbox.sepay.vn/v2
SEPAY_API_BASE_URL=
```

---

## Buoc 7: Test Webhook Thu Cong

```bash
# Simulate SePay webhook (thay the gia tri <> cho phu hop)
curl -X POST http://localhost:8085/v1/payments/webhooks/sepay \
  -H "Authorization: Apikey <SEPAY_WEBHOOK_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "id": 99999,
    "gateway": "BIDV",
    "transactionDate": "2026-09-02 10:00:00",
    "code": "<DEP_CODE_TU_ORDER>",
    "transferType": "in",
    "transferAmount": 100000,
    "content": "<DEP_CODE_TU_ORDER>",
    "referenceCode": "TEST001"
  }'

# Ket qua mong doi: {"success":true}
# Kiem tra balance tang: GET /v1/wallet
```

---

## Luong Hoan Chinh

```
User chon so tien → POST /v1/payments/orders
  → Payment service sinh DEP_xxx
  → Lay so TK BIDV tu SePay API
  → Build QR VietQR URL
  → Luu PaymentOrder (PENDING)
  → Tra ve: orderCode, STK, chu TK, ngan hang, QR URL

User chuyen khoan (hoac quet QR)
  → SePay phat hien giao dich BIDV
  → SePay extract ma DEP_xxx tu noi dung
  → SePay POST webhook → /v1/payments/webhooks/sepay

Payment service nhan webhook
  → Verify Apikey
  → Tim PaymentOrder theo sepay_order_code
  → Kiem tra so tien khop
  → DB transaction: INSERT wallet_transaction (credit) + UPDATE order PAID
  → (Dedup: neu da xu ly roi thi bo qua)

Frontend poll GET /v1/payments/orders/{id} moi 3s
  → Khi status = PAID → hien thi thanh cong
```

---

## Xu Ly Loi Thuong Gap

| Loi | Nguyen nhan | Giai phap |
|-----|-------------|-----------|
| Webhook khong den | URL sai hoac server chua public | Kiem tra URL webhook tren SePay, dung ngrok cho local dev |
| `code` = null trong webhook | Payment code structure chua cau hinh | Cau hinh prefix DEP tren my.sepay.vn |
| Amount mismatch | Khach chuyen sai so tien | Admin credit thu cong qua /v1/admin/wallet/credit |
| Double credit | Webhook gui nhieu lan | Da phong bang partial unique index (ref_id WHERE type='deposit') |
| 401 Unauthorized | Sai SEPAY_WEBHOOK_API_KEY | Kiem tra lai gia tri tren SePay → Webhooks |
