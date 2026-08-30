# HQuizlet Platform - Phase 2 Execution Plan And Dev Rules

Ngay tao: 2026-08-30

Vai tro tai lieu: PM/Tech Lead gate cho Phase 2.

## 1. Muc Tieu Phase 2

Phase 2 tap trung bien Sprint 1 thanh mot san pham hoc tap co flow that, giam loi integration va chuan bi nen tang de clone tiep cac tinh nang chinh cua Quizlet.

Ket qua mong muon:

1. Auth, Study Set, Flashcard, Learning modes chay end-to-end tren PostgreSQL.
2. Docker Compose chay on dinh tu may moi clone ve.
3. Frontend khong con placeholder hoac mock trong flow chinh.
4. API contract khop backend va frontend.
5. Moi dev co checklist bat buoc truoc khi push.
6. Moi branch dev co ownership ro, tranh conflict va tranh push code chua merge het.

## 2. Bai Hoc Tu Sprint 1

| Loi da gap | Nguyen nhan | Quy dinh moi trong Phase 2 |
| --- | --- | --- |
| Docker build loi do thieu `go.sum` | Service import dependency moi nhung khong cap nhat checksum | Moi backend PR phai chay `go mod tidy` va verify Docker build |
| Frontend van dung mock du da co backend | Khong co gate cam mock trong flow chinh | Flow auth/study tren `main` khong duoc dung mock, tru khi file nam trong `lib/mock` va chi dung cho demo/test |
| Learning modes da code nhung `main` van placeholder | Branch Dev 4 chua merge kip, Dev 3/Dev 4 co diem cham nhau o `StudyDetail` | Moi UI placeholder phai co issue/task va han merge ro; truoc phase gate phai search placeholder |
| OpenAPI lech JSON backend | Contract khong duoc cap nhat cung code | Bat ky thay doi response/request phai sua OpenAPI trong cung PR hoac PR lien ket |
| Branch dev bi behind/ahead nhieu | Dev lam lau khong rebase | Moi ngay rebase/pull main truoc khi code, PR lon hon 500 dong phai chia nho |
| Push len main lam dev khac bi lech | Thieu quy trinh sau khi main thay doi | Sau moi merge vao main, owner branch phai rebase branch cua minh trong ngay |

## 3. Phase 2 Scope

### In Scope

1. Hoan thien auth session va user state.
2. Hoan thien CRUD study set va flashcard tren UI that.
3. Hoan thien 4 learning modes dung data that.
4. Them progress local/backend toi thieu cho learning.
5. Them folder core neu flow study on dinh.
6. Them test/checklist build cho frontend, backend, Docker.
7. Chuan hoa README dev setup va troubleshooting.

### Out Of Scope Tam Thoi

1. Payment production.
2. Mobile app.
3. OAuth Google/GitHub production.
4. Rust optimization neu logic Go/React chua on dinh.
5. Realtime/live quiz production.
6. Deployment cloud production.

## 4. Team 5 Dev Va Ownership

| Dev | Role | Ownership chinh | Ket qua Phase 2 |
| --- | --- | --- | --- |
| Dev 1 | Backend Go Dev - Auth & User | `services/auth/**` | Auth session on dinh, user profile base, permission contract |
| Dev 2 | Backend Go Dev - Study & Folder | `services/study/**` | Study set/flashcard/folder API chay that va co ownership |
| Dev 3 | Frontend React Dev - Core Product | `apps/web/src/features/auth/**`, `dashboard/**`, `study-sets/**` | UI auth/dashboard/create/edit/detail noi API that |
| Dev 4 | Frontend React Dev - Learning Experience | `apps/web/src/features/learning/**` | Flashcards/Learn/Test/Match dung data that, UI hoan thien |
| Dev 5 | Fullstack/Integration Dev | `services/gateway/**`, `infra/**`, `packages/api-contracts/**`, `apps/web/src/lib/api/**`, `docs/**` | Gateway, Docker, OpenAPI, API client, integration gate |

## 5. Skill Bat Buoc Theo Role

### Dev 1 - Backend Go Auth

Can nam:

