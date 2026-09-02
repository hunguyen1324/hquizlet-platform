# HQuizlet Platform - Phase 7 Execution Plan (5 Developers)

## 1. Muc Tieu Phase 7

Phase 7 xay dung Class domain end-to-end: teacher tao lop, moi member tham
gia bang invite code, gan study sets vao lop, va tat ca user co activity feed
tong hop the hien hanh dong gan nhat tren nen tang.

Ket qua cuoi phase:

- Teacher tao class va nhan invite code ngau nhien.
- User join class bang invite code; owner co the them member thu cong qua
  user ID.
- Class chi ro ba muc phan quyen: `owner`, `teacher`, `student`.
- Owner/teacher them study set vao class va go study set khoi class.
- Xoa class xoa tat ca quan he member/study-set nhung khong xoa study set goc.
- Member khong phan quyen khong sua/xoa class hoac quan ly member khac.
- Gateway route toan bo `/v1/classes/` va `/v1/activity` qua Class service
  voi auth va spoofed-header stripping.
- `GET /v1/activity` tra activity feed tong hop: hanh dong class, progress
  hoc tap tu Study service, va quiz result tu Quiz service; phan trang theo
  cursor.
- `services/class` la service doc lap moi: go.mod rieng, Docker Compose
  service rieng, port 8084, health check rieng.
- NATS JetStream phat domain event cho moi hanh dong quan trong cua Class.
- PostgreSQL luu class, member, class-study-set relation va activity events.
- OpenAPI, migrations, backend/frontend tests, security tests va Docker
  fresh-volume E2E deu xanh truoc khi danh dau GO.

## 2. Baseline Va Dieu Kien Bat Dau

### 2.1 Trang thai repo hien tai

Ket qua phan tich source va cau truc hien tai:

- Phase 6 GO gate: CONDITIONAL GO cho den khi fresh-volume Docker E2E co
  evidence day du. Phase 7 khong duoc bat dau implement production code khi
  Phase 6 fresh-volume gate chua dong.
- Gateway hien co `services/auth` (8081), `services/study` (8082),
  `services/quiz` (8083). Phase 7 them `services/class` (8084).
- Gateway da co pattern `authenticatedProxy` da duoc battle-tested tu Phase 2.
  Phase 7 tai su dung pattern nay cho toan bo Class endpoints.
- Docker Compose da co PostgreSQL, Redis, NATS nhung chua co Class service.
  Redis va NATS trong Phase 7 chi dung cho NATS event; Class service khong
  can Redis rieng trong phase nay.
- OpenAPI hien o version `1.4.0`; Phase 7 tang len `1.5.0`.
- Frontend dang dung state-based navigation; Phase 7 them class routes va
  activity route.
- `services/study` da co pattern layered architecture (config, http, model,
  repository, service, migration, middleware). `services/class` se copy
  pattern nay lam baseline.
- Study service da co progress tracking (Phase 4). Phase 7 se query Study
  service qua internal API de lay study activity cho activity feed.
- Khong co `services/class` nao ton tai. Day la service hoan toan moi.

### 2.2 Prerequisite tu Phase 5 va Phase 6

Truoc khi freeze Phase 7 contract:

- Dong Phase 6 fresh-volume gate: chay `infra/scripts/phase6-e2e.sh` tren
  Docker fresh volume va gan commit SHA + output vao Phase 6 release gate
  report. Phase 7 khong merge production code khi Phase 6 con `CONDITIONAL GO`.
- Xac nhan Study service internal API `/internal/study/study-sets/{id}` co
  the query ownership: Phase 7 can endpoint nay de Class service xac nhan
  study set ton tai va user co quyen gan vao class.
- Xac nhan Auth service internal API `/internal/auth/verify` van on dinh
  (da su dung tu Phase 2).
- Giu toan bo Phase 4 golden tests, Phase 5 folder tests va Phase 6 live
  quiz tests xanh. Khong de Phase 7 tao regression.

## 3. Pham Vi

### 3.1 In scope

- `services/class`: service Go moi voi clean architecture day du.
- PostgreSQL migrations cho `classes`, `class_members`, `class_study_sets`,
  `activity_events`, `class_event_outbox`.
- Invite code: generate bang CSPRNG, unique, khong suy doan duoc.
- Member roles: `owner`, `teacher`, `student` voi permission matrix ro rang.
- Class study set assignment: gan/go study set, verify ownership qua Study
  internal API.
- Activity feed: class events tu Class service, study progress tu Study
  service internal API, merge va tra theo cursor.
- NATS JetStream events versioned cho class domain.
- Gateway routes cho `/v1/classes/` va `/v1/activity`.
- Docker Compose: them `class` service, cap nhat `gateway` env vars va
  `healthz/services` endpoint.
- OpenAPI `1.5.0`: them tat ca Class va Activity schemas/endpoints/examples.
- Frontend: Class list, Class detail, create/edit/delete class, member
  management, activity feed page.
- Backend, frontend, security, integration, E2E tests.
- Phase 7 release gate report.

### 3.2 Out of scope

- Class visibility public/private cho nguoi ngoai (Phase nay la invite-only).
- Real-time class chat hay discussion board.
- Nested classes hoac sub-groups.
- Class assignment/homework tracking.
- Class-scoped live quiz (se tich hop voi Phase 6 Live Quiz o phase sau).
- Phan quyen granular hon ba role (vi du: multiple teachers, co-owner).
- Notification push/email khi co member moi.
- Activity feed realtime (SSE/WebSocket cho feed); Phase 7 la polling/REST.
- Cross-service activity aggregation bang NATS consumer phuc tap; Phase 7
  dung internal HTTP call de fetch study/quiz activity.
- Import class tu CSV/Excel.
- Payment/entitlement quanh class.

## 4. Quyet Dinh Kien Truc

### 4.1 Service moi: services/class

Class la domain doc lap voi lifecycle, schema va business logic rieng. Khong
dat vao Study service vi:

- Class quan ly nguoi dung (member), Study service quan ly noi dung hoc tap.
- Class se phat trien them (assignment, class quiz, analytics) ma khong anh
  huong Study.
- Tach service giup enforce ownership boundary ro rang.

Cau truc de xuat:

```text
services/class/
  cmd/server/main.go
  internal/config/config.go
  internal/http/
    handler.go          # class + member + study-set + activity handlers
    response.go         # writeJSON, writeError helpers
    response_test.go
  internal/middleware/
    middleware.go       # requestID, logging, userID extraction
    middleware_test.go
  internal/migration/
    migration.go        # embed SQL files, up/down runner
    migration_test.go
  internal/model/
    model.go            # Class, ClassMember, ClassStudySet, ActivityEvent
  internal/repository/
    interfaces.go       # ClassRepository, MemberRepository, ActivityRepository
    class.go
    member.go
    class_study_set.go
    activity.go
  internal/service/
    class.go
    class_test.go
    member.go
    member_test.go
    activity.go
    activity_test.go
  internal/events/
    publisher.go        # NATS JetStream publisher
    outbox.go           # outbox poller
  internal/client/
    study_client.go     # internal HTTP client -> Study service
    auth_client.go      # internal HTTP client -> Auth service (neu can)
  migrations/
    001_create_classes.sql
    002_create_class_members.sql
    003_create_class_study_sets.sql
    004_create_activity_events.sql
    005_create_class_event_outbox.sql
  go.mod
```

