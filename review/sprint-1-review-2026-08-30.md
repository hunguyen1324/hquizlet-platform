# Sprint 1 Review - HQuizlet Platform

Ngay review: 2026-08-30

Nguoi review: PM/Tech Lead

## Ket Luan Nhanh

Chua nen qua giai doan moi.

Team da di dung huong ve mat chia role va cau truc code, nhung Sprint 1 van chua dat Definition of Done vi frontend tren `main` con dung mock data va OpenAPI van lech voi backend Study model.

## Trang Thai Tong Quan

| Hang muc | Trang thai | Ghi chu |
| --- | --- | --- |
| Backend auth refactor | Pass co dieu kien | Da tach `cmd/internal`, migration da xu ly DB cu tot hon |
| Backend study refactor | Pass co dieu kien | Da tach `cmd/internal`, migration da them cot cho DB cu |
| Dockerfile Go service | Pass | Da build `./cmd/server` thay vi service root |
| Gateway route/CORS | Pass co dieu kien | Route chinh da co, can test Docker end-to-end |
| Frontend build | Pass | `npm run build` da pass |
| Frontend noi API that | Fail | `main` van dung mock login/register/study sets |
| OpenAPI contract | Fail co dieu kien | Auth da gan dung hon, StudySet/Flashcard con lech naming |
| Learning modes | Chua merge day du | Branch learning con diverged va can rebase/merge |
| Docker compose full stack | Chua verify | Moi truong review khong co `go`/`docker` trong PATH |

## Findings

### 1. Frontend tren `main` van dung mock data

Muc do: High

File lien quan:

```text
apps/web/src/features/auth/AuthContext.tsx
apps/web/src/features/dashboard/Dashboard.tsx
```

Van de:

- Login dang dung `mockLogin`.
- Register dang dung `mockRegister`.
- Logout chua goi `/v1/auth/logout`.
- Dashboard dang load `MOCK_SETS` thay vi `/v1/study-sets`.

Anh huong:

- UI build pass nhung khong test duoc flow that.
- User co the dang nhap bang mock trong khi backend auth da san sang.
- Study set tren UI khong phan anh PostgreSQL.

Can lam:

- Merge hoac port code tu `feature/web-core-pages` ve `main`, vi branch nay da co huong goi API that.
- Tao `apiFetch`/API client dung chung thay vi moi component tu goi rieng.
- Auth flow phai goi gateway:

```text
POST /v1/auth/login
POST /v1/auth/register
POST /v1/auth/logout
GET  /v1/auth/me
```

- Dashboard/editor phai goi gateway:

```text
GET    /v1/study-sets
POST   /v1/study-sets
GET    /v1/study-sets/{id}
PUT    /v1/study-sets/{id}
DELETE /v1/study-sets/{id}
POST   /v1/study-sets/{id}/flashcards
PUT    /v1/flashcards/{id}
DELETE /v1/flashcards/{id}
POST   /v1/flashcards/{id}/star
```

### 2. OpenAPI StudySet/Flashcard con lech voi backend JSON

Muc do: High

File lien quan:

```text
packages/api-contracts/openapi.yaml
services/study/internal/model/model.go
```

Van de:

OpenAPI dang khai bao mot so field theo snake_case:

```text
user_id
created_at
updated_at
study_set_id
```

Trong khi Go model tra JSON camelCase:

```text
userId
createdAt
updatedAt
studySetId
```

Anh huong:

- Frontend bam OpenAPI se map sai field.
- API contract khong con la nguon su that.
- De phat sinh bug khi generate client/type sau nay.

Can lam:

- Sua OpenAPI theo dung JSON tag trong Go model.
- Neu muon dung snake_case thi phai doi JSON tag backend, nhung hien frontend dang theo camelCase nen nen giu camelCase.

### 3. Branch dev con diverged

Muc do: Medium

Branch can xu ly:

```text
feature/web-core-pages
feature/web-learning-modes
feature/auth-service-core
feature/study-service-core
feature/integration-contracts
```

