# HQuizlet Platform — Phase 2 Integration Checklist

**Owner:** Dev5 — Fullstack/Integration  
**Purpose:** reproducible gate for Docker, gateway auth propagation, API contract, Study ownership, frontend build, and Phase 2 learning flow.

## 0. Fresh environment

```bash
git pull --rebase origin main
docker compose -f infra/docker/docker-compose.yml down -v
docker compose -f infra/docker/docker-compose.yml up --build -d
curl -s http://localhost:8080/healthz/services | python3 -m json.tool
```

Required: gateway, auth, and study report `ok`.

## 1. Auth gate

Register and login through `http://localhost:8080`.

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"p2test@example.com","password":"Test1234!"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')

curl -s http://localhost:8080/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

- [ ] register/login/me succeed
- [ ] invalid token is rejected
- [ ] refresh rotates the session token
- [ ] logout invalidates the token

## 2. Gateway identity gate — P0

Protected Study/Folder requests must pass Auth verification first.

```bash
curl -i http://localhost:8080/v1/study-sets
curl -i http://localhost:8080/v1/folders
```

- [ ] no token → `401`
- [ ] invalid token → `401`
- [ ] valid token → gateway calls `/internal/auth/verify`
- [ ] client-supplied `X-User-ID` is ignored/overwritten
- [ ] verified user ID is injected as `X-User-ID`

## 3. Study set gate

```bash
SET_ID=$(curl -s -X POST http://localhost:8080/v1/study-sets \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"Phase2 Test","description":"integration"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')

curl -s "http://localhost:8080/v1/study-sets?search=Phase2&sort=updated&page=1&per_page=20" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

curl -s "http://localhost:8080/v1/study-sets/$SET_ID" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

- [ ] list response is `{items,total,page,perPage,totalPages}`
- [ ] search uses `search`, not `q`
- [ ] detail is owner-scoped
- [ ] User B cannot read/update/delete User A's set

## 4. Flashcard + transactional bulk gate

The backend contract is **POST** with `{cards:[...]}`.

```bash
curl -s -X POST "http://localhost:8080/v1/study-sets/$SET_ID/flashcards/bulk" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"cards":[{"term":"Ephemeral","definition":"Short-lived","position":0},{"term":"Ubiquitous","definition":"Present everywhere","position":1}]}' \
  | python3 -m json.tool
```

- [ ] HTTP `200`
- [ ] response contains `created`, `updated`, `deleted`
- [ ] cross-study-set card IDs are rejected
- [ ] the operation is atomic
- [ ] User B cannot mutate User A's cards

## 5. Folder gate

```bash
curl -s http://localhost:8080/v1/folders \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

curl -s -X POST http://localhost:8080/v1/folders \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Phase2 Folder","description":"integration"}' | python3 -m json.tool
```

- [ ] `/v1/folders` is routed through gateway
- [ ] folder field is `name`
- [ ] CRUD and add/remove study-set routes are protected
- [ ] ownership is enforced

## 6. Frontend contract/build gate

```bash
cd apps/web
npm ci
npm run build
```

- [ ] clean install passes
- [ ] TypeScript/Vite build passes
- [ ] API client consumes paginated StudySet response
- [ ] API client uses `POST /flashcards/bulk` and `{cards}`
- [ ] API client uses `PATCH /v1/auth/profile`
- [ ] API client uses folder `name`

## 7. Backend/Docker gate

```bash
go test ./...
go build ./...
docker compose -f infra/docker/docker-compose.yml build
docker compose -f infra/docker/docker-compose.yml up --build
```

- [ ] Auth tests pass
- [ ] Study tests pass
- [ ] fresh PostgreSQL migrations pass
- [ ] no missing `go.sum`
- [ ] `/healthz/services` reports required services `ok`

## 8. Source sanity

```bash
git grep '<<<<<<<\|=======\|>>>>>>>' -- '*.go' '*.ts' '*.tsx' '*.yaml'
git grep 'mockLogin\|mockRegister\|MOCK_SETS' apps/web/src/features apps/web/src/main.tsx
```

- [ ] no conflict markers
- [ ] no production mock flow
- [ ] no hard-coded user identity
- [ ] no client-supplied identity trusted by gateway

## Phase 2 GO criteria

All P0 checks above must pass, plus Docker, fresh migrations, backend tests/build, frontend build, OpenAPI alignment, ownership isolation, and the four learning modes using persisted data.

**Dev5 rule:** do not mark the phase gate GO based only on static review; record reproducible command output in the final PR.