1. Go `net/http`, context, middleware.
2. PostgreSQL query voi `pgx`.
3. Password hashing bang bcrypt.
4. Session/token design.
5. Migration an toan, idempotent.
6. JSON error format thong nhat.

Khong duoc:

1. Viet SQL trong handler.
2. Luu plain password.
3. Thay doi response auth ma khong bao Dev 5.
4. Push dependency moi khi `go.sum` chua cap nhat.

### Dev 2 - Backend Go Study

Can nam:

1. Go layered architecture: handler -> service -> repository.
2. PostgreSQL transaction co ban.
3. Ownership check bang `userId`.
4. Pagination/search/sort co ban.
5. Migration them cot/bang khong pha DB cu.
6. API validation.

Khong duoc:

1. Cho phep user sua/xoa study set cua user khac.
2. Tra field JSON khac OpenAPI.
3. Tao endpoint moi ma gateway/OpenAPI chua biet.
4. Them dependency nhung khong chay `go mod tidy`.

### Dev 3 - Frontend React Core

Can nam:

1. React component state va form state.
2. TypeScript strict typing.
3. API client pattern trong `apps/web/src/lib/api`.
4. Auth protected flow.
5. Loading/error/empty states.
6. Responsive layout.

Khong duoc:

1. Goi backend truc tiep bo qua gateway.
2. Dung mock trong flow chinh tren `main`.
3. Hard-code token/userId.
4. Sua learning mode logic cua Dev 4 neu chua thong nhat.

### Dev 4 - Frontend React Learning

Can nam:

1. React state machine co ban cho mode hoc.
2. Shuffle, scoring, answer checking local.
3. Component isolation.
4. Accessibility keyboard/click state co ban.
5. Empty state khi set it hon 2 cards.
6. Responsive mobile cho learning.

Khong duoc:

1. De placeholder sau khi task da merge.
2. Tao type rieng lech voi `apps/web/src/types`.
3. Sua `StudyDetail` qua nhieu neu co the expose component entrypoint rieng.
4. Phu thuoc backend progress API neu Phase 2 chua chot contract.

### Dev 5 - Fullstack/Integration

Can nam:

1. Docker Compose multi-service.
2. Gateway proxy/CORS.
3. OpenAPI schema discipline.
4. Frontend API client abstraction.
5. Git merge/rebase va conflict resolution.
6. Integration checklist va release gate.

Khong duoc:

1. Merge PR khi Docker/build/test chua co bang chung.
2. Sua business logic cua Dev 1/2/3/4 ma khong tag owner.
3. De OpenAPI lech voi code.
4. Xoa branch/changset cua dev khac khi chua xac nhan.

## 6. Branch Strategy Phase 2

| Dev | Branch chinh | Merge vao |
| --- | --- | --- |
| Dev 1 | `feature/phase2-auth-user` | `main` qua PR |
| Dev 2 | `feature/phase2-study-folder` | `main` qua PR |
| Dev 3 | `feature/phase2-web-core` | `main` qua PR |
| Dev 4 | `feature/phase2-learning` | `main` qua PR |
| Dev 5 | `feature/phase2-integration` | `main` qua PR |

Quy dinh:

1. Khong push thang vao `main` tru khi PM/Tech Lead cho phep hotfix.
2. Moi branch phai rebase `origin/main` dau moi ngay.
3. Moi PR phai nho hon 500 dong diff neu co the.
4. PR co migration/API contract phai tag Dev 5.
5. PR frontend dung API moi phai tag backend owner lien quan.
6. Sau khi PR merge, owner branch xoa branch cu hoac tao branch phase moi.

## 7. Checklist Bat Buoc Truoc Khi Push

### Tat Ca Dev

Chay truoc khi push:

```powershell
git pull --rebase origin main
git status
```

Bat buoc kiem tra:

1. Khong commit file tam, file secret, file lock o sai vi tri.
2. Khong con conflict marker:

```powershell
git grep "<<<<<<<\\|=======\\|>>>>>>>"
```

3. Commit message co role/task:

```text
feat(auth): add session refresh [Dev1]
fix(docker): tidy modules before build [Dev5]
```

### Backend Go Dev

Trong service minh sua:

```powershell
go mod tidy
go test ./...
go build ./...
```

