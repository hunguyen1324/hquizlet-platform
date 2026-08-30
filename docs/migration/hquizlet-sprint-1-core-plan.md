# HQuizlet Platform - Sprint 1 Core Plan

## 1. Role Lam Viec

Muc tieu tam thoi la chia team 5 dev de phat trien song song, han che conflict code va khong de frontend/backend cho nhau qua lau.

| Dev | Role | Pham vi so huu | Muc tieu chinh |
| --- | --- | --- | --- |
| Dev 1 | Backend Go Dev - Auth | `services/auth/**` | Auth, user, session, middleware, migration auth |
| Dev 2 | Backend Go Dev - Study | `services/study/**` | Study set, flashcard, folder base, migration study |
| Dev 3 | Frontend React Dev - Core App | `apps/web/src/features/auth/**`, `apps/web/src/features/dashboard/**`, `apps/web/src/features/study-sets/**` | Auth UI, dashboard, create/edit study set UI |
| Dev 4 | Frontend React Dev - Learning | `apps/web/src/features/learning/**` | Flashcards, Learn, Test, Match UI bang mock data truoc |
| Dev 5 | Fullstack/Integration Dev | `services/gateway/**`, `infra/**`, `packages/api-contracts/**`, `apps/web/src/lib/api/**`, `docs/**` | Gateway, Docker, OpenAPI, API client, integration |

## 2. Nguyen Tac Lam Song Song

1. Moi dev lam tren branch rieng, khong commit truc tiep vao `main`.
2. Moi dev chi sua file trong pham vi minh so huu, tru khi co thong nhat truoc.
3. API thay doi phai cap nhat OpenAPI truoc hoac trong cung PR.
4. Frontend dung mock data theo OpenAPI de khong phai cho backend xong moi lam.
5. Backend test bang curl/Postman/API test, khong phu thuoc frontend.
6. Dev 5 chiu trach nhiem ghep nhieu nhanh vao branch integration.
7. Moi PR phai co mo ta ngan: da lam gi, test gi, co migration/env moi khong.

## 3. Branch De Xuat

| Dev | Branch |
| --- | --- |
| Dev 1 | `feature/auth-service-core` |
| Dev 2 | `feature/study-service-core` |
| Dev 3 | `feature/web-core-pages` |
| Dev 4 | `feature/web-learning-modes` |
| Dev 5 | `feature/integration-contracts` |
| Dev 5 | `integration/sprint-1-core` |

## 4. Sprint 1 - Nen Tang Core

Thoi gian de xuat: 1 tuan.

Muc tieu Sprint 1:

- Backend co cau truc sach hon, san sang mo rong.
- Auth va Study service co migration ro rang.
- Gateway proxy route dung.
- Frontend co layout/feature folder ro rang.
- Learning modes co UI skeleton dung mock data.
- Docker Compose chay duoc full stack.
- OpenAPI v1 duoc chot de team lam song song.

## 5. Cong Viec Sprint 1 Theo Tung Dev

### Dev 1 - Backend Go Auth

Pham vi file:

```text
services/auth/**
```

Task:

| Ma task | Cong viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| BE-AUTH-01 | Refactor `services/auth` tu `main.go` sang cau truc `cmd/internal` | Co `cmd/server/main.go`, `internal/http`, `internal/service`, `internal/repository`, `internal/model`, `internal/config` | Khong |
| BE-AUTH-02 | Tach config doc tu env | `PORT`, `DATABASE_URL`, `SESSION_TTL` doc tap trung | Khong |
| BE-AUTH-03 | Tao migration auth | SQL cho `users`, `sessions`, add column an toan voi DB cu | Khong |
| BE-AUTH-04 | Hoan thien register/login/logout/me/refresh | API auth chay that voi PostgreSQL | BE-AUTH-03 |
| BE-AUTH-05 | Tao auth middleware/token verifier | Ham verify token co the dung cho service khac | BE-AUTH-04 |
| BE-AUTH-06 | Chuan hoa JSON error/response | Loi tra ve format thong nhat | Khong |

Tieu chi hoan thanh:

