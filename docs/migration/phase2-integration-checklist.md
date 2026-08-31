# Phase 2 – Integration Checklist
# Dev 5 – [P2-INT-04]
# Chạy checklist này trước khi merge bất kỳ PR nào vào main trong Phase 2.

## 0. Chuẩn bị

```bash
# Clone mới về và không dùng cache cũ
git pull --rebase origin main
docker compose -f infra/docker/docker-compose.yml down -v  # xóa volume cũ nếu cần
docker compose -f infra/docker/docker-compose.yml up --build -d

# Đợi health OK (chạy lại vài lần nếu cần)
sleep 20
curl -s http://localhost:8080/healthz/services | python3 -m json.tool
```

---

## 1. Docker Build Gate

```bash
docker compose -f infra/docker/docker-compose.yml build 2>&1 | tail -30
```

- [ ] Build **không** lỗi (không có `ERROR` hoặc exit non-zero)
- [ ] Không có service nào dùng `go mod download` thất bại

```bash
# Kiểm tra go.sum đã commit
git status services/auth/go.sum services/study/go.sum services/gateway/go.sum
```

- [ ] Không file `go.sum` nào bị modified mà chưa commit

---

## 2. Health Check Gate

```bash
curl -s http://localhost:8080/healthz
```
- [ ] `{"service":"gateway","status":"ok"}`

```bash
curl -s http://localhost:8080/healthz/services | python3 -m json.tool
```
- [ ] `auth` status = `"ok"`
- [ ] `study` status = `"ok"`
- [ ] `quiz` status = `"ok"` (hoặc `"offline"` nếu chưa implement)

---

## 3. Auth Flow

### Register
```bash
curl -s -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Phase2 Tester","email":"p2test@example.com","password":"Test1234!"}' \
  | python3 -m json.tool
```
- [ ] HTTP 201
- [ ] Trả về `authenticated: true`, `token` không rỗng, `user.id` > 0
- [ ] `user.name` = "Phase2 Tester", `user.email` = "p2test@example.com"

```bash
# Thử register lại cùng email → phải 409
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Dup","email":"p2test@example.com","password":"Test1234!"}'
```
- [ ] HTTP 409

### Login
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"p2test@example.com","password":"Test1234!"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "TOKEN=$TOKEN"
```
- [ ] TOKEN không rỗng

```bash
# Sai password → 401
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"p2test@example.com","password":"WrongPass"}'
```
- [ ] HTTP 401

### Me
```bash
curl -s http://localhost:8080/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
- [ ] `authenticated: true`
- [ ] `user.email` = "p2test@example.com"

### Logout
```bash
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/v1/auth/logout \
  -H "Authorization: Bearer $TOKEN"
```
- [ ] HTTP 204

### Me sau logout → phải 401
```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/v1/auth/me \
  -H "Authorization: Bearer $TOKEN"
```
- [ ] HTTP 401