Neu sua Docker/backend chung:

```powershell
docker compose -f infra/docker/docker-compose.yml build auth study gateway quiz
```

Bat buoc:

1. `go.sum` phai duoc commit neu thay doi dependency.
2. Migration phai chay lai nhieu lan khong loi.
3. Handler khong query DB truc tiep.
4. Response JSON phai khop OpenAPI.

### Frontend React Dev

Trong `apps/web`:

```powershell
npm install
npm run build
```

Bat buoc:

1. Khong dung mock cho flow chinh tren `main`.
2. UI co loading/error/empty state.
3. TypeScript khong loi.
4. Khong hard-code API URL ngoai `lib/api`.
5. Responsive desktop va mobile co ban.

### Integration Dev

Tu repo root:

```powershell
docker compose -f infra/docker/docker-compose.yml up --build
```

Kiem tra:

```powershell
curl http://localhost:8080/healthz
curl http://localhost:8080/healthz/services
```

Flow gate:

1. Register user.
2. Login user.
3. Reload frontend van con session.
4. Tao study set.
5. Them flashcard.
6. Mo study detail.
7. Chay Flashcards/Learn/Test/Match.
8. Logout.

## 8. Definition Of Done Phase 2

Mot task chi duoc xem la Done khi:

1. Code nam dung ownership.
2. Build/test lien quan pass.
3. API change da cap nhat OpenAPI.
4. Docker Compose khong bi fail neu task cham backend/infra.
5. UI khong con placeholder neu task la user-facing.
6. Co test manual note trong PR.
7. Branch da rebase voi `main` moi nhat truoc merge.

Phase 2 chi duoc qua gate khi:

| Hang muc | Dieu kien pass |
| --- | --- |
| Docker | `docker compose up --build` pass tu repo root |
| Health | `/healthz/services` tra ve tat ca service `ok` |
| Auth | Register/login/me/logout chay qua gateway |
| Study | CRUD study set/flashcard chay qua UI va DB |
| Learning | 4 mode hoc dung data that, khong placeholder |
| Contract | OpenAPI khop backend va frontend types |
| Frontend | `npm run build` pass |
| Backend | `go test ./...` pass trong service co code Phase 2 |

## 9. Cong Viec Phase 2 Theo Tung Dev

### Dev 1 - Auth & User

| Ma task | Viec | Output | Co the lam song song | Phu thuoc |
| --- | --- | --- | --- | --- |
| P2-AUTH-01 | Chuan hoa session/token lifecycle | Login/me/logout/refresh ro han | Co | Khong |
| P2-AUTH-02 | Them middleware doc user tu token | User identity dung chung cho gateway/study | Co | Dev 5 contract header |
| P2-AUTH-03 | Them user profile base | Update name/image toi thieu | Co | OpenAPI Dev 5 |
| P2-AUTH-04 | Chuan hoa error auth | 401/403/409/422 thong nhat | Co | Dev 5 error schema |
| P2-AUTH-05 | Auth test cases | Test register/login/me/logout | Co | Khong |

Fallback khi cho:

1. Viet curl collection auth.
2. Them validation email/password.
3. Viet migration rollback note.
4. Review OpenAPI auth examples.

### Dev 2 - Study & Folder

| Ma task | Viec | Output | Co the lam song song | Phu thuoc |
| --- | --- | --- | --- | --- |
| P2-STUDY-01 | Ownership enforcement | User chi thay/sua data cua minh | Co | Dev 1 token identity |
| P2-STUDY-02 | Bulk save flashcards | Create/update/delete nhieu cards khi save editor | Co | Dev 5 OpenAPI |
| P2-STUDY-03 | Search/filter study sets | Search by title, sort updated | Co | Khong |
| P2-STUDY-04 | Folder core schema/API | CRUD folder, add/remove set | Co dieu kien | Study set stable |
| P2-STUDY-05 | Study service tests | Repository/service tests | Co | Khong |

Fallback khi cho:

1. Pagination design.
2. Seed data dev.
3. Validate min cards cho learning.
4. SQL index review.

### Dev 3 - Web Core Product