- Dang ky user luu DB.
- Password duoc hash bang bcrypt.
- Login tra token/session.
- `GET /v1/auth/me` tra user that neu co token.
- Logout xoa session.
- Migration chay lai khong loi.

### Dev 2 - Backend Go Study

Pham vi file:

```text
services/study/**
```

Task:

| Ma task | Cong viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| BE-STUDY-01 | Refactor `services/study` sang cau truc `cmd/internal` | Co handler/service/repository/model/config | Khong |
| BE-STUDY-02 | Tao migration study | SQL cho `study_sets`, `flashcards`, index can thiet | Khong |
| BE-STUDY-03 | Them `user_id` vao `study_sets` | Study set thuoc user | Can contract auth token tu Dev 1/Dev 5 |
| BE-STUDY-04 | CRUD study set | List/detail/create/update/delete | BE-STUDY-02 |
| BE-STUDY-05 | CRUD flashcard | Create/update/delete/list theo study set | BE-STUDY-02 |
| BE-STUDY-06 | Star/unstar flashcard | API toggle starred | BE-STUDY-05 |
| BE-STUDY-07 | Validate owner permission | User chi sua/xoa du lieu cua minh | BE-STUDY-03 |

Tieu chi hoan thanh:

- Tao/sua/xoa study set qua API.
- Them/sua/xoa flashcard qua API.
- Data reload van con trong PostgreSQL.
- Co check user ownership cho API private.
- Handler khong viet SQL truc tiep.

### Dev 3 - Frontend React Core App

Pham vi file:

```text
apps/web/src/features/auth/**
apps/web/src/features/dashboard/**
apps/web/src/features/study-sets/**
apps/web/src/components/**
```

Task:

| Ma task | Cong viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| FE-CORE-01 | Tach cau truc frontend theo feature folder | Code khong con gom tat ca trong `main.tsx` | Khong |
| FE-CORE-02 | Tao API client wrapper | Goi API qua `apps/web/src/lib/api` | Contract tu Dev 5 |
| FE-CORE-03 | Lam login/register UI | Form auth sach, co loading/error | Khong, dung mock truoc |
| FE-CORE-04 | Protected layout | Neu chua login thi ve auth screen | FE-CORE-03 |
| FE-CORE-05 | Dashboard study set | List study set, empty state, reload | Co mock data truoc |
| FE-CORE-06 | Create/edit study set page | UI tao hoc phan nhu Quizlet, nhap nhieu card | Co mock data truoc |
| FE-CORE-07 | Noi API that khi backend san sang | Auth + study set flow chay end-to-end | Dev 1, Dev 2, Dev 5 |

Tieu chi hoan thanh:

- UI dang nhap/dang ky dung duoc.
- Dashboard co danh sach hoc phan.
- Trang tao hoc phan co title, description, nhieu card term/definition.
- Co loading/error/empty states.
- Frontend build pass.

### Dev 4 - Frontend React Learning

Pham vi file:

```text
apps/web/src/features/learning/**
apps/web/src/components/learning/**
```

Task:

| Ma task | Cong viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| FE-LEARN-01 | Tao data model UI cho learning modes | Type `StudySet`, `Flashcard`, `LearningProgress` | Contract tu Dev 5 |
| FE-LEARN-02 | Flashcards mode skeleton | Lat the, next/prev, shuffle UI | Mock data |
| FE-LEARN-03 | Learn mode skeleton | Hoi dap, check answer local | Mock data |
| FE-LEARN-04 | Test mode skeleton | Cau hoi tu flashcards, input answer | Mock data |
| FE-LEARN-05 | Match mode skeleton | Ghep term/definition bang click pair | Mock data |
| FE-LEARN-06 | Noi voi study set detail data | Modes dung data that khi Dev 3 co detail page | Dev 3 |

Tieu chi hoan thanh:

- 4 mode co giao dien rieng.
- Chay duoc bang mock data.
- Khong phu thuoc backend trong Sprint 1.
- Responsive desktop/mobile co ban.

### Dev 5 - Fullstack/Integration

Pham vi file:

```text
services/gateway/**
infra/**
packages/api-contracts/**
apps/web/src/lib/api/**
docs/**
```

