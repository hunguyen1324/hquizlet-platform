# Sprint 1 – Integration Checklist
# Dev 5 – [INT-06]
# Dùng checklist này để test end-to-end trước khi merge vào integration/sprint-1-core

## Chuẩn bị

```bash
# Đảm bảo stack đang chạy
docker compose -f infra/docker/docker-compose.yml up --build -d

# Chờ health OK
curl -s http://localhost:8080/healthz/services | jq .
```

---

## 1. Health Check

- [ ] `GET /healthz` → `{"service":"gateway","status":"ok"}`
- [ ] `GET /healthz/services` → auth, study online

---

## 2. Auth Flow

### Register
```bash
curl -s -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test1234!","display_name":"Test User"}' | jq .
```
- [ ] HTTP 201
- [ ] Trả về `user.id`, `user.email`, `tokens.access_token`

### Login
```bash
ACCESS_TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test1234!"}' | jq -r '.tokens.access_token')
echo "Token: $ACCESS_TOKEN"
```
- [ ] HTTP 200
- [ ] `ACCESS_TOKEN` không rỗng

### Me
```bash
curl -s http://localhost:8080/v1/auth/me \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq .
```
- [ ] HTTP 200
- [ ] Trả về đúng user

### Logout
```bash
curl -s -X POST http://localhost:8080/v1/auth/logout \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```
- [ ] HTTP 204

### Me sau logout
```bash
curl -s http://localhost:8080/v1/auth/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```
- [ ] HTTP 401

---

## 3. Study Set Flow

> Cần ACCESS_TOKEN hợp lệ từ bước 2.

### Tạo study set
```bash
SET_ID=$(curl -s -X POST http://localhost:8080/v1/study-sets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"title":"IELTS Vocab","description":"Band 7 words","flashcards":[{"term":"Ephemeral","definition":"Short-lived"},{"term":"Ubiquitous","definition":"Present everywhere"}]}' | jq -r '.id')
echo "Set ID: $SET_ID"
```
- [ ] HTTP 201
- [ ] Trả về `id`, `title`, `flashcards` array

### List study sets
```bash
curl -s http://localhost:8080/v1/study-sets \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq .
```
- [ ] HTTP 200
- [ ] `data` chứa set vừa tạo

### Chi tiết study set
```bash
curl -s http://localhost:8080/v1/study-sets/$SET_ID \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq .
```
- [ ] HTTP 200
- [ ] Có `flashcards` array

### Update study set
```bash
curl -s -X PUT http://localhost:8080/v1/study-sets/$SET_ID \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"title":"IELTS Vocab – Updated"}' | jq .
```
- [ ] HTTP 200
- [ ] `title` đã đổi

---

## 4. Flashcard Flow

### Thêm flashcard
```bash
CARD_ID=$(curl -s -X POST http://localhost:8080/v1/study-sets/$SET_ID/flashcards \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"term":"Verbose","definition":"Using more words than needed"}' | jq -r '.id')
echo "Card ID: $CARD_ID"
```
- [ ] HTTP 201

### Update flashcard
```bash
curl -s -X PUT http://localhost:8080/v1/flashcards/$CARD_ID \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"definition":"Using too many words"}' | jq .
```
- [ ] HTTP 200

### Star flashcard
```bash
curl -s -X POST http://localhost:8080/v1/flashcards/$CARD_ID/star \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq .
```
- [ ] HTTP 200, `starred: true`
- [ ] Gọi lại → `starred: false` (toggle)

### Xóa flashcard
```bash
curl -s -X DELETE http://localhost:8080/v1/flashcards/$CARD_ID \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```
- [ ] HTTP 204

---

## 5. Authorization Guards

```bash
# Truy cập study set của user khác phải bị 403
curl -s http://localhost:8080/v1/study-sets/00000000-0000-0000-0000-000000000000 \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```
- [ ] HTTP 403 hoặc 404

---

## 6. Ownership Check

```bash
# Đăng ký user thứ 2
TOKEN2=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"other@example.com","password":"Other1234!"}' | jq -r '.tokens.access_token')

# Cố xóa study set của user 1
curl -s -X DELETE http://localhost:8080/v1/study-sets/$SET_ID \
  -H "Authorization: Bearer $TOKEN2"
```
- [ ] HTTP 403

---

## 7. CORS

```bash
curl -s -X OPTIONS http://localhost:8080/v1/auth/login \
  -H "Origin: http://localhost:5173" \
  -H "Access-Control-Request-Method: POST" \
  -v 2>&1 | grep -i "access-control"
```
- [ ] `Access-Control-Allow-Origin: http://localhost:5173`
- [ ] `Access-Control-Allow-Methods` có POST

---

## 8. Frontend Build

```bash
cd apps/web && pnpm build
```
- [ ] Build pass, không có lỗi TypeScript

---

## 9. Migration Safety

```bash
# Chạy lại migration nhiều lần không lỗi
migrate -path services/auth/migrations \
        -database "$DATABASE_URL" up
migrate -path services/auth/migrations \
        -database "$DATABASE_URL" up  # lần 2 phải OK (no-op)
```
- [ ] Lần 2 chạy không báo lỗi

---

## Done khi

- [ ] Tất cả checklist trên pass
- [ ] Không có secret/token trong source code
- [ ] `docker compose up --build` chạy được từ repo root
- [ ] Frontend build pass
