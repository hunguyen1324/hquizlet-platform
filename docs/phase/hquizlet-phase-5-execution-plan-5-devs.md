# HQuizlet Platform - Phase 5 Execution Plan (5 Developers)

## 1. Muc Tieu Phase 5

Phase 5 hoan thien tinh nang Folder de user co the to chuc study sets theo
thu muc, dong thoi ra quyet dinh ky thuat ve Rust runtime integration dua tren
evidence cua Phase 4 benchmark.

Ket qua cuoi phase:

- User tao, xem, sua va xoa folder tren du lieu that.
- User them study set vao folder va go study set khoi folder.
- Xoa folder khong xoa study set goc.
- Folder va study set deu enforce ownership; User B khong doc/sua/xoa du lieu
  cua User A.
- Gateway route folder API voi auth, request ID va spoofed-header stripping.
- Frontend co Folder list, Folder detail, create/edit/delete va add/remove
  study set flow.
- PostgreSQL migrations chay duoc tu database sach va co rollback.
- API contract, backend tests, frontend tests, Docker fresh-volume E2E va docs
  deu xanh truoc khi danh dau GO.
- Rust FFI/WASM/service runtime chi duoc de xuat neu Phase 4 benchmark chung
  minh loi ich ro rang; neu khong, Rust tiep tuc la spec/golden crate.

## 2. Pham Vi

### 2.1 In scope

- Folder domain trong Study service.
- PostgreSQL schema cho folders va folder-study-set relationship.
- Folder API qua Gateway.
- Frontend folder screens va reusable add-to-folder UI.
- Backend, frontend, contract va E2E tests.
- Phase 5 release gate report.
- ADR revisit cho Rust quiz engine runtime integration.

### 2.2 Out of scope

- Folder sharing/public collaboration.
- Nested folders.
- Class folder/library folder.
- Bulk import/export folder.
- Payment/entitlement quanh folder.
- Bat buoc dua Rust vao production runtime. Day la quyet dinh co dieu kien,
  khong phai deliverable mac dinh cua Folder feature.

## 3. Quyet Dinh Kien Truc

### 3.1 Service ownership

Phase 5 nen dat Folder trong `services/study`, vi folder la cach to chuc
study sets va phu thuoc ownership cua study set. Chua can tao `services/folder`
rieng trong phase nay.

De xuat cau truc:

```text
services/study/
  internal/model/folder.go
  internal/repository/folder_repository.go
  internal/service/folder_service.go
  internal/http/folder_handler.go
  migrations/
```

### 3.2 Luong du lieu chuan

1. Frontend goi Gateway voi token nguoi dung.
2. Gateway xac thuc, strip identity header gia va inject `X-User-ID`.
3. Gateway forward folder request sang Study service.
4. Study service validate payload, check folder ownership va study set
   ownership.
5. Repository doc/ghi PostgreSQL.
6. Study service tra response theo OpenAPI contract.
7. Frontend render dung response va khong suy doan ownership o client.

### 3.3 Rust runtime decision

ADR-003 hien dang de Rust `quiz-core` o trang thai pure crate. Phase 5 chi
revisit sau khi co:

- Golden vectors da freeze.
- Benchmark 10, 100, 1.000 va 10.000 cards.
- So sanh Rust voi Go port bang input/seed tuong duong.
- CI co the build/test binding mot cach on dinh.
- Loi ich latency/performance du lon de chap nhan them runtime boundary.

Neu evidence khong du manh, tiep tuc giu Go port trong production va dung Rust
lam spec/golden/test tooling.

## 4. Database Schema

### 4.1 `folders`