Van de:

- Mot so branch co commit moi chua nam trong `main`.
- Mot so branch da bi `main` vuot nhieu commit.
- `feature/web-learning-modes` co force update, can review lai truoc khi merge.

Can lam:

- Rebase tung branch len `main` moi nhat.
- Neu noi dung da merge qua branch khac thi dong branch cu.
- Chi merge branch con gia tri that va build pass.

### 4. Chua verify Docker compose full stack

Muc do: Medium

Lenh can chay tren may dev co Docker:

```powershell
docker compose -f infra/docker/docker-compose.yml up --build
```

Can kiem tra:

```powershell
curl http://localhost:8080/healthz
curl http://localhost:8080/healthz/services
```

Flow can test:

1. Register user.
2. Login user.
3. Reload trang van giu session.
4. Create study set.
5. Add flashcard.
6. Open study detail.
7. Logout.

## Nhung Diem Da Lam Dung

### Backend

- Auth va Study da duoc tach sang cau truc `cmd/internal`.
- Handler/service/repository/model da ro hon ban dau.
- Migration auth da them logic xu ly DB cu cho `image`, `role`.
- Migration study da them logic xu ly DB cu cho `user_id`, `updated_at`.
- Dockerfile da sua build `./cmd/server`.

### Frontend

- Da tach feature folder:

```text
features/auth
features/dashboard
features/study-sets
```

- UI create/edit study set da dung huong.
- Build frontend pass.

### Integration/Docs

- Da co OpenAPI lon hon ban dau.
- Da co `.env.example`.
- Da co docs Sprint 1 va checklist.
- Docker Compose da co healthcheck ro hon.

## Viec Can Lam Truoc Khi Qua Giai Doan Moi

| Priority | Owner | Viec can lam | Dieu kien pass |
| --- | --- | --- | --- |
| P0 | Dev 3 | Bo mock, noi frontend voi API that | Login/register/dashboard/create set goi gateway that |
| P0 | Dev 5 | Sua OpenAPI StudySet/Flashcard ve camelCase | Contract khop Go JSON response |
| P0 | Dev 4 | Rebase va merge learning modes | Branch khong diverged, build pass |
| P1 | Dev 5 | Chay Docker compose full stack | `healthz/services` tat ca `ok` |
| P1 | QA/Dev 5 | Test core flow end-to-end | Register -> login -> create set -> add card -> logout pass |
| P1 | Dev 1 + Dev 2 | Dong/rebase branch cu | Khong con branch diverged khong ro muc dich |

## Goi Y Thu Tu Merge Tiep Theo

1. Merge/port `feature/web-core-pages` vao `main` sau khi rebase.
2. Sua OpenAPI naming trong `feature/integration-contracts`, rebase roi merge.
3. Rebase `feature/web-learning-modes`, review conflict voi `StudyDetail`, build pass roi merge.
4. Chay Docker compose full stack.
5. Neu pass, tao tag/checkpoint `sprint-1-core-complete` hoac merge sang branch integration.

## Gate Qua Giai Doan Moi

Chi qua giai doan moi khi tat ca dieu kien sau pass:

- Frontend khong con dung mock cho auth va study set core flow.
- OpenAPI khop backend response.
- `npm run build` pass.
- `docker compose -f infra/docker/docker-compose.yml up --build` pass.
- `GET /healthz/services` tra all `ok`.
- Register/login/me/logout chay qua gateway.
- Create study set/add flashcard/open detail chay qua gateway va PostgreSQL.
- Branch dev cu da merge, rebase, hoac dong ro rang.

## Ket Luan PM

Team da sua dung cac blocker ha tang quan trong, nhung Sprint 1 chua nen dong. Viec can tap trung ngay la noi frontend voi API that va dong bo lai OpenAPI. Sau khi core flow that pass tren Docker, luc do moi nen chuyen sang giai doan tiep theo nhu learning progress, folder, class hoac live quiz.