### Re-login để lấy token cho các bước tiếp theo
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"p2test@example.com","password":"Test1234!"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
```

---

## 4. Study Set Flow

### Tạo study set
```bash
SET_ID=$(curl -s -X POST http://localhost:8080/v1/study-sets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"IELTS Vocab Phase2","description":"Band 7 words"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "SET_ID=$SET_ID"
```
- [ ] HTTP 201
- [ ] SET_ID là số nguyên > 0

### List study sets
```bash
curl -s http://localhost:8080/v1/study-sets \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
- [ ] HTTP 200
- [ ] Set vừa tạo có trong danh sách

### Search (P2-STUDY-03)
```bash
curl -s "http://localhost:8080/v1/study-sets?q=IELTS" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
- [ ] Kết quả chỉ chứa set có "IELTS" trong title (nếu backend implement)
  - **Note**: Nếu backend chưa implement filter, result là toàn bộ sets → ghi rõ "filter not yet implemented" vào PR

### Chi tiết study set
```bash
curl -s "http://localhost:8080/v1/study-sets/$SET_ID" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
- [ ] HTTP 200, có `flashcards` array

### Update study set
```bash
curl -s -X PUT "http://localhost:8080/v1/study-sets/$SET_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"IELTS Vocab Phase2 – Updated"}' | python3 -m json.tool
```
- [ ] HTTP 200, `title` đã đổi

---

## 5. Flashcard Flow

### Thêm flashcard
```bash
CARD_ID=$(curl -s -X POST "http://localhost:8080/v1/study-sets/$SET_ID/flashcards" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"term":"Ephemeral","definition":"Lasting for a very short time"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "CARD_ID=$CARD_ID"
```
- [ ] HTTP 201, CARD_ID > 0

### Bulk save (P2-STUDY-02)
```bash
curl -s -X PUT "http://localhost:8080/v1/study-sets/$SET_ID/flashcards/bulk" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"flashcards\":[{\"id\":$CARD_ID,\"term\":\"Ephemeral\",\"definition\":\"Short-lived\"},{\"term\":\"Ubiquitous\",\"definition\":\"Present everywhere\"}]}" \
  | python3 -m json.tool
```
- [ ] HTTP 200, trả về danh sách flashcards
  - **Note**: Nếu endpoint chưa implement → ghi "bulk not yet implemented" vào PR

### Update flashcard
```bash
curl -s -X PUT "http://localhost:8080/v1/flashcards/$CARD_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"definition":"Very short-lived"}' | python3 -m json.tool
```
- [ ] HTTP 200, `definition` đã đổi

### Star toggle
```bash
curl -s -X POST "http://localhost:8080/v1/flashcards/$CARD_ID/star" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
- [ ] HTTP 200, `starred: true`

```bash
# Toggle lại → false
curl -s -X POST "http://localhost:8080/v1/flashcards/$CARD_ID/star" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```
- [ ] `starred: false`

### Xóa flashcard
```bash
curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://localhost:8080/v1/flashcards/$CARD_ID" \
  -H "Authorization: Bearer $TOKEN"
```
- [ ] HTTP 204

---

## 6. Ownership Guard

```bash
# Tạo user 2
TOKEN2=$(curl -s -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Other User","email":"other2@example.com","password":"Test1234!"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# User 2 cố xóa set của user 1 → 403
curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://localhost:8080/v1/study-sets/$SET_ID" \
  -H "Authorization: Bearer $TOKEN2"
```
- [ ] HTTP 403

```bash
# User không có token → 401
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/v1/study-sets
```
- [ ] HTTP 401

---

## 7. CORS Check

```bash
curl -s -X OPTIONS http://localhost:8080/v1/auth/login \
  -H "Origin: http://localhost:5173" \
  -H "Access-Control-Request-Method: POST" \
  -v 2>&1 | grep -i "access-control"
```
- [ ] `Access-Control-Allow-Origin: http://localhost:5173`
- [ ] `Access-Control-Allow-Methods` chứa `POST`

---

## 8. Frontend Build Gate

```bash
cd apps/web
npm install
npm run build 2>&1 | tail -20
```
- [ ] Build pass, không lỗi TypeScript
- [ ] Không có warning `any` hoặc type error nghiêm trọng

---

## 9. Source Code Sanity Check

```bash
# Không còn mock trong flow chính
git grep "mockLogin\|mockRegister\|MOCK_SETS" apps/web/src/features/ apps/web/src/main.tsx
```
- [ ] Không có output (hoặc chỉ trong `lib/mock/`)

```bash
# Không có conflict markers
git grep "<<<<<<<\|=======\|>>>>>>>" -- "*.ts" "*.tsx" "*.go" "*.yaml"
```
- [ ] Không có output

```bash
# go.sum phải tồn tại và được commit
ls services/auth/go.sum services/study/go.sum services/gateway/go.sum
```
- [ ] Tất cả file tồn tại

---

## 10. Phase 2 Folder Gate (nếu Dev 2 đã implement P2-STUDY-04)

```bash
FOLDER_ID=$(curl -s -X POST http://localhost:8080/v1/folders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"My Folder","description":"Phase 2 test"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "FOLDER_ID=$FOLDER_ID"
```
- [ ] HTTP 201 (hoặc 404 nếu chưa route vào gateway → ghi note)

---

## Pass Criteria Phase 2

| Hạng mục | Điều kiện |
| --- | --- |
| Docker build | Pass không lỗi |
| Health | gateway + auth + study = `ok` |
| Auth | register/login/me/logout qua gateway |
| Study set | CRUD pass, ownership enforced |
| Flashcard | add/update/star/delete pass |
| Frontend | `npm run build` pass |
| Source | Không còn mock trong flow chính |
| Contract | OpenAPI v2.0.0 commit |

---

_Checklist này do Dev 5 maintain. Mọi endpoint mới phải thêm vào đây trước khi merge._