```sql
CREATE TABLE folders (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Khuyen nghi constraints/indexes:

- `title` khong duoc rong sau khi trim.
- Index `folders_user_id_idx` tren `user_id`.
- Neu `users` table cung DB/schema kha dung, them foreign key
  `folders.user_id -> users.id`.

### 4.2 `folder_to_study_sets`

```sql
CREATE TABLE folder_to_study_sets (
  folder_id BIGINT NOT NULL,
  study_set_id BIGINT NOT NULL,
  added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (folder_id, study_set_id)
);
```

Khuyen nghi constraints/indexes:

- `folder_id` foreign key den `folders(id)` voi `ON DELETE CASCADE`.
- `study_set_id` foreign key den `study_sets(id)` voi behavior phu hop schema
  hien tai.
- Index `folder_to_study_sets_study_set_id_idx` tren `study_set_id`.
- Khong cho duplicate study set trong cung folder bang primary key composite.

### 4.3 Migration requirements

- Migration up/down ro rang.
- Chay duoc tren database sach.
- Chay duoc sau migrations Phase 1-4.
- Rollback khong de lai table/index mo coi.
- Neu co foreign key cross-table, phai verify order migration trong Docker.

## 5. API Contract

### 5.1 Endpoints

| Method | Endpoint | Muc dich |
| --- | --- | --- |
| `GET` | `/v1/folders` | List folders cua current user |
| `POST` | `/v1/folders` | Tao folder |
| `GET` | `/v1/folders/{folderId}` | Chi tiet folder kem study sets |
| `PUT` | `/v1/folders/{folderId}` | Sua folder |
| `DELETE` | `/v1/folders/{folderId}` | Xoa folder |
| `POST` | `/v1/folders/{folderId}/study-sets` | Them study set vao folder |
| `DELETE` | `/v1/folders/{folderId}/study-sets/{studySetId}` | Go study set khoi folder |

### 5.2 Schemas

`FolderSummary`:

```json
{
  "id": 1,
  "title": "English",
  "description": "IELTS vocabulary",
  "studySetCount": 3,
  "createdAt": "2026-09-01T10:00:00Z",
  "updatedAt": "2026-09-01T10:00:00Z"
}
```

`FolderDetail`:

```json
{
  "id": 1,
  "title": "English",
  "description": "IELTS vocabulary",
  "studySetCount": 2,
  "studySets": [
    {
      "id": 10,
      "title": "Unit 1",
      "description": "Basic vocabulary",
      "flashcardCount": 20,
      "createdAt": "2026-09-01T10:00:00Z",
      "updatedAt": "2026-09-01T10:00:00Z"
    }
  ],
  "createdAt": "2026-09-01T10:00:00Z",
  "updatedAt": "2026-09-01T10:00:00Z"
}
```

`CreateFolderRequest`:

```json
{
  "title": "English",
  "description": "IELTS vocabulary"
}
```

`UpdateFolderRequest`:

```json
{
  "title": "English - IELTS",
  "description": "Vocabulary for IELTS"
}
```

`AddStudySetToFolderRequest`:

```json
{
  "studySetId": 10
}
```

### 5.3 Error contract

Tat ca loi dung envelope thong nhat:

```json
{
  "code": "VALIDATION_ERROR",
  "message": "Invalid request",
  "requestId": "req_123",
  "details": {
    "field": "title"
  }
}
```

Bat buoc cover:

- `401` khi thieu/invalid auth.
- `403` hoac `404` khi user truy cap folder/study set khong thuoc minh.
- `404` folder khong ton tai.
- `404` study set khong ton tai.
- `409` duplicate relation neu contract chon explicit conflict.
- `422` payload sai, title rong, path id invalid.
- `500` internal error nhung khong leak `err.Error()` tho.

## 6. Backend Implementation Plan

### 6.1 Repository

Can implement:

- `ListFoldersByUser(ctx, userID)`
- `CreateFolder(ctx, userID, input)`
- `GetFolderByID(ctx, userID, folderID)`
- `UpdateFolder(ctx, userID, folderID, input)`
- `DeleteFolder(ctx, userID, folderID)`
- `ListFolderStudySets(ctx, userID, folderID)`
- `AddStudySetToFolder(ctx, userID, folderID, studySetID)`
- `RemoveStudySetFromFolder(ctx, userID, folderID, studySetID)`
- `StudySetBelongsToUser(ctx, userID, studySetID)`

Repository khong duoc quyet dinh HTTP status; chi tra typed domain errors.

### 6.2 Service

Service can enforce:

- `title` trim va khong rong.
- `description` optional, trim neu co.
- folder ownership truoc update/delete/add/remove.
- study set ownership truoc add.
- duplicate add xu ly theo contract.
- remove relation khong anh huong study set goc.

Typed errors de mapping:

- `ErrUnauthorized`
- `ErrForbidden`
- `ErrNotFound`
- `ErrValidation`
- `ErrConflict`
- `ErrInternal`

### 6.3 HTTP handlers

Handlers can:

- Lay `X-User-ID` da duoc Gateway inject.
- Parse path params an toan.
- Decode JSON voi size limit neu pattern repo da co.
- Tra response status dung contract.
- Tra error envelope co `requestId`.
- Khong de SQL/DB query truc tiep trong handler.

### 6.4 Gateway

Can them route:

- `/v1/folders`
- `/v1/folders/*`

Gateway requirements:

- Auth bat buoc cho moi folder route.
- Strip spoofed `X-User-ID` tu client.
- Inject authenticated `X-User-ID`.
- Forward hoac tao `X-Request-ID`.
- Map upstream unavailable thanh error envelope.

## 7. Frontend Implementation Plan

### 7.1 API client

Them methods vao frontend API client:

- `listFolders()`
- `createFolder(input)`
- `getFolder(folderId)`
- `updateFolder(folderId, input)`
- `deleteFolder(folderId)`
- `addStudySetToFolder(folderId, studySetId)`
- `removeStudySetFromFolder(folderId, studySetId)`

Types:

- `FolderSummary`
- `FolderDetail`
- `FolderStudySetSummary`
- `CreateFolderInput`
- `UpdateFolderInput`

### 7.2 Routes

De xuat routes:

```text
/folders
/folders/new
/folders/:folderId
/folders/:folderId/edit
```

Neu app hien co route convention khac, uu tien convention hien tai.

### 7.3 Folder list page

Can co:

- loading state.
- empty state.
- error state co retry.
- list folder cua current user.
- study set count.
- create folder action.
- click vao folder detail.

### 7.4 Create/Edit folder

Can co:

- title input.
- description textarea optional.
- client-side validation nhe.
- server validation display.
- submit loading.
- cancel/back.
- redirect sau success.

### 7.5 Folder detail

Can co:

- title va description.
- study sets trong folder.
- empty state neu folder chua co set.
- add study set button.
- remove study set action.
- edit folder action.
- delete folder action.

### 7.6 Add study set dialog

Can co:

- fetch study sets cua user.
- search/filter neu UI pattern co san.
- disable hoac mark study sets da nam trong folder.
- submit selected study set.
- handle duplicate/conflict.
- refresh folder detail sau add.

### 7.7 Delete/remove behavior

- Delete folder phai co confirmation.
- Delete folder khong xoa study sets goc.
- Remove study set khoi folder khong xoa study set goc.
- UI phai cap nhat optimistic hoac refetch ro rang, khong de stale count.

## 8. Testing Plan

### 8.1 Contract tests

- OpenAPI lint.
- Folder examples validate duoc.
- Frontend type assumptions khop OpenAPI.
- Backend response fields khop examples.

### 8.2 Backend unit/integration tests

Bat buoc:

- create folder success.
- create folder empty title -> `422`.
- list chi tra folder cua current user.
- get own folder success.
- get other user's folder blocked.
- update own folder success.
- delete own folder success.
- add own study set success.
- add duplicate study set deterministic.
- add other user's study set blocked.
- remove relation success.
- delete folder leaves study sets intact.
- invalid path id -> `422`.
- repository handles DB constraint errors as typed errors.

### 8.3 Gateway tests

- no token -> `401`.
- invalid token -> `401`.
- spoofed `X-User-ID` ignored.
- `X-Request-ID` preserved/generated.
- Study upstream down -> standard error envelope.

### 8.4 Frontend tests

Bat buoc:

- folder list loading.
- folder list empty.
- folder list error + retry.
- folder list success.
- create folder success.
- create folder validation error.
- edit folder success.
- delete folder success.
- folder detail shows study sets.
- add study set success.
- duplicate add handling.
- remove study set success.
- unauthorized/session expired behavior.

### 8.5 E2E tests

Tao script:

```text
infra/scripts/phase5-e2e.sh
```

Flow API E2E:

1. Start stack tu fresh PostgreSQL volume.
2. Register User A.
3. Register User B.
4. User A tao 2 study sets.
5. User A tao folder.
6. User A add 2 study sets vao folder.
7. User A get folder detail va thay 2 study sets.
8. User A remove 1 study set.
9. User A get folder detail va thay con 1 study set.
10. User B bi chan khi get/update/delete folder cua User A.
11. User B bi chan khi add study set cua User A vao folder cua B.
12. User A delete folder.
13. Verify study sets goc van ton tai.
14. Verify list folders khong con folder da xoa.

Browser E2E neu Playwright da co:

- login.
- open `/folders`.
- create folder.
- add study set.
- open detail.
- remove study set.
- delete folder.
- verify UI states.

## 9. CI Gate

Them hoac verify CI jobs:

- OpenAPI lint.
- Contract examples validation.
- Go tests cho Study/Gateway.
- Go build cho services lien quan.
- Frontend tests.
- Frontend production build.
- Migration test tu DB sach.
- `phase5-e2e.sh` syntax check.
- Docker full E2E opt-in hoac nightly.
- `git diff --check`.

Phase 5 GO gate bat buoc:

- Contract PASS.
- Migrations PASS tren database sach.
- Backend tests PASS.
- Frontend tests PASS.
- Gateway security tests PASS.
- Docker fresh-volume E2E PASS.
- Docs va curl examples cap nhat.
- Phase 5 gate report co command, commit SHA va evidence.

## 10. Rust Runtime ADR Track

### 10.1 Inputs can co

- Phase 4 Rust benchmark report.
- Phase 4 Go benchmark report.
- Golden vector status.
- CI build time va complexity neu co prototype.
- Runtime deployment impact.

### 10.2 Options

#### Option A: Keep Go runtime, Rust spec only

Chon neu:

- Go p95 da dat muc tieu.
- Rust speedup khong dang ke trong request path.
- FFI/WASM lam CI/deploy phuc tap hon loi ich.

#### Option B: Go FFI/CGO binding

Chi chon neu:

- Rust nhanh hon ro ret voi deck lon.
- Container build co toolchain on dinh.
- CI build duoc deterministically.
- Co rollback sang Go engine.

#### Option C: Rust sidecar/service

Chi chon neu:

- Can runtime tach rieng vi load cao.
- Network hop cost nho hon loi ich xu ly.
- Observability/retry/circuit breaker duoc thiet ke ro.

#### Option D: WASM

Chi chon neu:

- Co nhu cau chay deterministic engine o browser/offline.
- Bundle size va build pipeline chap nhan duoc.
- Security va versioning contract duoc kiem soat.

### 10.3 Deliverable

- ADR Phase 5 cap nhat quyet dinh.
- Neu chon runtime integration, tao Phase 5.x hoac Phase 6 tech task rieng.
- Khong tron prototype FFI/WASM vao PR Folder production.

## 11. Phan Cong 5 Developer

## Dev 1 - Contract, Gateway va Release Owner

Vai tro: khoa API contract, route Gateway va quan ly gate release.

Cong viec:

- `[P5-CON-01]` Them OpenAPI cho folder endpoints.
- `[P5-CON-02]` Dinh nghia schemas folder, folder detail, add/remove relation va
  error envelope.
- `[P5-CON-03]` Them contract examples cho list/create/detail/update/delete/add/remove.
- `[P5-GW-01]` Them Gateway routes den Study service.
- `[P5-GW-02]` Them auth, spoofed-header stripping va request ID tests.
- `[P5-CI-01]` Them contract lint/examples validation vao CI.
- `[P5-DOC-01]` Cap nhat docs, curl examples va release gate report.

Definition of Done:

- OpenAPI lint PASS.
- Gateway tests PASS.
- Contract khong con field ngam giua frontend/backend.

## Dev 2 - Database va Repository Owner

Vai tro: so huu schema, migrations va repository correctness.

Cong viec:

- `[P5-DB-01]` Tao migration `folders`.
- `[P5-DB-02]` Tao migration `folder_to_study_sets`.
- `[P5-DB-03]` Them indexes/constraints can thiet.
- `[P5-DB-04]` Implement folder repository.
- `[P5-DB-05]` Test migration up/down/fresh database.
- `[P5-DB-06]` Test duplicate relation va delete cascade.

Definition of Done:

- Migration chay duoc tu DB sach.
- Repository tests PASS.
- Delete folder khong xoa study set goc.

## Dev 3 - Backend Service va Handler Owner

Vai tro: implement folder business logic, handlers va ownership enforcement.

Cong viec:

- `[P5-GO-01]` Implement folder service layer.
- `[P5-GO-02]` Implement HTTP handlers.
- `[P5-GO-03]` Validate payload/path params.
- `[P5-GO-04]` Enforce ownership folder va study set.
- `[P5-GO-05]` Map typed errors sang standard envelope.
- `[P5-GO-06]` Them unit/integration tests.
- `[P5-GO-07]` Them logs co request ID neu pattern service da co.

Definition of Done:

- Study service tests PASS.
- User B khong truy cap du lieu User A.
- Handlers khong query DB truc tiep.

## Dev 4 - Frontend Folder Owner

Vai tro: xay UX folder end-to-end.

Cong viec:

- `[P5-FE-API-01]` Them folder API client va TypeScript types.
- `[P5-FE-LIST-01]` Build Folder list page.
- `[P5-FE-FORM-01]` Build create/edit folder form.
- `[P5-FE-DETAIL-01]` Build Folder detail page.
- `[P5-FE-ADD-01]` Build Add study set dialog.
- `[P5-FE-REMOVE-01]` Build remove study set action.
- `[P5-FE-DELETE-01]` Build delete folder confirmation.
- `[P5-FE-TEST-01]` Them frontend tests cho loading/empty/error/success.

Definition of Done:

- Frontend test va build PASS.
- UI co loading/error/empty states.
- Khong co production mock/no-op.

## Dev 5 - E2E, QA va Rust ADR Owner

Vai tro: chung minh Phase 5 chay that va chot ADR Rust runtime.

Cong viec:

- `[P5-E2E-01]` Tao `infra/scripts/phase5-e2e.sh`.
- `[P5-E2E-02]` Test full folder flow tren Docker fresh-volume.
- `[P5-E2E-03]` Test User A/B ownership va spoofed identity.
- `[P5-E2E-04]` Verify delete folder khong xoa study set.
- `[P5-QA-01]` Lap Phase 5 release checklist va evidence links.
- `[P5-ADR-01]` Tong hop Phase 4 benchmark va golden evidence.
- `[P5-ADR-02]` Cap nhat ADR ve Rust runtime integration.

Definition of Done:

- Docker fresh-volume E2E PASS.
- Gate report ghi ro command, commit SHA va evidence.
- Rust runtime decision co ADR, khong dua vao production neu chua du evidence.

## 12. Lich Thuc Hien 4 Tuan

| Tuan | Dev 1 | Dev 2 | Dev 3 | Dev 4 | Dev 5 |
| --- | --- | --- | --- | --- | --- |
| Tuan 1 | Freeze OpenAPI/examples | Migrations/schema | Service/handler skeleton | UX audit/routes | E2E skeleton, ADR inputs |
| Tuan 2 | Gateway routes/tests | Repository impl/tests | Folder service logic | List/create/edit UI | API E2E happy path |
| Tuan 3 | CI contract gate | Migration rollback/fixes | Ownership/error tests | Detail/add/remove/delete UI | Security/error E2E |
| Tuan 4 | Docs/release report | DB hardening | Backend regression fixes | Frontend tests/build | Docker evidence, Rust ADR |

## 13. Dependency va Thu Tu Merge

1. Dev 1 merge contract/examples truoc.
2. Dev 2 merge migrations va repository.
3. Dev 3 merge backend service/handlers sau khi schema on dinh.
4. Dev 1 merge Gateway routes sau khi backend routes co contract.
5. Dev 4 merge frontend API/UI sau khi contract freeze.
6. Dev 5 merge E2E va release report cuoi cung.
7. ADR Rust runtime merge rieng, khong block Folder neu chi la decision doc.

Khong merge song song cac thay doi cung OpenAPI. Moi thay doi contract sau
freeze can migration note va approval cua Dev 1 + cac dev bi anh huong.

## 14. Branch va PR Rules

- Branch: `phase5/dev{n}-{task-id}-{short-name}`.
- Mot PR chi nen giai quyet mot nhom task cung boundary.
- PR bat buoc co:
  - task ID.
  - contract impact.
  - migration impact.
  - test commands.
  - evidence.
  - rollback plan.
- Khong push thang `main`.
- Khong commit token, secret, DB dump co PII hoac screenshot chua thong tin nhay cam.

## 15. Gate Bat Buoc Truoc Phase 6

### Contract gate

- OpenAPI syntax/lint PASS.
- Folder examples validate duoc.
- Frontend/backend dung cung schema va error envelope.

### Database gate

- Migration up/down PASS.
- Fresh-volume PostgreSQL PASS.
- Delete folder khong xoa study sets.
- Duplicate folder-study-set relation duoc xu ly dung contract.

### Security gate

- Tat ca folder endpoints can auth.
- Spoofed `X-User-ID` khong bypass duoc.
- User B khong doc/sua/xoa/add/remove folder/study set cua User A.

### Build/test gate

- Go tests/build PASS cho services lien quan.
- Frontend tests/build PASS.
- Contract tests PASS.
- E2E PASS hoac co evidence manual bat buoc neu CI full Docker chua chay.

### UX gate

- Folder list/detail/create/edit/delete hoat dong tren UI.
- Loading/error/empty states day du.
- Add/remove study set flow ro rang.
- Reload trang van thay du lieu dung.

### ADR gate

- Rust runtime decision duoc cap nhat bang ADR.
- Khong them FFI/WASM/service vao request path neu chua co benchmark evidence va
  rollback path.

## 16. Checklist Tong Hop

- `[ ]` OpenAPI them `/v1/folders`.
- `[ ]` Contract examples cho folder flows.
- `[ ]` Migration `folders`.
- `[ ]` Migration `folder_to_study_sets`.
- `[ ]` Repository folder.
- `[ ]` Service ownership validation.
- `[ ]` HTTP handlers.
- `[ ]` Gateway routes.
- `[ ]` Gateway auth/spoof tests.
- `[ ]` Frontend folder API client.
- `[ ]` Folder list page.
- `[ ]` Folder create/edit form.
- `[ ]` Folder detail page.
- `[ ]` Add study set dialog.
- `[ ]` Remove study set action.
- `[ ]` Delete folder confirmation.
- `[ ]` Backend tests.
- `[ ]` Frontend tests.
- `[ ]` Docker fresh-volume E2E.
- `[ ]` Docs va curl examples.
- `[ ]` Phase 5 gate report.
- `[ ]` Rust runtime ADR update.

## 17. Ket Luan

Phase 5 nen duoc xem la Folder feature end-to-end. Rust runtime integration la
track quyet dinh kien truc dua tren benchmark, khong phai dieu kien bat buoc de
Folder chay production. Phase 5 chi duoc danh dau GO khi Folder co contract,
migrations, backend, frontend, security tests, Docker E2E va release evidence
day du.