Handler khong ghi SQL/NATS truc tiep. Handler goi service; service dieu phoi
repository, internal client va event publisher. Repository la noi duy nhat
query PostgreSQL.

### 4.2 Ownership va trust boundary

```text
[Frontend] -> [Gateway] -> [Class service]
                  |
                  +-- verifies Bearer token voi Auth service
                  +-- strips X-User-ID tu client
                  +-- injects X-User-ID tu verified identity
                  +-- forwards X-Request-ID
```

Class service nhan `X-User-ID` (integer string) tu Gateway; khong bao gio tin
`X-User-ID` do client gui. Class service tu xac minh ownership o service
layer, khong phu thuoc Gateway enforce ownership.

```text
[Class service] -> [Study service] /internal/study/study-sets/{id}
                                   # verify set ton tai + owner
```

Internal call Class -> Study dung `X-Internal-Token` shared secret (env var)
hoac service mesh trong tuong lai. Phase 7 dung simple shared secret de
khong lam phuc tap deploy; doi sang mTLS o Phase sau.

### 4.3 Member roles va permission matrix

| Hanh dong | owner | teacher | student |
| --- | --- | --- | --- |
| Xem class detail/members/study-sets | Y | Y | Y |
| Sua class (name, description) | Y | N | N |
| Xoa class | Y | N | N |
| Them member (manual) | Y | Y | N |
| Xoa member | Y | N | N |
| Gan study set vao class | Y | Y | N |
| Go study set khoi class | Y | Y | N |
| Roi class (leave) | N | Y | Y |

Owner khong the roi class (phai xoa hoac chuyen quyen, ngoai scope Phase 7).
Moi user chi co mot role trong mot class (no duplicate member).

### 4.4 Invite code

- Invite code la 8 ky tu tu alphabet `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`
  (loai cac ky tu gay nham: 0/O, 1/I/L).
- Generate bang `crypto/rand`, retry khi unique conflict (max 5 lan).
- Invite code luu trong column `invite_code VARCHAR(8) UNIQUE`.
- `POST /v1/classes/{code}/join` la public endpoint (co rate limit).
- Join bang code tu dong gan role `student`. Owner khong the join class
  cua chinh minh.
- Invite code khong thay the auth. User join bang code van phai co Bearer
  token hop le.
- Code co the reset bang `POST /v1/classes/{id}/invite-code/reset` (owner
  only); code cu bi invalidate ngay lap tuc.

### 4.5 Activity feed aggregation

`GET /v1/activity` la endpoint tong hop tu nhieu nguon:

```text
Class service (handler)
  |
  +-- 1. Query activity_events WHERE user_id = ? (class events: joined,
  |       created, study-set added, member added)
  |
  +-- 2. Internal HTTP GET /internal/study/progress/recent?userId=?&limit=?
  |       (Study service: recent progress records)
  |
  +-- 3. [Optional Phase 7] Internal HTTP GET
  |       /internal/quiz/results/recent?userId=?&limit=?
  |       (Quiz service: recent live quiz results)
  |
  +-- Merge theo occurred_at DESC, cursor phan trang
  +-- Tra ActivityFeedResponse
```

Cursor phan trang: opaque base64 encode {source, id, occurred_at} de frontend
co the load more. Phase 7 chi can cursor don gian: timestamp + tiebreak ID.

Neu internal call Study hoac Quiz fail: log canh bao, tra partial result tu
cac nguon con lai. Khong fail toan bo request vi mot nguon con unavailable.

### 4.6 NATS event contract Phase 7

Class service publish event qua outbox pattern (tuong tu Phase 6). Consumer
trong Phase 7 la optional (khong co service nao bat buoc consume ngay); viec
publish la de cac phase sau (analytics, notification) co the subscribe.

| Subject | Khi publish |
| --- | --- |
| `hquizlet.class.created.v1` | Class da persist |
| `hquizlet.class.updated.v1` | Class da update |
| `hquizlet.class.deleted.v1` | Class da xoa |
| `hquizlet.class.member.joined.v1` | Member join thanh cong |
| `hquizlet.class.member.removed.v1` | Member bi xoa khoi class |
| `hquizlet.class.studyset.added.v1` | Study set gan vao class |
| `hquizlet.class.studyset.removed.v1` | Study set go khoi class |

Envelope tuong tu Phase 6:

```json
{
  "eventId": "018f...",
  "eventType": "class.member.joined",
  "eventVersion": 1,
  "aggregateId": "class-id",
  "occurredAt": "2026-09-02T10:00:00Z",
  "requestId": "req_123",
  "data": {}
}
```

Khong publish PII (email, raw token) trong payload. User ID la int; class name
la string khong noi toi thong tin ca nhan.

### 4.7 Consistency model

- PostgreSQL la source of truth cho class, member, study-set relation va
  activity events.
- Moi hanh dong quan trong ghi `activity_events` + `class_event_outbox` trong
  cung DB transaction voi thay doi chinh.
- Outbox worker poll va publish NATS JetStream at-least-once; consumer
  deduplicate theo `eventId`.
- Class service khong dung Redis trong Phase 7 (khong co live state phuc tap
  nhu Phase 6). Cache co the them o phase sau neu can.
- Study service tra study activity qua internal HTTP; Phase 7 khong require
  event fan-out cho activity. Neu Study service restart, activity feed se tu
  hien thi lai sau khi Study healthy.

## 5. Database Schema

### 5.1 `classes`