| Ma task | Viec | Output | Co the lam song song | Phu thuoc |
| --- | --- | --- | --- | --- |
| P2-WEB-01 | Auth persistence | Refresh page van giu user | Co | Dev 1 `/me` stable |
| P2-WEB-02 | Study set editor API thật | Save title/description/cards vao DB | Co | Dev 2 bulk/card API |
| P2-WEB-03 | Dashboard polish | Search, sort, empty/loading/error | Co | Dev 2 list API |
| P2-WEB-04 | Detail page polish | Header/actions/card list dung data that | Co | Dev 2 detail API |
| P2-WEB-05 | Shared UI components | Button/Input/Alert/EmptyState | Co | Khong |

Fallback khi cho:

1. Responsive editor.
2. Form validation.
3. Error mapping user-friendly.
4. Skeleton loading.

### Dev 4 - Learning Experience

| Ma task | Viec | Output | Co the lam song song | Phu thuoc |
| --- | --- | --- | --- | --- |
| P2-LEARN-01 | Flashcards mode polish | Flip, next/prev, shuffle, starred filter | Co | Study detail data |
| P2-LEARN-02 | Learn mode scoring | Local score, retry wrong answers | Co | Khong |
| P2-LEARN-03 | Test mode generation | Question list tu flashcards | Co | Khong |
| P2-LEARN-04 | Match mode completion | Pair matching, timer local, result state | Co | Khong |
| P2-LEARN-05 | Learning progress contract draft | De xuat API luu progress cho Phase 3 | Co | Dev 5 review |

Fallback khi cho:

1. Empty state khi < 2 cards.
2. Keyboard navigation.
3. Mobile layout.
4. Component-level manual test notes.

### Dev 5 - Integration & Contract

| Ma task | Viec | Output | Co the lam song song | Phu thuoc |
| --- | --- | --- | --- | --- |
| P2-INT-01 | OpenAPI Phase 2 update | Auth/study/folder/learning progress draft | Co | Input Dev 1/2/4 |
| P2-INT-02 | API client typed wrapper | Frontend goi API thong nhat | Co | OpenAPI |
| P2-INT-03 | Docker build gate | Compose build/run docs va fix loi | Co | Khong |
| P2-INT-04 | Integration checklist script | Manual/curl checklist de dev chay | Co | Dev 1/2 endpoints |
| P2-INT-05 | Phase gate review | Review branch, conflict, build, docs | Co | Tat ca PR |

Fallback khi cho:

1. README troubleshooting.
2. PR template.
3. `.env.example` review.
4. Healthcheck/log format review.

## 10. Lich Lam Viec De Giam Cho Nhau

### Ngay 1 - Contract Va Stability

| Owner | Viec uu tien | Ly do |
| --- | --- | --- |
| Dev 5 | Cap nhat OpenAPI Phase 2 draft | De backend/frontend bam chung |
| Dev 1 | Chot auth token/me/logout behavior | Chan bug session |
| Dev 2 | Chot study ownership va bulk card behavior | Chan bug data user |
| Dev 3 | Chuan hoa API client usage trong UI | Bo cach goi API le |
| Dev 4 | Polish learning local logic | Khong can cho backend |

### Ngay 2-3 - Build Feature Song Song

| Owner | Viec uu tien |
| --- | --- |
| Dev 1 | Auth tests, profile base |
| Dev 2 | Bulk flashcards, search/filter |
| Dev 3 | Editor save API, dashboard polish |
| Dev 4 | 4 learning modes polish |
| Dev 5 | Docker compose gate, API client, docs |

### Ngay 4 - Integration

| Owner | Viec uu tien |
| --- | --- |
| Dev 1 | Fix auth bugs tu integration |
| Dev 2 | Fix study bugs tu integration |
| Dev 3 | Fix UI/API mapping |
| Dev 4 | Fix learning edge cases |
| Dev 5 | Merge/rebase review, run full checklist |

### Ngay 5 - Phase Gate

| Owner | Viec uu tien |
| --- | --- |
| PM/Tech Lead | Review final |
| Dev 5 | Docker + OpenAPI + checklist |
| Tat ca dev | Fix blocker trong ownership |

## 11. Blocking Rules