Task:

| Ma task | Cong viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| INT-01 | Chot OpenAPI v1 cho auth/study/flashcard | `packages/api-contracts/openapi.yaml` cap nhat | Can trao doi Dev 1, Dev 2, Dev 3 |
| INT-02 | Chuan hoa gateway proxy | Proxy auth/study/flashcard/live route dung | Khong |
| INT-03 | Chuan hoa CORS dev | Cho phep port web Docker va local dev | Khong |
| INT-04 | Chuan hoa Docker Compose | `docker compose up --build` chay gateway/auth/study/quiz/web/postgres | Khong |
| INT-05 | Tao README setup dev | Huong dan clone, pull, build, run, migrate | INT-04 |
| INT-06 | Tao integration checklist | Checklist test auth + study flow | Dev 1, Dev 2, Dev 3 |
| INT-07 | Tao branch `integration/sprint-1-core` | Ghép PR sau khi tung phan pass | Phu thuoc PR cua team |

Tieu chi hoan thanh:

- OpenAPI co route auth/study/flashcard.
- Gateway route khong chan UI.
- Docker Compose chay duoc tu repo root.
- README du cho dev moi setup.
- Co checklist test end-to-end.

## 6. Thu Tu Lam Trong Sprint 1

### Ngay 1

| Dev | Viec can lam |
| --- | --- |
| Dev 1 | Tao structure auth moi, tach config/model co ban |
| Dev 2 | Tao structure study moi, thiet ke migration study |
| Dev 3 | Tao frontend feature folder va auth/dashboard skeleton |
| Dev 4 | Tao learning folder va mock data |
| Dev 5 | Chot OpenAPI draft, sua gateway route va CORS |

### Ngay 2

| Dev | Viec can lam |
| --- | --- |
| Dev 1 | Migration users/sessions, register/login service |
| Dev 2 | CRUD study set repository/service |
| Dev 3 | Login/register UI + protected layout |
| Dev 4 | Flashcards mode skeleton |
| Dev 5 | Docker Compose health check va README setup draft |

### Ngay 3

| Dev | Viec can lam |
| --- | --- |
| Dev 1 | Logout/me/refresh + token verifier |
| Dev 2 | CRUD flashcard + star/unstar |
| Dev 3 | Dashboard + study set list mock |
| Dev 4 | Learn mode skeleton |
| Dev 5 | API client wrapper + update OpenAPI theo backend |

### Ngay 4

| Dev | Viec can lam |
| --- | --- |
| Dev 1 | Auth middleware + error format |
| Dev 2 | User ownership cho study set |
| Dev 3 | Create/edit study set page |
| Dev 4 | Test mode skeleton |
| Dev 5 | Integration branch, test Docker stack |

### Ngay 5

| Dev | Viec can lam |
| --- | --- |
| Dev 1 | Fix bug auth, review PR Dev 2 lien quan auth |
| Dev 2 | Fix bug study, review PR Dev 1 lien quan token |
| Dev 3 | Noi auth/study API that neu ready |
| Dev 4 | Match mode skeleton + responsive |
| Dev 5 | Chay integration checklist, merge vao `integration/sprint-1-core` |

## 7. Daily Sync Checklist

Moi ngay moi dev tra loi 3 cau:

1. Hom qua da xong task nao?
2. Hom nay lam task nao?
3. Dang bi chan boi API/schema/file nao?

Neu co thay doi API:

- Cap nhat OpenAPI.
- Bao Dev 3/Dev 4 neu response shape thay doi.
- Them vi du request/response vao PR.

## 8. Definition Of Done Cho Sprint 1

Sprint 1 duoc xem la xong khi:

- `docker compose -f infra/docker/docker-compose.yml up --build` chay duoc.
- Gateway `GET /healthz/services` tra status services.
- Auth register/login/me/logout chay qua gateway.
- Study set CRUD chay qua gateway.
- Flashcard CRUD chay qua gateway.
- Frontend build pass.
- Dashboard, auth UI, create study set page co UI dung duoc.
- Learning modes co skeleton bang mock/data that neu san sang.
- README co huong dan setup.
- Khong co secret/token trong source code.