```sql
CREATE TABLE classes (
  id         BIGSERIAL    PRIMARY KEY,
  owner_user_id BIGINT    NOT NULL,
  name       TEXT         NOT NULL,
  description TEXT,
  invite_code VARCHAR(8)  NOT NULL UNIQUE,
  max_members INTEGER     NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

Constraints/indexes:

- `name` khong duoc rong sau khi trim (`CHECK (trim(name) <> '')`).
- `max_members BETWEEN 1 AND 1000`.
- Index `classes_owner_user_id_idx` tren `(owner_user_id)`.
- Index `classes_invite_code_idx` tren `(invite_code)` (unique da cover,
  explicit index de ro rang intent).
- `owner_user_id` la owner ban dau; se co entry tuong ung trong `class_members`
  voi `role = 'owner'`.

### 5.2 `class_members`

```sql
CREATE TABLE class_members (
  id         BIGSERIAL    PRIMARY KEY,
  class_id   BIGINT       NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  user_id    BIGINT       NOT NULL,
  role       VARCHAR(16)  NOT NULL DEFAULT 'student',
  joined_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (class_id, user_id)
);
```

Constraints/indexes:

- `role` check trong `('owner', 'teacher', 'student')`.
- Index `class_members_class_id_idx` tren `(class_id)`.
- Index `class_members_user_id_idx` tren `(user_id)` (de list classes cua mot
  user).
- Moi class co dung mot `owner`; enforce o service layer (khong dung DB
  partial unique trong Phase 7 de don gian migration).
- Cascade DELETE tu `classes.id` dam bao xoa class xoa toan bo members.

### 5.3 `class_study_sets`

```sql
CREATE TABLE class_study_sets (
  class_id       BIGINT       NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  study_set_id   BIGINT       NOT NULL,
  added_by_user_id BIGINT     NOT NULL,
  added_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY (class_id, study_set_id)
);
```

Constraints/indexes:

- `study_set_id` khong co foreign key sang Study service (cross-service).
  Study service verify su ton tai truoc khi Class service luu.
- Index `class_study_sets_class_id_idx` tren `(class_id)`.
- Index `class_study_sets_study_set_id_idx` tren `(study_set_id)` de biet
  study set nay dang o lop nao.
- Cascade DELETE tu `classes.id`; xoa class xoa tat ca study set assignments
  nhung khong xoa study set goc (study set song trong Study service).

### 5.4 `activity_events`

```sql
CREATE TABLE activity_events (
  id            BIGSERIAL     PRIMARY KEY,
  user_id       BIGINT        NOT NULL,
  event_type    VARCHAR(64)   NOT NULL,
  entity_type   VARCHAR(32)   NOT NULL,
  entity_id     BIGINT,
  class_id      BIGINT        REFERENCES classes(id) ON DELETE SET NULL,
  metadata      JSONB,
  occurred_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
```

Event types co the co:
- `class.created`, `class.updated`, `class.deleted`
- `class.member.joined`, `class.member.removed`
- `class.studyset.added`, `class.studyset.removed`

`metadata` JSONB luu context bo sung (vi du: class name tai thoi diem event,
ten study set) de hien thi feed ma khong phai re-fetch.

Index:

- `activity_events_user_id_occurred_at_idx` tren `(user_id, occurred_at DESC)`
  cho query chinh cua activity feed.
- `activity_events_class_id_idx` tren `(class_id)` cho class-scoped feed.
- Row TTL policy: Phase 7 khong enforce TTL; co the add pg_cron cleanup o
  phase sau. Ghi chu trong migration.

### 5.5 `class_event_outbox`

```sql
CREATE TABLE class_event_outbox (
  event_id       UUID          PRIMARY KEY,
  aggregate_id   BIGINT        NOT NULL,
  subject        VARCHAR(120)  NOT NULL,
  event_version  INTEGER       NOT NULL,
  payload        JSONB         NOT NULL,
  occurred_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
  published_at   TIMESTAMPTZ,
  attempts       INTEGER       NOT NULL DEFAULT 0,
  last_error     TEXT
);
```

Index partial: `(occurred_at) WHERE published_at IS NULL` de worker poll
chi scan unpublished events.

### 5.6 Migration requirements

- Five migrations: 001 den 005 theo thu tu phu thuoc.
- Moi migration co up/down ro rang.
- Chay duoc tren database sach (fresh) va tren database da co Phase 1-6.
- Rollback khong de lai table/index/constraint mo coi.
- Cascade delete duoc test trong migration test.
- Cross-service reference (`study_set_id`) khong co DB foreign key; comment
  ro trong migration file.
- Snapshot naming convention tuong tu Phase 6: `snapshotVersion` trong JSONB
  neu can sau.

## 6. Internal API Contract

Class service goi cac internal endpoint nay. Cac endpoint nay phai ton tai
va on dinh truoc khi Class service duoc deploy.

### 6.1 Study service internal API (required)

```text
GET /internal/study/study-sets/{id}
Header: X-Internal-Token: <shared secret>
Response 200: { "id": 10, "userId": 5, "title": "..." }
Response 404: { "code": "NOT_FOUND", ... }
```

Neu Study service chua co endpoint nay, Dev 2 (database/infra owner) phai
tao PR vao Study service trong Tuan 1 song song voi migrations.

### 6.2 Study service internal API (activity feed)

```text
GET /internal/study/progress/recent?userId=5&limit=20&before=<timestamp>
Header: X-Internal-Token: <shared secret>
Response 200: { "items": [...progress records...] }
```

Neu Study service chua co, Class service co the bo qua nguon nay trong
Phase 7 va chi tra class-domain activity. Ghi ro trong release gate report.

### 6.3 X-Internal-Token

- `CLASS_INTERNAL_TOKEN` env var trong Class service la token no gui di.
- `STUDY_INTERNAL_TOKEN` env var trong Study service la token no nhan.
- Trong Docker Compose dev, dung gia tri mac dinh `dev-internal-token`.
- Khong commit token that vao repo.
- Tuong lai: doi sang mTLS hoac service mesh JWT.

## 7. API Contract Phase 7

OpenAPI version: `1.5.0`

### 7.1 Class endpoints

| Method | Endpoint | Auth | Muc dich |
| --- | --- | --- | --- |
| `GET` | `/v1/classes` | Required | List classes user la member hoac owner |
| `POST` | `/v1/classes` | Required | Tao class moi |
| `GET` | `/v1/classes/{classId}` | Required, Member | Chi tiet class |
| `PUT` | `/v1/classes/{classId}` | Required, Owner | Sua class |
| `DELETE` | `/v1/classes/{classId}` | Required, Owner | Xoa class |
| `POST` | `/v1/classes/{code}/join` | Required | Join class bang invite code |
| `POST` | `/v1/classes/{classId}/invite-code/reset` | Required, Owner | Reset invite code |

### 7.2 Member endpoints

| Method | Endpoint | Auth | Muc dich |
| --- | --- | --- | --- |
| `GET` | `/v1/classes/{classId}/members` | Required, Member | List members |
| `POST` | `/v1/classes/{classId}/members` | Required, Owner/Teacher | Them member thu cong |
| `PUT` | `/v1/classes/{classId}/members/{userId}` | Required, Owner | Doi role member |
| `DELETE` | `/v1/classes/{classId}/members/{userId}` | Required, Owner | Xoa member |
| `DELETE` | `/v1/classes/{classId}/members/me` | Required, Member | Roi class |

### 7.3 Study set assignment endpoints

| Method | Endpoint | Auth | Muc dich |
| --- | --- | --- | --- |
| `GET` | `/v1/classes/{classId}/study-sets` | Required, Member | List study sets cua class |
| `POST` | `/v1/classes/{classId}/study-sets` | Required, Owner/Teacher | Gan study set |
| `DELETE` | `/v1/classes/{classId}/study-sets/{studySetId}` | Required, Owner/Teacher | Go study set |

### 7.4 Activity endpoint

| Method | Endpoint | Auth | Muc dich |
| --- | --- | --- | --- |
| `GET` | `/v1/activity` | Required | Activity feed cua current user |

### 7.5 Request/response schemas

`ClassSummary`:

```json
{
  "id": 1,
  "name": "English A1",
  "description": "Beginner English class",
  "inviteCode": "Q7KM2P3R",
  "memberCount": 12,
  "studySetCount": 5,
  "myRole": "owner",
  "createdAt": "2026-09-02T10:00:00Z",
  "updatedAt": "2026-09-02T10:00:00Z"
}
```

`ClassDetail`:

```json
{
  "id": 1,
  "name": "English A1",
  "description": "Beginner English class",
  "inviteCode": "Q7KM2P3R",
  "memberCount": 12,
  "studySetCount": 5,
  "myRole": "owner",
  "maxMembers": 100,
  "createdAt": "2026-09-02T10:00:00Z",
  "updatedAt": "2026-09-02T10:00:00Z"
}
```

`CreateClassRequest`:

```json
{
  "name": "English A1",
  "description": "Beginner English class",
  "maxMembers": 50
}
```

`UpdateClassRequest`:

```json
{
  "name": "English A1 - Updated",
  "description": "Updated description"
}
```

`ClassMember`:

```json
{
  "userId": 5,
  "role": "student",
  "joinedAt": "2026-09-02T10:00:00Z"
}
```

`AddMemberRequest`:

```json
{
  "userId": 5,
  "role": "student"
}
```

`UpdateMemberRoleRequest`:

```json
{
  "role": "teacher"
}
```

`ClassStudySetSummary`:

```json
{
  "studySetId": 10,
  "title": "Unit 1 Vocabulary",
  "flashcardCount": 30,
  "addedByUserId": 3,
  "addedAt": "2026-09-02T10:00:00Z"
}
```

`AddStudySetToClassRequest`:

```json
{
  "studySetId": 10
}
```

`JoinClassResponse`:

```json
{
  "classId": 1,
  "className": "English A1",
  "myRole": "student",
  "joinedAt": "2026-09-02T10:00:00Z"
}
```

`ActivityItem`:

```json
{
  "id": 101,
  "eventType": "class.member.joined",
  "entityType": "class",
  "entityId": 1,
  "classId": 1,
  "metadata": {
    "className": "English A1"
  },
  "occurredAt": "2026-09-02T10:00:00Z"
}
```

`ActivityFeedResponse`:

```json
{
  "items": [...],
  "nextCursor": "eyJpZCI6MTAxfQ==",
  "hasMore": true
}
```

Cursor la opaque base64; frontend khong parse, chi gui lai o request tiep
theo: `GET /v1/activity?cursor=<value>&limit=20`.

### 7.6 Error contract

Dung cung envelope nhu cac phase truoc:

```json
{
  "code": "FORBIDDEN",
  "message": "you do not have permission to perform this action",
  "requestId": "req_123",
  "details": {}
}
```

Bat buoc cover:

- `400` path/query param sai dinh dang.
- `401` thieu/invalid Bearer token.
- `403` user khong co role can thiet.
- `404` class, member, study set khong ton tai hoac khong phai member.
- `409` user da la member cua class (join hoac add duplicate).
- `409` study set da duoc gan vao class.
- `409` invite code conflict khi reset (nen retry tu dong, nhung neu
  qua gioi han retry thi bao loi ro rang).
- `422` validation: name rong, role khong hop le, maxMembers ngoai range.
- `429` rate limit join bang invite code.
- `503` Study service unavailable khi validate study set.
- `500` loi noi bo; khong leak `err.Error()` tho ra ngoai.

## 8. Backend Implementation Plan

### 8.1 Repository interfaces

```go
type ClassRepository interface {
    Create(ctx context.Context, ownerID int64, input CreateClassInput) (*Class, error)
    GetByID(ctx context.Context, classID int64) (*Class, error)
    GetByInviteCode(ctx context.Context, code string) (*Class, error)
    ListByUserID(ctx context.Context, userID int64) ([]*Class, error)
    Update(ctx context.Context, classID int64, input UpdateClassInput) (*Class, error)
    Delete(ctx context.Context, classID int64) error
    ResetInviteCode(ctx context.Context, classID int64, newCode string) error
}

type MemberRepository interface {
    Add(ctx context.Context, classID, userID int64, role string) (*ClassMember, error)
    ListByClass(ctx context.Context, classID int64) ([]*ClassMember, error)
    GetRole(ctx context.Context, classID, userID int64) (string, error)
    UpdateRole(ctx context.Context, classID, userID int64, role string) error
    Remove(ctx context.Context, classID, userID int64) error
    CountByClass(ctx context.Context, classID int64) (int, error)
}

type ClassStudySetRepository interface {
    Add(ctx context.Context, classID, studySetID, addedByUserID int64) error
    List(ctx context.Context, classID int64) ([]*ClassStudySet, error)
    Remove(ctx context.Context, classID, studySetID int64) error
    CountByClass(ctx context.Context, classID int64) (int, error)
}

type ActivityRepository interface {
    Create(ctx context.Context, event ActivityEvent) error
    ListByUser(ctx context.Context, userID int64, limit int, cursor *ActivityCursor) ([]*ActivityEvent, error)
}
```

Repository khong quyet dinh HTTP status. Chi tra typed domain errors:
`ErrNotFound`, `ErrConflict`, `ErrForbidden`, `ErrValidation`, `ErrInternal`.

### 8.2 Service layer rules

`ClassService` enforce:

- `name` trim, khong rong, max 120 ky tu.
- `description` optional, trim.
- `maxMembers` trong khoang [2, 1000], default 100.
- Khi tao class: tao member record cho owner voi `role = 'owner'`.
- Owner khong the bi xoa khoi class bang `/members/{userId}`.
- Owner khong the doi role cua chinh minh.
- Khi xoa class: cascade DB, ghi activity event, publish outbox.

`MemberService` enforce:

- User da la member thi join/add phai tra `409 CONFLICT`.
- Join bang code: user phai co Bearer token (da qua Gateway auth).
- Them member thu cong: kiem tra `max_members` truoc khi add.
- `role` phai la `'owner'`, `'teacher'`, hoac `'student'`; khong the set
  `owner` qua add member endpoint (owner la nguoi tao class).
- Xoa member: chi owner; khong the xoa owner.
- Roi class (`DELETE /members/me`): khong the roi neu la owner.

`ClassStudySetService` enforce:

- Goi Study internal API de xac nhan study set ton tai.
- Chua bat buoc study set phai thuoc ve user (teacher co the gan study set cua
  nguoi khac neu Study service cho phep doc); ghi ro trong ADR neu can thay doi.
  Phase 7: chi can study set ton tai, khong enforce ownership cua study set.
- Duplicate study set trong class tra `409`.

`ActivityService`:

- Ghi class-domain activity events trong cung DB transaction voi thay doi chinh
  (qua transactional repository).
- `GetFeed(userID, limit, cursor)`: query `activity_events` + goi Study
  internal API + merge + sort + cursor.
- Neu Study internal call fail, log warning va tra partial result.

### 8.3 HTTP handlers

Handlers theo pattern cua Study service:

- Doc `X-User-ID` tu header (inject boi Gateway).
- Parse va validate path params/query/body.
- Goi service method.
- Map typed service error sang HTTP status.
- Tra JSON response theo contract.
- Khong co SQL, NATS, hoac business logic trong handler.

### 8.4 Outbox worker

Tuong tu Phase 6 outbox:

- Goroutine poll `class_event_outbox WHERE published_at IS NULL`.
- Publish NATS JetStream voi `Msg-Id = event_id` (deduplication).
- Danh dau `published_at = NOW()` sau khi NATS ACK.
- Retry voi backoff; ghi `attempts` va `last_error`.
- Startup: neu NATS unavailable, service van khoi dong (outbox publish se
  retry); API endpoint van chay, chuc nang khong bi block.

## 9. Docker Compose Changes

Cap nhat `infra/docker/docker-compose.yml`:

```yaml
class:
  build:
    context: ../..
    dockerfile: infra/docker/go-service.Dockerfile
    args:
      SERVICE: class
  environment:
    PORT: "8084"
    DATABASE_URL: "${DATABASE_URL:-postgres://hquizlet:hquizlet@postgres:5432/hquizlet?sslmode=disable}"
    AUTH_SERVICE_URL: "${AUTH_SERVICE_URL:-http://auth:8081}"
    STUDY_SERVICE_URL: "${STUDY_SERVICE_URL:-http://study:8082}"
    NATS_URL: "${NATS_URL:-nats://nats:4222}"
    CLASS_INTERNAL_TOKEN: "${CLASS_INTERNAL_TOKEN:-dev-internal-token}"
  ports:
    - "${CLASS_PORT:-8084}:8084"
  depends_on:
    postgres:
      condition: service_healthy
    auth:
      condition: service_healthy
    nats:
      condition: service_started
  restart: on-failure
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://localhost:8084/healthz"]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 15s
```

Gateway service cap nhat:

```yaml
environment:
  CLASS_SERVICE_URL: "${CLASS_SERVICE_URL:-http://class:8084}"
```

`depends_on` cua gateway them `class: condition: service_healthy`.

`servicesHealth` trong Gateway them Class service vao danh sach kiem tra.

NATS cap nhat them `-js` flag de bat JetStream (neu Phase 6 chua lam):

```yaml
nats:
  command: ["-js"]
```

`go.work` them `services/class`:

```text
./services/class
```

## 10. Gateway Routes

```go
classURL := env("CLASS_SERVICE_URL", "http://localhost:8084")
mux.HandleFunc("/v1/classes", authenticatedProxy(authURL, classURL))
mux.HandleFunc("/v1/classes/", authenticatedProxy(authURL, classURL))
mux.HandleFunc("/v1/activity", authenticatedProxy(authURL, classURL))
```

Luu y: `/v1/classes/{code}/join` la public (co rate limit), nhung van can
Bearer token de biet user nao join. Dung `authenticatedProxy` cho toan bo
`/v1/classes/` la dung vi join endpoint van yeu cau login.

Rate limit cho join endpoint: Gateway them middleware dem request theo IP
tren `/v1/classes/` + method POST + path suffix `/join`. Gioi han de xuat:
10 requests/minute/IP. Tra `429 TOO_MANY_REQUESTS` voi `Retry-After` header.

Spoofed header rules:

- Gateway strip `X-User-ID`, `X-Class-Role`, `X-Member-ID` tu client.
- Chi inject `X-User-ID` tu verified identity.

## 11. Frontend Plan

### 11.1 Routes moi

```text
/classes                  -> Class list page
/classes/new              -> Create class page
/classes/:id              -> Class detail page (tabs: Study Sets, Members)
/classes/:id/edit         -> Edit class page
/classes/join             -> Join class by invite code page
/activity                 -> Activity feed page
```

### 11.2 Class list page

- Hien thi danh sach classes user la member.
- Moi card hien thi: name, member count, study set count, role (badge),
  invite code (chi hien voi owner).
- Button "Create class" va "Join class".
- Loading/empty/error states.

### 11.3 Class detail page

- Hai tabs: "Study Sets" va "Members".
- Tab Study Sets: danh sach study set gang vao class, button "Add study set"
  (owner/teacher), button "Remove" cho tung set.
- Tab Members: danh sach members voi role badge, button "Add member"
  (owner/teacher), button "Remove" (owner), button "Change role" (owner).
- Header: class name, description, invite code (copy button, chi hien voi
  owner/teacher), member count, study set count.
- Edit/Delete class (chi hien voi owner).
- "Leave class" button (cho non-owner member).

### 11.4 Create/Edit class form

- Fields: Name (required), Description (optional), Max members.
- Validation: name khong rong, max members 2-1000.
- Submit tao/cap nhat; redirect den class detail sau khi thanh cong.

### 11.5 Join class page

- Input field cho invite code.
- Submit button.
- Hien thi class name sau khi join thanh cong; redirect den class detail.
- Error states: code khong hop le, da la member, class day.

### 11.6 Activity feed page

- Danh sach activity items sap xep theo thoi gian moi nhat.
- Moi item hien thi: icon theo event type, mo ta (vi du: "You joined English
  A1"), thoi gian tuong doi (X minutes ago).
- Infinite scroll hoac "Load more" button dung cursor pagination.
- Loading/empty/error states.
- Empty state: "No recent activity. Start studying or join a class!"

### 11.7 TypeScript types

```typescript
interface ClassSummary {
  id: number;
  name: string;
  description: string | null;
  inviteCode: string;
  memberCount: number;
  studySetCount: number;
  myRole: 'owner' | 'teacher' | 'student';
  createdAt: string;
  updatedAt: string;
}

interface ActivityItem {
  id: number;
  eventType: string;
  entityType: string;
  entityId: number | null;
  classId: number | null;
  metadata: Record<string, unknown> | null;
  occurredAt: string;
}

interface ActivityFeedResponse {
  items: ActivityItem[];
  nextCursor: string | null;
  hasMore: boolean;
}
```

## 12. Tests

### 12.1 Backend unit/integration tests

- `ClassService`: tao class tao member owner, khong tao class voi name rong,
  xoa class xoa members (cascade).
- `MemberService`: join duplicate tra conflict, join bang code cap nhat
  member count, owner khong the bi xoa, khong the join class cua chinh minh.
- `ClassStudySetService`: add duplicate tra conflict, add study set khong ton
  tai tra error, remove study set khong anh huong study set goc.
- `ActivityService`: merge activity tu class domain va Study internal (mock
  client), cursor phan trang chinh xac, Study unavailable tra partial result.
- `MemberRepository`: unique constraint class_id/user_id, cascade delete,
  count chinh xac.
- Migration: fresh database PASS, up/down/re-up PASS, xoa class cascade
  members va study-set assignments.

### 12.2 Gateway security tests

- `/v1/classes` khong co token tra `401`.
- `/v1/classes/{id}` voi spoofed `X-User-ID` header bi strip.
- `/v1/classes/{code}/join` voi token khong hop le tra `401`.
- Rate limit join: sau 10 request/minute cung IP tra `429`.
- `/v1/activity` khong co token tra `401`.

### 12.3 Frontend tests

- Class list: render dung cards, empty state.
- Create class form: validation, submit.
- Class detail: hien thi dung tabs, add/remove study set, add/remove member.
- Activity feed: render items, load more.
- Join class: submit code, success/error states.

### 12.4 E2E script `infra/scripts/phase7-e2e.sh`

Cover:

1. Register User A (owner) va User B (student).
2. User A tao class, verify invite code trong response.
3. User B join class bang invite code.
4. User A them study set vao class (study set da tao tu Phase 3).
5. User B xem class detail - thay study set.
6. User A them User C thu cong voi role `teacher`.
7. User A doi role User B len `teacher`.
8. User B (bay gio la teacher) them study set moi vao class.
9. User A xoa study set khoi class; verify study set goc van ton tai trong
   Study service.
10. User B roi class.
11. User A xoa User C khoi class.
12. User A xoa class; verify User B khong the truy cap class nua.
13. User A xem activity feed; verify co items.
14. Cross-user security: User B khong tao class voi `X-User-ID` gia.
15. Rate limit: 11 join request trong 1 phut tu cung IP tra `429` lan cuoi.

## 13. CI

Pipeline Phase 7 them:

- `go build ./services/class/...`
- `go test ./services/class/...`
- `go test -race ./services/class/...`
- `go test ./services/gateway/...` (co Phase 7 route tests)
- `npm test --prefix apps/web`
- `npm run build --prefix apps/web`
- `npx @redocly/cli lint packages/api-contracts/openapi.yaml`
- `bash -n infra/scripts/phase7-e2e.sh`
- Docker fresh-volume E2E: opt-in `PHASE7_E2E_ENABLED=true`.

Khong de Phase 7 break Phase 4 golden tests, Phase 5 folder tests hoac
Phase 6 live quiz tests. Moi PR phai pass full regression suite.

## 14. Phan Cong 5 Developer

---

## Dev 1 - Contract, Gateway va Release Owner

Vai tro: freeze API/security contract, route Gateway, CI va quan ly gate.

Cong viec:

- `[P7-CON-01]` Nang version OpenAPI len `1.5.0`.
- `[P7-CON-02]` Dinh nghia Class, ClassDetail, ClassMember, ClassStudySet
  schemas trong OpenAPI.
- `[P7-CON-03]` Dinh nghia Activity, ActivityFeedResponse, cursor schema.
- `[P7-CON-04]` Them tat ca endpoints Class, Member, StudySet, Activity vao
  OpenAPI paths voi request/response/error examples.
- `[P7-CON-05]` Dinh nghia error codes: `CLASS_NOT_FOUND`, `NOT_MEMBER`,
  `ALREADY_MEMBER`, `FORBIDDEN_ROLE`, `CLASS_FULL`, `INVALID_INVITE_CODE`,
  `STUDY_SET_ALREADY_ADDED` trong shared error schema.
- `[P7-GW-01]` Them Gateway routes `/v1/classes`, `/v1/classes/`,
  `/v1/activity` dung `authenticatedProxy`.
- `[P7-GW-02]` Them rate limit middleware cho join endpoint (10 req/min/IP).
- `[P7-GW-03]` Cap nhat `servicesHealth` them Class service.
- `[P7-GW-04]` Them Gateway tests: auth required, spoofed X-User-ID bi strip,
  rate limit join, activity auth.
- `[P7-CI-01]` Them `go build/test/race services/class` vao CI.
- `[P7-CI-02]` Them `bash -n phase7-e2e.sh` syntax check vao CI.
- `[P7-DOC-01]` Them curl examples cho tat ca Class/Activity endpoints.
- `[P7-DOC-02]` Cap nhat README.md them Class service thong tin port/env.
- `[P7-GATE-01]` Tong hop va viet Phase 7 release gate report.

Definition of Done:

- OpenAPI lint PASS voi khong co schema loi.
- Gateway tests PASS.
- Contract freeze: moi PR sau merge contracts can approval Dev 1.
- Rate limit test co evidence (curl hoac script).

---

## Dev 2 - Infrastructure, Database va Internal API Owner

Vai tro: Class service scaffold, migrations, repository, Study internal API.

Cong viec:

- `[P7-INFRA-01]` Tao `services/class/go.mod`, `cmd/server/main.go` skeleton,
  `internal/config/config.go`, `Dockerfile` target.
- `[P7-INFRA-02]` Them `./services/class` vao `go.work`.
- `[P7-INFRA-03]` Cap nhat `infra/docker/docker-compose.yml`: them `class`
  service voi env vars day du, cap nhat `gateway` env vars, cap nhat NATS
  `-js` flag.
- `[P7-DB-01]` Tao migration `001_create_classes.sql` (up/down).
- `[P7-DB-02]` Tao migration `002_create_class_members.sql` (up/down).
- `[P7-DB-03]` Tao migration `003_create_class_study_sets.sql` (up/down).
- `[P7-DB-04]` Tao migration `004_create_activity_events.sql` (up/down).
- `[P7-DB-05]` Tao migration `005_create_class_event_outbox.sql` (up/down).
- `[P7-DB-06]` Tao `internal/migration/migration.go` dung embed SQL; test
  fresh DB + up/down/re-up.
- `[P7-REPO-01]` Implement `ClassRepository` (Create, GetByID,
  GetByInviteCode, ListByUserID, Update, Delete, ResetInviteCode).
- `[P7-REPO-02]` Implement `MemberRepository` (Add, ListByClass, GetRole,
  UpdateRole, Remove, CountByClass).
- `[P7-REPO-03]` Implement `ClassStudySetRepository` (Add, List, Remove,
  CountByClass).
- `[P7-REPO-04]` Implement `ActivityRepository` (Create, ListByUser voi
  cursor).
- `[P7-REPO-05]` Test cascade delete (class -> members, class -> study-sets),
  unique constraints, count accuracy.
- `[P7-INTERNAL-01]` Tao `internal/client/study_client.go`: HTTP client goi
  Study internal API voi timeout, retry, `X-Internal-Token`.
- `[P7-INTERNAL-02]` Neu Study service chua co `/internal/study/study-sets/{id}`,
  tao PR vao Study service de them endpoint nay.
- `[P7-INTERNAL-03]` Test study_client voi mock server: happy path, 404, 503.

Definition of Done:

- Migration chay duoc tu fresh DB va sau Phase 1-6 migrations.
- Repository unit tests PASS.
- Cascade delete verified bang test.
- Study client co test voi mock; khong panic khi Study unavailable.
- Docker Compose `docker compose -f infra/docker/docker-compose.yml up --build`
  khoi dong duoc class service voi healthcheck xanh.

---

## Dev 3 - Backend Service, Handler va NATS Owner

Vai tro: implement business logic, HTTP handlers, outbox publisher, activity
aggregation.

Cong viec:

- `[P7-GO-01]` Implement `ClassService`: Create (voi invite code generation),
  GetByID, ListByUserID, Update, Delete, ResetInviteCode.
- `[P7-GO-02]` Implement `MemberService`: JoinByCode (verify khong phai owner
  cua chinh minh, check max_members), AddMember, ListMembers, UpdateRole,
  RemoveMember, LeaveClass.
- `[P7-GO-03]` Implement `ClassStudySetService`: AddStudySet (goi Study
  internal API), ListStudySets, RemoveStudySet.
- `[P7-GO-04]` Implement `ActivityService`: RecordEvent (transactional),
  GetFeed (merge class events + Study progress, cursor paging).
- `[P7-GO-05]` Implement HTTP handlers cho toan bo Class + Member + StudySet +
  Activity endpoints.
- `[P7-GO-06]` Implement typed error -> HTTP status mapping.
- `[P7-GO-07]` Unit tests: ClassService (name validation, cascade, role),
  MemberService (duplicate join, owner protection, max members),
  ActivityService (merge, partial failure Study).
- `[P7-GO-08]` Integration tests voi real PostgreSQL (dung test DB).
- `[P7-NATS-01]` Tao `internal/events/publisher.go`: NATS JetStream client,
  stream setup (`HQUIZLET_CLASS`), publish voi Msg-Id deduplication.
- `[P7-NATS-02]` Tao `internal/events/outbox.go`: goroutine poll outbox,
  retry voi exponential backoff, danh dau published_at.
- `[P7-NATS-03]` Service startup: neu NATS unavailable, log warning nhung
  khong crash. Outbox retry tu dong.
- `[P7-NATS-04]` Test: publish event sau Create/Join/Delete, outbox retry
  khi NATS down, khong double publish.

Definition of Done:

- All service tests PASS (`go test ./services/class/...`).
- Race detector PASS (`go test -race ./services/class/...`).
- Handlers khong chua SQL hoac NATS call truc tiep.
- Owner protection co test ro rang.
- NATS down khong anh huong API availability; outbox catch up sau khi NATS
  quay lai.
- Activity feed tra partial result khi Study unavailable (co test).

---

## Dev 4 - Frontend Owner

Vai tro: xay tat ca Class va Activity UI end-to-end.

Cong viec:

- `[P7-FE-API-01]` Them Class API client: classApi.list, create, get, update,
  delete, join, resetInviteCode.
- `[P7-FE-API-02]` Them Member API client: listMembers, addMember, updateRole,
  removeMember, leaveClass.
- `[P7-FE-API-03]` Them StudySet-in-Class API client: listClassStudySets,
  addStudySet, removeStudySet.
- `[P7-FE-API-04]` Them Activity API client: getFeed (voi cursor support).
- `[P7-FE-CLASS-01]` Build Class list page: cards, empty state, create/join
  buttons.
- `[P7-FE-CLASS-02]` Build Create class form: validation, submit, redirect.
- `[P7-FE-CLASS-03]` Build Class detail page: header, Study Sets tab, Members
  tab.
- `[P7-FE-CLASS-04]` Build Add study set dialog: search/select study set,
  submit.
- `[P7-FE-CLASS-05]` Build Member list: role badges, Add member dialog,
  Change role dropdown (owner only), Remove button (owner only), Leave button
  (non-owner member).
- `[P7-FE-CLASS-06]` Build Edit class page: name/description form.
- `[P7-FE-CLASS-07]` Build Delete class confirmation dialog.
- `[P7-FE-JOIN-01]` Build Join class page: invite code input, submit, success/
  error states.
- `[P7-FE-ACT-01]` Build Activity feed page: list items, event type icons,
  relative timestamps, Load more (cursor).
- `[P7-FE-NAV-01]` Them navigation links: Classes, Activity.
- `[P7-FE-TEST-01]` Tests: Class list loading/empty/error, create form
  validation, join form success/error.
- `[P7-FE-TEST-02]` Tests: class detail tabs render dung, add/remove study
  set.
- `[P7-FE-TEST-03]` Tests: member list, role badge, activity feed render va
  load more.

Definition of Done:

- Frontend build PASS (`npm run build --prefix apps/web`).
- Frontend tests PASS (`npm test --prefix apps/web`).
- Tat ca Class/Member/Activity flows chay tren API that (khong co mock
  production path).
- Loading/error/empty states co o moi page.
- Role-based UI: edit/delete chi hien voi owner, Add member chi hien voi
  owner/teacher.

---

## Dev 5 - E2E, Security Va Release Owner

Vai tro: chung minh Phase 7 chay that, dong gate.

Cong viec:

- `[P7-E2E-01]` Tao `infra/scripts/phase7-e2e.sh` cover full flow (16 steps
  nhu Section 12.4).
- `[P7-E2E-02]` Docker fresh-volume E2E: xay build tu dau, chay full stack,
  attach output va commit SHA.
- `[P7-E2E-03]` Security matrix: User B khong truy cap class cua User A, User
  B khong sua class cua User A, spoofed X-User-ID bi tu choi.
- `[P7-E2E-04]` Rate limit test: xac nhan 429 sau 10 join/minute tu cung IP.
- `[P7-E2E-05]` Xoa class E2E: verify member khong xem duoc class sau khi
  xoa, verify study set van ton tai trong Study service.
- `[P7-E2E-06]` Activity feed E2E: verify items xuat hien sau cac hanh dong
  class va sau khi hoc study set.
- `[P7-E2E-07]` NATS outbox E2E: tat NATS, thuc hien hanh dong class, bat lai
  NATS, verify events duoc publish.
- `[P7-E2E-08]` Restart recovery: restart Class service, verify data van dung
  trong PostgreSQL, outbox catch up.
- `[P7-REGRESSION-01]` Chay Phase 4 golden tests, Phase 5 folder E2E va Phase
  6 live quiz flow sau khi Phase 7 merge. Dam bao khong co regression.
- `[P7-QA-01]` Lap Phase 7 release gate report voi command, commit SHA va
  evidence link.
- `[P7-QA-02]` Viet failure-mode matrix: Study unavailable, NATS unavailable,
  Class service restart, max members exceeded.

Definition of Done:

- Docker fresh-volume E2E PASS voi commit SHA ro rang.
- Security matrix co evidence cu the (curl commands + responses).
- Rate limit co evidence.
- Phase 4/5/6 regression tests PASS.
- Gate report duoc merge vao `docs/phase/phase-7-release-gate-{date}.md`.

---

## 15. Lich Thuc Hien 4 Tuan

| Tuan | Dev 1 | Dev 2 | Dev 3 | Dev 4 | Dev 5 |
| --- | --- | --- | --- | --- | --- |
| Tuan 1 | Freeze OpenAPI 1.5.0: Class + Activity schemas + error codes | Service scaffold (go.mod, Dockerfile, Compose) + migrations 001-005 | Service/handler skeleton (no logic) + NATS client setup | UX audit + routes + API client skeleton | E2E script skeleton, failure-mode matrix draft |
| Tuan 2 | Gateway routes + auth + rate limit | Repository impl (ClassRepo, MemberRepo) + Study internal client | ClassService + MemberService logic + tests | Class list + Create + Join pages | Happy-path API E2E (curl), security matrix draft |
| Tuan 3 | Gateway tests (auth/spoof/rate) + CI gate | ClassStudySetRepo + ActivityRepo + migration tests | ClassStudySetService + ActivityService + NATS outbox | Class detail (tabs) + Activity feed page | Security E2E + rate limit evidence + NATS outbox test |
| Tuan 4 | curl examples + CI full gate + release report | Cascade/constraint hardening + fresh DB evidence | Race tests + handler integration tests | Frontend tests + build + a11y fixes | Docker fresh-volume E2E + regression suite + gate report |

## 16. Dependency Va Thu Tu Merge

1. Merge ADR/contract freeze (Dev 1 contracts) truoc bat ky PR nao khac.
2. Dev 2 merge service scaffold + Compose + migrations (prerequisites cho moi
   dev khac de run locally).
3. Dev 1 merge Gateway routes sau khi service scaffold co healthcheck.
4. Dev 2 merge repositories.
5. Dev 3 merge services/handlers (yeu cau repositories on dinh).
6. Dev 2 merge Study internal client (co the song song voi Dev 3 sau scaffold).
7. Dev 4 merge API clients truoc, sau do tung page theo thu tu dependency.
8. Dev 3 merge NATS outbox sau khi service handlers stable.
9. Dev 5 merge E2E + regression; release report cuoi cung.

Khong de hai PR sua cung luc OpenAPI, go.work, hoac docker-compose.yml.
Thay doi contract sau freeze can approval Dev 1 + tat ca dev bi anh huong.

## 17. Branch Va PR Rules

- Branch: `phase7/dev{n}-{task-id}-{short-name}`.
  Vi du: `phase7/dev2-p7-db-01-class-migration`.
- Mot PR giai quyet mot boundary co the review/test doc lap.
- PR bat buoc co:
  - Task ID va API/contract impact.
  - Migration impact (neu co).
  - Security/permission impact.
  - Test commands va expected output.
  - Evidence (output, curl, screenshot neu can).
  - Rollback/rollout plan.
- Khong push thang `main`.
- Khong commit token, secret, DB dump, hoac PII.
- Moi thay doi permission logic phai co test bao phu owner/teacher/student
  va unauthorized case.

## 18. Gate Bat Buoc Truoc Phase 8

### Contract gate

- OpenAPI `1.5.0` lint PASS.
- Class, Activity examples validate duoc.
- Error code matrix day du.
- Frontend/backend dung cung contract (khong co field ngam).

### Database gate

- Fresh database migration PASS (001-005).
- Up/down/re-up PASS.
- Existing Phase 1-6 database migration PASS.
- Cascade delete verified: xoa class xoa members/study-set-assignments, khong
  xoa study set goc.
- Unique constraints: duplicate member, duplicate study set trong class.
- Count accurate sau add/remove.

### Security gate

- Tat ca endpoints can auth; khong co endpoint public ngoai `/v1/classes/{code}/join`
  (van can Bearer token).
- Spoofed `X-User-ID` bi strip va reject.
- User khong phai member khong truy cap class detail.
- User khong phai owner khong sua/xoa class.
- User khong phai owner/teacher khong them/go member/study-set.
- Rate limit join PASS.
- Owner khong the bi xoa khoi class cua chinh minh.
- Cross-user: User B khong doc data cua User A.

### Build/test gate

- `go build ./services/class/...` PASS.
- `go test ./services/class/...` PASS.
- `go test -race ./services/class/...` PASS.
- `go test ./services/gateway/...` PASS.
- `npm test --prefix apps/web` PASS.
- `npm run build --prefix apps/web` PASS.
- Phase 4 golden tests PASS.
- Phase 5 folder E2E PASS.
- Phase 6 live quiz E2E PASS (khong co regression).

### NATS/outbox gate

- Events duoc publish sau Class create/join/delete/studyset-add.
- NATS down khong crash API; outbox catch up sau NATS quay lai.
- Khong double publish (Msg-Id deduplication).

### UX gate

- Class list/detail/create/edit/delete hoat dong tren API that.
- Join class bang invite code hoat dong.
- Member add/remove/role-change hoat dong.
- Study set add/remove hoat dong.
- Activity feed hien thi dung items.
- Loading/error/empty states day du o moi page.
- Role-based UI (edit/delete/add-member chi hien voi dung role).

### Docker gate

- `docker compose -f infra/docker/docker-compose.yml up --build` khoi dong
  duoc full stack bao gom Class service.
- `GET /healthz/services` bao gom class: ok.
- Docker fresh-volume E2E PASS voi commit SHA ro rang.

## 19. Checklist Tong Hop

- `[ ]` Dong Phase 6 fresh-volume gate.
- `[ ]` OpenAPI 1.5.0 voi Class + Activity schemas.
- `[ ]` OpenAPI examples va error codes.
- `[ ]` Gateway routes Class + Activity.
- `[ ]` Gateway rate limit join endpoint.
- `[ ]` Gateway security tests.
- `[ ]` `services/class` scaffold (go.mod, cmd, config).
- `[ ]` `go.work` cap nhat.
- `[ ]` Docker Compose them Class service.
- `[ ]` NATS `-js` flag trong Compose.
- `[ ]` Migration `001_create_classes.sql`.
- `[ ]` Migration `002_create_class_members.sql`.
- `[ ]` Migration `003_create_class_study_sets.sql`.
- `[ ]` Migration `004_create_activity_events.sql`.
- `[ ]` Migration `005_create_class_event_outbox.sql`.
- `[ ]` Migration tests (fresh/up/down/cascade).
- `[ ]` `ClassRepository` + tests.
- `[ ]` `MemberRepository` + tests.
- `[ ]` `ClassStudySetRepository` + tests.
- `[ ]` `ActivityRepository` + tests.
- `[ ]` Study internal client + tests.
- `[ ]` `ClassService` + tests.
- `[ ]` `MemberService` + tests.
- `[ ]` `ClassStudySetService` + tests.
- `[ ]` `ActivityService` + merge + cursor tests.
- `[ ]` HTTP handlers cho tat ca endpoints.
- `[ ]` Typed error -> HTTP status mapping.
- `[ ]` NATS publisher + stream setup.
- `[ ]` Outbox worker + retry.
- `[ ]` Invite code CSPRNG generation.
- `[ ]` Frontend Class API + Activity API clients.
- `[ ]` TypeScript types cho Class/Member/Activity.
- `[ ]` Class list page.
- `[ ]` Create class form.
- `[ ]` Join class page.
- `[ ]` Class detail page (Study Sets + Members tabs).
- `[ ]` Edit class page.
- `[ ]` Delete class confirmation.
- `[ ]` Activity feed page.
- `[ ]` Role-based UI (owner/teacher/student).
- `[ ]` Frontend tests.
- `[ ]` Frontend production build.
- `[ ]` `infra/scripts/phase7-e2e.sh`.
- `[ ]` Docker fresh-volume E2E evidence.
- `[ ]` Security matrix evidence.
- `[ ]` Rate limit evidence.
- `[ ]` NATS outbox recovery evidence.
- `[ ]` Phase 4/5/6 regression PASS.
- `[ ]` Phase 7 release gate report voi commit SHA.

## 20. Ket Luan

Phase 7 khong chi la CRUD class. Deliverable la:

- Mot service Go moi doc lap (`services/class`) voi clean architecture day du,
  migrations, repository, service, handler, NATS outbox.
- Permission model ro rang (owner/teacher/student) duoc enforce o service
  layer va co evidence qua security tests.
- Invite code dung CSPRNG, khong suy doan duoc, co rate limit protect.
- Activity feed la diem tich hop dau tien giua Class domain va Study domain,
  mo duong cho analytics phase sau.
- NATS event contract da versioned, san sang cho consumer tuong lai.

Phase chi duoc danh dau GO khi Docker fresh-volume E2E PASS, security matrix
co evidence, Phase 4/5/6 khong co regression, Class service restart khong
mat data, NATS down khong crash API, va tat ca contract/migration/test/UX
gate co evidence gan voi commit SHA cu the.