| Neu bi block | Lam ngay | Bao ai |
| --- | --- | --- |
| Cho OpenAPI | Dung draft hien tai, viet TODO trong PR | Dev 5 |
| Cho backend API | Dung API adapter co mock fallback chi trong dev/test | Dev 3/Dev 5 |
| Cho auth identity | Lam logic CRUD local voi `userId` parameter noi bo | Dev 1/Dev 2 |
| Conflict file chung | Dung merge, tag owner file | Owner file + Dev 5 |
| Docker fail | Ghi command/log vao PR, khong merge | Dev 5 |
| Build frontend fail | Khong push/merge, fix TypeScript truoc | Dev 3/Dev 4 |
| Dependency moi | Chay tidy/install va commit lock/checksum dung vi tri | Dev owner |

Quy tac 30 phut:

Neu bi block qua 30 phut, dev phai:

1. Comment vao PR/task dang bi block.
2. Tag owner lien quan.
3. Chuyen sang fallback task cua role minh.
4. Khong sua vung code cua nguoi khac de tu unblock neu chua thong nhat.

## 12. File Ownership Matrix

| Path | Owner | Reviewer bat buoc |
| --- | --- | --- |
| `services/auth/**` | Dev 1 | Dev 1, Dev 5 neu doi API |
| `services/study/**` | Dev 2 | Dev 2, Dev 5 neu doi API |
| `apps/web/src/features/auth/**` | Dev 3 | Dev 3 |
| `apps/web/src/features/dashboard/**` | Dev 3 | Dev 3 |
| `apps/web/src/features/study-sets/**` | Dev 3 | Dev 3, Dev 4 neu cham learning entry |
| `apps/web/src/features/learning/**` | Dev 4 | Dev 4, Dev 3 neu cham detail page |
| `apps/web/src/lib/api/**` | Dev 5 | Dev 5, Dev 3 |
| `apps/web/src/types/**` | Dev 5 | Dev 3, Dev 4 |
| `services/gateway/**` | Dev 5 | Dev 5 |
| `infra/**` | Dev 5 | Dev 5 |
| `packages/api-contracts/**` | Dev 5 | Dev 1/2/3/4 lien quan endpoint |
| `docs/**` | PM/Dev 5 | PM/Dev 5 |

## 13. PR Template Bat Buoc

Moi PR phai co:

```markdown
## Summary
- 

## Scope
- Owner:
- Files touched:

## Test Evidence
- [ ] npm run build
- [ ] go test ./...
- [ ] go build ./...
- [ ] docker compose up --build
- [ ] Manual flow tested

## API/DB Changes
- OpenAPI updated: Yes/No
- Migration added/changed: Yes/No
- Env changed: Yes/No

## Risks
- 

## Screenshots / Logs
- 
```

## 14. Phase 2 Review Checklist Cho PM

Chay truoc khi cho qua phase:

```powershell
git fetch origin --prune
git log --oneline -10
git branch -r
npm --prefix apps/web run build
docker compose -f infra/docker/docker-compose.yml up --build
```

Search loi hay lap:

```powershell
git grep "Dev 4 đang phát triển"
git grep "mockLogin\\|mockRegister\\|MOCK_SETS"
git grep "user_id\\|study_set_id\\|created_at\\|updated_at" packages/api-contracts/openapi.yaml
git grep "<<<<<<<\\|=======\\|>>>>>>>"
```

Pass khi:

1. Khong co placeholder user-facing.
2. Khong co mock trong production flow.
3. OpenAPI khong lech field JSON.
4. Docker build pass tren may dev.
5. Tat ca service health `ok`.
6. Flow user chinh pass.

## 15. Ket Luan Dieu Phoi

Trong Phase 2, 5 dev van co the lam song song. Diem can dong bo nhieu nhat la contract API va file giao nhau giua Dev 3/Dev 4.

Thu tu uu tien:

1. Dev 5 chot contract va gate build.
2. Dev 1/2 lam backend theo contract.
3. Dev 3 noi UI core voi API that.
4. Dev 4 polish learning tren data that, khong de placeholder.
5. PM/Dev 5 chi cho qua phase khi checklist pass.

