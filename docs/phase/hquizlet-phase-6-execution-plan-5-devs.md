# HQuizlet Platform - Phase 6 Execution Plan (5 Developers)

## 1. Muc Tieu Phase 6

Phase 6 xay dung Live Quiz end-to-end tren Study Set that: host tao phong,
nguoi choi join bang code, host dieu khien vong cau hoi, server cham diem va
tat ca client nhan leaderboard theo thoi gian thuc.

Ket qua cuoi phase:

- Host da dang nhap tao Live Session tu study set minh so huu.
- Player co the join bang code va display name ma khong can tai khoan.
- Host co the start, mo/dong cau hoi, chuyen cau va ket thuc session.
- Server la nguon chan ly cho clock, dap an dung, score va session state.
- Player submit moi cau mot lan theo idempotency contract; reconnect khong nop
  trung hoac mat identity.
- Host va player nhan state/leaderboard realtime qua Server-Sent Events (SSE).
- PostgreSQL luu session, participant, answer va ket qua cuoi.
- Redis luu state nong co TTL; PostgreSQL van la durable source of truth.
- NATS JetStream phat domain event versioned theo co che at-least-once.
- Gateway tach ro host auth va participant-token auth, luon strip identity
  header do client gui.
- OpenAPI, migrations, backend/frontend tests, chaos/reconnect tests va Docker
  fresh-volume E2E deu xanh truoc khi danh dau GO.

## 2. Baseline Va Dieu Kien Bat Dau

### 2.1 Trang thai repo hien tai

CodeGraph va source hien tai cho thay:

- `services/quiz` dang xu ly deterministic generate/evaluate cua Phase 4,
  nhung chua co live domain, database repository hoac Redis/NATS client.
- Gateway da khai bao `/v1/live-sessions` va `/v1/live-sessions/`, nhung dang
  dung `reverseProxy` khong xac thuc. Khong duoc xem route placeholder nay la
  implementation Phase 6.
- Gateway da co pattern `authenticatedProxy`: verify token voi Auth, strip
  `X-User-ID`, inject identity va forward request ID.
- Docker Compose da co PostgreSQL, Redis 7 va NATS 2, nhung Quiz service chua
  nhan `REDIS_URL`/`NATS_URL`; NATS chua bat JetStream va chua co healthcheck.
- OpenAPI hien tai o version `1.4.0`, co contract quiz generate/evaluate nhung
  chua co Live Quiz schemas/endpoints.
- Frontend dang dung state-based navigation trong `apps/web/src/main.tsx`,
  chua co route hay feature Live Quiz.
- ADR-003 dang chon Go production runtime; Phase 6 khong tu dong dua Rust vao
  request path.

### 2.2 Prerequisite tu Phase 5

Phase 5 release report hien la `CONDITIONAL GO` cho den khi fresh-volume Docker
E2E co evidence. Truoc khi freeze Phase 6 contract can:

- Dong Phase 5 fresh-volume gate hoac ghi ro exception duoc release owner chap
  thuan.
- Khong de thay doi Folder chua on dinh lam nhiem regression baseline.
- Giu tat ca Phase 4 deterministic quiz/golden tests xanh vi Live Quiz se tai
  su dung question generation va scoring.

## 3. Pham Vi

### 3.1 In scope

- Live Quiz domain trong `services/quiz`.
- PostgreSQL migrations cho session, participant, answer va event outbox.
- Redis-backed live state, presence, current question va leaderboard.
- NATS JetStream events versioned va idempotent consumer contract.
- Host-authenticated API, public join API va participant-token API.
- SSE stream cho host/player, co reconnect bang `Last-Event-ID`.
- Host lobby/control UI, join UI, player screen va leaderboard UI.
- API, integration, race, reconnect, E2E, load-smoke va failure-mode tests.
- Metrics, structured logs va Phase 6 release gate report.

### 3.2 Out of scope

- Team mode, tournament bracket, matchmaking cong khai.
- Audio/video chat va chat room.
- Native mobile implementation.
- Co-host, moderator va chuyen quyen host.
- Anonymous spectator rieng biet.
- Cross-region active-active va multi-datacenter Redis.
- Payment/entitlement cho Live Quiz.
- Bat buoc dua Rust FFI/WASM/sidecar vao production.
- Bao cao analytics day du; Phase 6 chi luu ket qua can cho audit va replay.

## 4. Quyet Dinh Kien Truc

### 4.1 Service ownership

Live Quiz thuoc `services/quiz`. Quiz service so huu lifecycle, scoring,
PostgreSQL tables, Redis keys va NATS subjects cua live domain. Study service
van so huu study set/flashcard; Quiz chi doc snapshot qua internal Study API.

De xuat cau truc:

```text
services/quiz/
  cmd/server/main.go                       # refactor root main neu can
  internal/live/model/
  internal/live/repository/
  internal/live/service/
  internal/live/http/
  internal/live/realtime/
  internal/live/redisstore/
  internal/live/events/
  internal/live/outbox/
  migrations/
```

Khong de Live handler ghi SQL/Redis/NATS truc tiep. Handler goi service;
service dieu phoi repository, state store, scoring engine va event publisher.

### 4.2 Transport realtime: SSE + HTTP commands

Phase 6 chon:

- HTTP JSON cho create/join/start/open/close/next/answer/end.
- SSE cho server -> host/player state, countdown, participant va leaderboard.
- Polling `GET state` la fallback khi SSE mat ket noi.

Ly do:

- Luong command cua Live Quiz la client -> server theo request ro rang, khong
  can full-duplex WebSocket.
- SSE co san tren browser, de reconnect va de quan sat hon trong phase dau.
- Gateway hien la Go reverse proxy don gian; SSE can streaming flush va cancel
  propagation, nhung khong can WebSocket upgrade implementation.

Gateway reverse proxy phai duoc harden cho SSE:

- Flush tung event, khong buffer toan bo response.
- Propagate context cancellation khi browser disconnect.
- Khong dat response timeout ngan cho stream.
- Gui heartbeat comment moi 15 giay.
- Set `Cache-Control: no-cache`, `Connection: keep-alive` va
  `X-Accel-Buffering: no` neu deploy sau reverse proxy.

### 4.3 Server-authoritative state machine

State hop le:

```text
LOBBY -> QUESTION_OPEN -> QUESTION_CLOSED -> LEADERBOARD
  |            ^                                  |
  |            +------------- next ---------------+
  +-------------------------------------------> ENDED
```

Rules:

- Chi host cua session duoc transition state.
- `start` chi hop le tu `LOBBY` va khi study snapshot co du card.
- `submit answer` chi hop le trong `QUESTION_OPEN` va truoc `closesAt` theo
  server clock.
- `close` phai idempotent; auto-close va host-close khong tao hai ket qua.
- Het cau hoi thi chi co the `end`, khong wrap ve cau dau.
- `ENDED` la terminal state; khong join/start/answer lai.
- Moi transition dung optimistic version/CAS de tranh double command.

### 4.4 Question snapshot va scoring

Khi create/start session, Quiz service:

1. Xac minh host so huu study set qua Study service.
2. Doc danh sach flashcard that.
3. Freeze `study_set_id`, title, card IDs va noi dung can thiet vao snapshot.
4. Dung Go deterministic quiz engine hien tai de tao thu tu/cau hoi theo seed.
5. Khong gui dap an dung trong player payload.

Sua study set sau khi session tao khong duoc thay doi session dang chay. Score
duoc tinh server-side tu snapshot:

```text
baseScore = 1000 neu dung, 0 neu sai
timeBonus = floor(500 * remainingMs / questionDurationMs)
questionScore = baseScore + clamp(timeBonus, 0, 500)
```

Freeze cong thuc tren trong contract/golden vectors. Tie-break leaderboard:

1. Tong score giam dan.
2. So cau dung giam dan.
3. Tong response time tang dan.
4. `joined_at` tang dan.

### 4.5 Consistency model

- PostgreSQL la durable source cho session metadata, participant, answer,
  final score va outbox.
- Redis la projection/state nong de doc nhanh; moi key phai co TTL.
- Moi answer duoc persist PostgreSQL truoc khi ACK thanh cong.
- Redis leaderboard duoc cap nhat sau persistence; neu cap nhat Redis fail,
  worker co the rebuild tu PostgreSQL.
- Mutating command ghi domain row + outbox trong cung DB transaction.
- Outbox worker publish NATS JetStream at-least-once va danh dau published.
- Consumer deduplicate theo `eventId`; khong gia dinh exactly-once.

### 4.6 Authentication va trust boundary

Co hai identity type:

- Host: Bearer token tu Auth service, Gateway inject `X-User-ID`.
- Player guest: opaque participant token do Join API cap, server chi luu hash.

Rules bat buoc:

- Create/control/end/host stream/host state can host auth.
- Join by code la public nhung co rate limit va validation.
- Player state/answer/stream can `Authorization: Participant <token>` hoac
  participant bearer scheme duoc freeze trong OpenAPI.
- Gateway strip `X-User-ID`, `X-Participant-ID`, `X-Live-Role` tu client truoc
  khi inject identity da verify.
- Join code khong duoc dung nhu credential de submit answer.
- Token khong ghi vao log, URL/query string, NATS payload hoac analytics.

## 5. Database Schema

### 5.1 `live_sessions`

```sql
CREATE TABLE live_sessions (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(8) NOT NULL UNIQUE,
  host_user_id BIGINT NOT NULL,
  study_set_id BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL,
  seed BIGINT NOT NULL,
  question_count INTEGER NOT NULL,
  question_duration_ms INTEGER NOT NULL,
  current_question_index INTEGER,
  state_version BIGINT NOT NULL DEFAULT 1,
  question_snapshot JSONB NOT NULL,
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Constraints/indexes:

- `status` check trong `LOBBY`, `QUESTION_OPEN`, `QUESTION_CLOSED`,
  `LEADERBOARD`, `ENDED`.
- `question_count > 0`; duration trong khoang contract, vi du 5-120 giay.
- Index `(host_user_id, created_at DESC)` va `(status, updated_at)`.
- Code sinh bang CSPRNG tu alphabet khong gay nham (`ABCDEFGHJKLMNPQRSTUVWXYZ23456789`),
  retry khi unique conflict; khong dung auto-increment de suy code.

### 5.2 `live_session_participants`

```sql
CREATE TABLE live_session_participants (
  id UUID PRIMARY KEY,
  live_session_id BIGINT NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  user_id BIGINT,
  display_name VARCHAR(40) NOT NULL,
  token_hash CHAR(64) NOT NULL UNIQUE,
  total_score INTEGER NOT NULL DEFAULT 0,
  correct_count INTEGER NOT NULL DEFAULT 0,
  total_response_time_ms BIGINT NOT NULL DEFAULT 0,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  left_at TIMESTAMPTZ,
  UNIQUE (live_session_id, display_name)
);
```

Display name uniqueness can normalize case/space bang mot cot normalized hoac
unique functional index. Khong luu raw participant token.

### 5.3 `live_session_answers`

```sql
CREATE TABLE live_session_answers (
  id BIGSERIAL PRIMARY KEY,
  live_session_id BIGINT NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  participant_id UUID NOT NULL REFERENCES live_session_participants(id) ON DELETE CASCADE,
  question_index INTEGER NOT NULL,
  flashcard_id BIGINT NOT NULL,
  submitted_answer JSONB NOT NULL,
  is_correct BOOLEAN NOT NULL,
  score_awarded INTEGER NOT NULL,
  response_time_ms INTEGER NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (live_session_id, participant_id, question_index),
  UNIQUE (participant_id, idempotency_key)
);
```

### 5.4 `live_event_outbox`

```sql
CREATE TABLE live_event_outbox (
  event_id UUID PRIMARY KEY,
  aggregate_id BIGINT NOT NULL,
  subject VARCHAR(120) NOT NULL,
  event_version INTEGER NOT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at TIMESTAMPTZ,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);
```

Index partial tren `occurred_at` khi `published_at IS NULL` de worker poll.

### 5.5 Migration requirements

- Up/down ro rang va chay sau migrations Phase 1-5.
- Fresh database va existing database deu PASS.
- Constraint/index name deterministic.
- Rollback khong de lai trigger/index/type mo coi.
- Snapshot schema co `snapshotVersion` de migrate/replay an toan.
- Test cascade, uniqueness, concurrent answer va outbox transaction.

## 6. Redis Design

Key convention, tat ca co namespace va TTL:

```text
live:v1:session:{sessionId}:state          HASH/JSON, TTL 24h sau ended
live:v1:session:{sessionId}:participants   SET, TTL 24h sau ended
live:v1:session:{sessionId}:presence       ZSET lastSeenEpochMs, TTL 30m
live:v1:session:{sessionId}:leaderboard    ZSET compositeScore, TTL 24h
live:v1:session:{sessionId}:question       HASH, TTL session lifetime
live:v1:code:{joinCode}                    STRING sessionId, TTL session lifetime
live:v1:events:{sessionId}                 STREAM replay window, maxlen bounded
```

Rules:

- Khong luu correct answer trong key player co the truy cap.
- State transition dung Lua script hoac `WATCH`/transaction CAS tren
  `stateVersion`.
- Presence chi la hint; offline khong tu dong xoa participant/score.
- SSE event co monotonically increasing event ID va replay bounded.
- Startup/recovery rebuild Redis projection tu PostgreSQL neu key mat.
- Redis unavailable: khong mo transition/answer moi neu khong dam bao
  consistency; tra `503 LIVE_STATE_UNAVAILABLE`, khong silently fallback vao
  memory cua mot instance.

## 7. NATS Event Contract

### 7.1 Subjects

| Subject | Khi publish |
| --- | --- |
| `hquizlet.live.session.created.v1` | Session da persist |
| `hquizlet.live.participant.joined.v1` | Participant join thanh cong |
| `hquizlet.live.session.started.v1` | Host start session |
| `hquizlet.live.question.opened.v1` | Cau hoi bat dau |
| `hquizlet.live.answer.submitted.v1` | Answer da persist/cham diem |
| `hquizlet.live.question.closed.v1` | Het gio/host close |
| `hquizlet.live.session.ended.v1` | Final result da persist |

### 7.2 Envelope

```json
{
  "eventId": "018f...",
  "eventType": "live.answer.submitted",
  "eventVersion": 1,
  "aggregateId": "123",
  "occurredAt": "2026-09-02T10:00:00Z",
  "requestId": "req_123",
  "data": {}
}
```

Khong publish raw token, correct answer truoc reveal, email hoac PII khong can
thiet. Bat JetStream trong Compose (`-js`), them healthcheck va volume neu can
durability local. Publisher dung message ID = `eventId`.

## 8. API Contract

### 8.1 Endpoint matrix

| Method | Endpoint | Identity | Muc dich |
| --- | --- | --- | --- |
| `POST` | `/v1/live-sessions` | Host | Tao lobby |
| `GET` | `/v1/live-sessions/{sessionId}` | Host | Host state/detail |
| `POST` | `/v1/live-sessions/{code}/join` | Public | Join va nhan participant token |
| `GET` | `/v1/live-sessions/{sessionId}/player-state` | Player | Polling fallback |
| `GET` | `/v1/live-sessions/{sessionId}/events` | Host/Player | SSE theo scope |
| `POST` | `/v1/live-sessions/{sessionId}/start` | Host | Start va mo cau dau |
| `POST` | `/v1/live-sessions/{sessionId}/questions/current/close` | Host | Dong cau hien tai |
| `POST` | `/v1/live-sessions/{sessionId}/questions/next` | Host | Mo cau tiep |
| `POST` | `/v1/live-sessions/{sessionId}/answers` | Player | Submit answer |
| `GET` | `/v1/live-sessions/{sessionId}/leaderboard` | Host/Player | Leaderboard hien tai |
| `POST` | `/v1/live-sessions/{sessionId}/end` | Host | Ket thuc session |

Roadmap ban dau co 4 endpoint; cac endpoint bo sung la can thiet de bieu dien
state machine va realtime mot cach explicit. Contract freeze phai quyet dinh
path parameter nao la `sessionId`, path nao la `code`, khong overload mo ho.

### 8.2 Request/response toi thieu

`CreateLiveSessionRequest`:

```json
{
  "studySetId": 101,
  "questionCount": 10,
  "questionDurationMs": 20000,
  "seed": 42
}
```

`CreateLiveSessionResponse`:

```json
{
  "id": 9001,
  "code": "Q7KM2P",
  "status": "LOBBY",
  "studySetId": 101,
  "questionCount": 10,
  "questionDurationMs": 20000,
  "stateVersion": 1,
  "createdAt": "2026-09-02T10:00:00Z"
}
```

`JoinLiveSessionRequest` va response:

```json
{
  "displayName": "An"
}
```

```json
{
  "sessionId": 9001,
  "participantId": "018f...",
  "participantToken": "one-time-opaque-value",
  "status": "LOBBY"
}
```

`SubmitLiveAnswerRequest`:

```json
{
  "questionIndex": 0,
  "answer": { "selectedIndex": 2 },
  "idempotencyKey": "018f-answer-001"
}
```

Response chi tra accepted/correct sau policy reveal da freeze. Mac dinh khong
leak `correct` trong khi `QUESTION_OPEN`:

```json
{
  "accepted": true,
  "questionIndex": 0,
  "submittedAt": "2026-09-02T10:00:12Z"
}
```

### 8.3 SSE contract

Event names toi thieu:

- `session.snapshot`
- `participant.joined`
- `session.started`
- `question.opened`
- `answer.accepted` (chi player cua answer do)
- `question.closed`
- `leaderboard.updated`
- `session.ended`
- `heartbeat`

Moi data envelope co `eventId`, `sessionId`, `stateVersion`, `serverTime` va
payload. Player event khong co correct answer truoc `question.closed`. Client
gui `Last-Event-ID` khi reconnect; neu replay gap thi server gui
`session.snapshot` moi.

### 8.4 Error contract

Tiep tuc dung envelope:

```json
{
  "code": "LIVE_INVALID_STATE",
  "message": "question is not open",
  "requestId": "req_123",
  "details": { "status": "LEADERBOARD", "stateVersion": 7 }
}
```

Bat buoc cover:

- `401 UNAUTHORIZED` cho host token thieu/sai.
- `401 PARTICIPANT_TOKEN_INVALID` cho player token sai/het han.
- `403 FORBIDDEN` khi khong phai host/participant cua session.
- `404 LIVE_SESSION_NOT_FOUND` va join code khong hop le/het han.
- `409 LIVE_INVALID_STATE`, `ANSWER_ALREADY_SUBMITTED`,
  `DISPLAY_NAME_TAKEN`, stale state version.
- `410 LIVE_SESSION_ENDED` khi resource da terminal/expired.
- `422 VALIDATION_ERROR` cho payload/path/range sai.
- `429 RATE_LIMITED` cho join/answer abuse.
- `503 LIVE_STATE_UNAVAILABLE` khi Redis/NATS dependency can thiet khong san sang.

## 9. Backend Implementation Plan

### 9.1 Repository interfaces

Implement theo boundary, khong tra HTTP status:

- `CreateSession`, `GetSession`, `GetHostSession`, `UpdateSessionCAS`.
- `CreateParticipant`, `GetParticipantByTokenHash`, `TouchParticipant`.
- `InsertAnswerIdempotent`, `ListAnswers`, `FinalizeScores`.
- `CreateOutboxEvent`, `ClaimOutboxBatch`, `MarkPublished`.
- `RebuildSessionProjection`.

Typed errors:

- `ErrNotFound`, `ErrForbidden`, `ErrInvalidState`, `ErrConflict`.
- `ErrAlreadyAnswered`, `ErrExpired`, `ErrValidation`.
- `ErrStateUnavailable`, `ErrDependencyUnavailable`.

### 9.2 Live service

Can implement:

- CSPRNG join code va participant token.
- Ownership check qua Study client.
- Freeze question snapshot/version.
- State transition CAS va timer auto-close an toan khi restart.
- Server-side scoring va deterministic tie-break.
- Answer validation theo participant/session/current question/deadline.
- Idempotent retry tra cung ket qua cho cung key/payload; cung key payload khac
  tra conflict.
- Recovery scan cho session non-terminal khi process restart.
- Rebuild Redis va resume timer tu PostgreSQL timestamps.

Khong tao mot goroutine vo han cho moi session ma khong co lifecycle control.
Scheduler can co cancellation, bounded worker va recovery test.

### 9.3 HTTP/SSE handlers

- Gioi han body size va reject unknown JSON fields.
- Parse int/UUID/code an toan.
- Map typed error sang envelope co request ID.
- SSE authorize truoc khi flush header.
- Khong ghi token/payload dap an vao log.
- Handle disconnect/backpressure va slow consumer; khong block broadcaster.
- Response mutating command tra state version moi.

### 9.4 Gateway

Thay hai placeholder `reverseProxy(quizURL)` bang route policy ro rang:

- Host endpoints -> `authenticatedProxy`.
- Join endpoint -> public rate-limited proxy.
- Player endpoints -> verify participant token tai Quiz/internal verifier hoac
  forward token de Quiz verify; tuyet doi khong inject participant ID tu raw
  client header.
- SSE proxy streaming/flush/cancel tests.
- Rate limit theo IP + join code cho join; participant ID cho answer.
- Strip spoofed host/player identity headers.

### 9.5 Configuration va dependency readiness

Quiz config them:

- `DATABASE_URL`, `REDIS_URL`, `NATS_URL`.
- `LIVE_SESSION_TTL`, `LIVE_JOIN_RATE_LIMIT`.
- `LIVE_MAX_PARTICIPANTS`, `LIVE_SSE_REPLAY_SIZE`.
- Timeout/pool size cho Study, Redis, PostgreSQL va NATS.

`/healthz` chi chung minh process live. Them `/readyz` kiem PostgreSQL, Redis,
NATS va Study dependency voi timeout; Gateway service health nen phan biet
`ok`, `degraded`, `offline` neu contract cho phep.

## 10. Frontend Implementation Plan

### 10.1 Feature structure va navigation

De xuat:

```text
apps/web/src/features/live/
  LiveHome.tsx
  CreateLiveSession.tsx
  HostLobby.tsx
  HostGame.tsx
  JoinLiveSession.tsx
  PlayerLobby.tsx
  PlayerGame.tsx
  LiveLeaderboard.tsx
  liveApi.ts
  liveEvents.ts
  useLiveSession.ts
  live.css
```

Neu chua dua router library vao app, mo rong `AppView` co typed payload cho
phase nay. Tuy nhien deep-link/reload cua join code phai duoc giai quyet; uu
tien them route toi thieu:

```text
/live
/live/host/:sessionId
/live/join/:code?
/live/play/:sessionId
```

### 10.2 API client va participant session

- Types sinh/doi chieu tu OpenAPI.
- `create`, `join`, `getHostState`, `getPlayerState`, `start`, `close`, `next`,
  `submitAnswer`, `leaderboard`, `end`.
- EventSource native khong set custom Authorization header; implementation
  phai dung fetch streaming hoac participant cookie httpOnly. Khong dua token
  vao query string.
- Neu Phase 6 dung fetch-SSE, parser can test partial chunk, multi-line data,
  retry va `Last-Event-ID`.
- Participant token chi luu session-scoped; document rui ro neu dung
  `sessionStorage`. Host tiep tuc dung auth token pattern hien tai.

### 10.3 Host UX

- Chon study set va options hop le.
- Lobby hien join code lon, copy action va participant presence.
- Disable start khi chua du cau hoi hoac request dang chay.
- Host controls ro state: start, close, next, end.
- Hien answer count, countdown theo `closesAt`/server time, khong dung client
  clock lam authority.
- Confirm end; reconnect phai restore dung state.

### 10.4 Player UX

- Join form co uppercase/trim code, display name validation va rate-limit error.
- Lobby co reconnect/removed/ended state.
- Question screen chi cho submit mot lan; retry idempotent khi network error.
- Sau submit hien waiting, khong leak correctness truoc reveal.
- Countdown la presentation; server response moi quyet dinh late answer.
- Refresh tab restore participant session neu con token.
- Accessibility: keyboard answer, focus state, aria-live cho countdown/state,
  khong chi dung mau de bieu thi dung/sai.

### 10.5 Leaderboard UX

- Stable ordering theo server rank.
- Highlight current player.
- Hien top N va current player neu ngoai top N.
- Loading/reconnecting/stale indicator.
- Final leaderboard freeze khi `ENDED`.

## 11. Testing Plan

### 11.1 Contract tests

- OpenAPI lint va example validation.
- Host/player security schemes khong bi nhap nhang.
- SSE event fixtures validate theo schema/version.
- Khong co correct answer trong open-question player fixture.
- Frontend types/request khop OpenAPI.

### 11.2 Unit tests

- State transition table: moi state x command.
- Score boundary: dung/sai, 0 ms, deadline, sau deadline.
- Tie-break deterministic.
- Join code/token randomness interface test, collision retry.
- Display name normalization.
- Answer idempotency va payload mismatch.
- Question snapshot khong doi khi source study set doi.
- Event envelope khong chua secret/answer leak.

### 11.3 Repository/integration tests

- Migration up/down/fresh/existing DB.
- Concurrent join cung display name: chi mot success.
- Concurrent submit cung participant/question: chi mot answer/score.
- CAS transition: chi mot close/start/next success.
- DB + outbox cung transaction.
- Redis projection rebuild sau flush/restart.
- Outbox retry va NATS duplicate deduplication.

Chay `go test -race` cho live service, broadcaster va scheduler.

### 11.4 Gateway/SSE security tests

- Host endpoint no/invalid token -> `401`.
- Public join khong cho spoof `X-User-ID`.
- Spoofed `X-Participant-ID`/role bi strip.
- Participant A khong answer/stream session cua participant B.
- Non-host khong start/next/end.
- SSE flush event truoc khi stream dong.
- Disconnect cancel upstream; reconnect replay/snapshot dung.
- Slow consumer khong lam block session.
- Join/answer rate limit tra `429` envelope.

### 11.5 Frontend tests

- Create/lobby loading, error, success.
- Join invalid code/name taken/rate limited/success.
- SSE snapshot, incremental event, duplicate event va replay gap.
- Submit double click chi tao mot idempotency key/request logic.
- Network error retry cung key.
- Refresh/reconnect restore state.
- Countdown khong quyet dinh acceptance client-side.
- Host controls enable/disable theo state.
- Leaderboard ordering/highlight/final state.
- Session expired va auth expired behavior.

### 11.6 Docker E2E

Tao:

```text
infra/scripts/phase6-e2e.sh
```

Flow bat buoc:

1. Start stack tu fresh PostgreSQL/Redis/NATS volumes.
2. Register Host A va User B.
3. Host A tao study set co it nhat 5 flashcards.
4. User B bi chan khi tao live session tu set cua Host A.
5. Host A tao session, verify code va `LOBBY`.
6. Player 1 va Player 2 join; duplicate name bi chan.
7. Host stream nhan participant events.
8. Non-host bi chan khi start.
9. Host start, player streams nhan cau hoi khong co dap an dung.
10. Hai player submit; duplicate retry khong tang diem hai lan.
11. Late answer bi chan.
12. Close -> leaderboard dung score/tie-break.
13. Next question va lap it nhat 2 vong.
14. Restart Quiz service; state/participant/leaderboard duoc recovery.
15. Disconnect/reconnect SSE voi `Last-Event-ID` khong mat state.
16. Host end; final result persist PostgreSQL.
17. Answer/control sau end bi chan.
18. Verify NATS events/outbox khong mat va khong chua token.

Browser E2E neu Playwright co san: mot host context + hai player browser
contexts chay flow lobby -> answer -> leaderboard -> final.

### 11.7 Load va resilience smoke

Muc tieu Phase 6 local/staging, can freeze bang ADR/performance note:

- 1 session voi 100 concurrent participants.
- p95 answer ACK < 250 ms trong moi truong benchmark da ghi ro.
- p95 event fanout < 500 ms.
- Khong data race, goroutine leak hoac unbounded memory growth.
- Redis restart, NATS tam mat va Quiz restart co ket qua documented.
- NATS down khong lam mat durable outbox; backlog drain sau recovery.

Day khong phai production capacity claim; report phai ghi hardware, commit SHA,
tool va dataset.

## 12. Observability Va Operations

Metrics toi thieu:

- `live_sessions_active`.
- `live_participants_connected`.
- `live_commands_total{command,result}`.
- `live_answers_total{result}`.
- `live_answer_latency_ms`.
- `live_sse_connections` va `live_sse_dropped_total{reason}`.
- `live_outbox_pending`, `live_outbox_publish_failures_total`.
- Redis/PostgreSQL/NATS operation latency/error.

Structured log fields: `requestId`, `sessionId`, `participantId` (ID, khong
token), `stateVersion`, `command`, `eventId`, `durationMs`. Khong log raw
answer neu co the chua PII; khong log correct answer truoc reveal.

Runbook can co:

- Redis projection rebuild.
- Outbox backlog/drain.
- Stuck `QUESTION_OPEN` recovery.
- Force-end session bang admin procedure co audit (khong can public API phase
  nay).
- Safe cleanup session/Redis keys het TTL.

## 13. CI Va Release Gate

CI jobs:

- OpenAPI lint + Live examples/SSE fixtures validation.
- Go format/vet/test cho Quiz/Gateway.
- `go test -race` cho live packages.
- Frontend tests va production build.
- PostgreSQL migration fresh/up/down test.
- Redis/NATS integration tests.
- `phase6-e2e.sh` syntax check.
- Docker fresh-volume E2E opt-in/nightly hoac release job.
- Load-smoke job co threshold, khong flaky.
- `git diff --check` va secret scan.

Phase 6 GO gate:

- Contract va security model freeze/PASS.
- Phase 4 quiz regression/golden PASS.
- Migrations fresh/existing/up/down PASS.
- State/concurrency/race tests PASS.
- Gateway host/player identity va SSE tests PASS.
- Frontend test/build PASS.
- Docker fresh-volume + restart/reconnect E2E PASS.
- Redis/NATS failure evidence va outbox recovery PASS.
- Load-smoke dat threshold da freeze.
- Release report co command, date, environment, commit SHA va evidence.

## 14. Phan Cong 5 Developer

## Dev 1 - Contract, Gateway Va Security Owner

Vai tro: freeze public contract, identity boundary, streaming proxy va release
coordination.

Cong viec:

- `[P6-CON-01]` Them OpenAPI tags, security schemes va Live schemas.
- `[P6-CON-02]` Freeze endpoint/state/error/idempotency contract.
- `[P6-CON-03]` Them HTTP va SSE golden examples/version validator.
- `[P6-GW-01]` Thay placeholder Live proxy bang host/public/player policy.
- `[P6-GW-02]` Implement/test SSE flush, cancel va reconnect forwarding.
- `[P6-SEC-01]` Strip spoofed identity headers va add rate limits.
- `[P6-CI-01]` Contract/security gate va release report template.

Definition of Done:

- OpenAPI lint/examples PASS.
- Host/player/public route matrix co automated tests.
- Gateway khong tin identity do client tu khai.
- SSE qua Gateway flush/reconnect dung.

## Dev 2 - Persistence, Redis Va NATS Owner

Vai tro: durable storage, state projection va reliable event delivery.

Cong viec:

- `[P6-DB-01]` Tao 4 migrations va constraints/indexes.
- `[P6-DB-02]` Implement repository + transaction/outbox.
- `[P6-REDIS-01]` Implement key schema, TTL, CAS va projection rebuild.
- `[P6-NATS-01]` Bat JetStream, healthcheck va versioned publisher.
- `[P6-OUTBOX-01]` Implement bounded retry/backoff/dedup.
- `[P6-DB-TEST-01]` Concurrent answer/join/transition tests.
- `[P6-RECOVERY-01]` Redis/NATS/Quiz restart recovery tests.

Definition of Done:

- Fresh/existing/up/down migration PASS.
- Khong double score khi concurrent/retry.
- Outbox khong mat event khi NATS down.
- Redis projection rebuild duoc tu PostgreSQL.

## Dev 3 - Live Domain Va Realtime Backend Owner

Vai tro: state machine, scoring, scheduler, handlers va broadcaster.

Cong viec:

- `[P6-GO-01]` Implement model/service state machine.
- `[P6-GO-02]` Integrate Study ownership + frozen question snapshot.
- `[P6-GO-03]` Implement participant token va answer idempotency.
- `[P6-GO-04]` Implement server scoring/tie-break golden tests.
- `[P6-GO-05]` Implement HTTP command handlers va typed errors.
- `[P6-SSE-01]` Implement scoped SSE broadcaster/replay/heartbeat.
- `[P6-TIMER-01]` Implement auto-close/recovery scheduler.
- `[P6-OBS-01]` Them metrics, structured logs va readiness.

Definition of Done:

- State transition table va race tests PASS.
- Player khong nhan correct answer truoc reveal.
- Restart khong mat current state/deadline.
- Khong goroutine leak/slow-consumer blocking trong test.

## Dev 4 - Host Frontend Owner

Vai tro: UX tao session, lobby va dieu khien host.

Cong viec:

- `[P6-FE-API-01]` Them shared Live API/types/event client.
- `[P6-FE-HOST-01]` Build create Live Session flow.
- `[P6-FE-HOST-02]` Build lobby, join code va participant list.
- `[P6-FE-HOST-03]` Build start/close/next/end controls.
- `[P6-FE-HOST-04]` Build host question/answer-count/leaderboard view.
- `[P6-FE-RECONNECT-01]` Restore host state sau reload/reconnect.
- `[P6-FE-TEST-01]` Them host loading/error/reconnect/state tests.

Definition of Done:

- Host flow chay tren Gateway API that.
- Double click/stale command khong tao transition trung.
- UI dung server time/state version.
- Frontend tests/build PASS.

## Dev 5 - Player Frontend, E2E Va Performance Owner

Vai tro: player UX va evidence chay that cua Phase 6.

Cong viec:

- `[P6-FE-JOIN-01]` Build join code/display name flow.
- `[P6-FE-PLAYER-01]` Build lobby/question/submit/wait/reveal screens.
- `[P6-FE-LB-01]` Build realtime/final leaderboard.
- `[P6-FE-A11Y-01]` Keyboard/focus/aria-live/accessibility tests.
- `[P6-E2E-01]` Tao `infra/scripts/phase6-e2e.sh`.
- `[P6-E2E-02]` Multi-client ownership/idempotency/restart/reconnect E2E.
- `[P6-PERF-01]` 100-participant load-smoke va report.
- `[P6-QA-01]` Lap release evidence va failure-mode matrix.

Definition of Done:

- Player refresh/reconnect khong mat identity/state.
- Double submit khong double score.
- Docker fresh-volume E2E PASS.
- Performance report co environment/commit/threshold ro rang.

## 15. Lich Thuc Hien 4 Tuan

| Tuan | Dev 1 | Dev 2 | Dev 3 | Dev 4 | Dev 5 |
| --- | --- | --- | --- | --- | --- |
| Tuan 1 | Freeze state/API/security/SSE | Schema, Compose JetStream, repository skeleton | State machine/scoring spec | Host UX/routes skeleton | Player UX + E2E/load skeleton |
| Tuan 2 | OpenAPI examples, Gateway route policy | Repository, Redis CAS/projection | Create/join/start/answer handlers | Create + lobby + SSE client | Join + lobby + player question |
| Tuan 3 | SSE proxy/security/rate tests | Outbox publisher + recovery | Close/next/end, scheduler, SSE replay | Host controls/leaderboard/reconnect | Submit/reveal/leaderboard + API E2E |
| Tuan 4 | Contract/CI/docs/release gate | Failure/restart/concurrency hardening | Race/metrics/readiness/regression | Frontend tests/build/a11y fixes | Fresh E2E, load-smoke, evidence |

## 16. Dependency Va Thu Tu Merge

1. Merge ADR/contract state machine + identity + SSE decision truoc.
2. Dev 1 merge OpenAPI schemas/examples; freeze contract.
3. Dev 2 merge migrations va repository interfaces.
4. Dev 3 merge domain/state/scoring khong transport, sau do handlers/SSE.
5. Dev 2 merge Redis/outbox/NATS integration theo service interfaces da freeze.
6. Dev 1 merge Gateway route policy va streaming proxy.
7. Dev 4/5 merge shared API/event client truoc, sau do host/player UI tach PR.
8. Dev 5 merge E2E/load evidence; release owner chot gate report.

Khong de hai PR sua cung luc OpenAPI, shared live types hoac event envelope.
Thay doi contract sau freeze can migration/version note va approval Dev 1 +
cac owner bi anh huong.

## 17. Branch Va PR Rules

- Branch: `phase6/dev{n}-{task-id}-{short-name}`.
- Mot PR chi xu ly mot boundary co the review/test doc lap.
- PR bat buoc ghi:
  - task ID va state/API/event impact.
  - database/Redis/NATS impact.
  - security/privacy impact.
  - test commands va evidence.
  - rollout/rollback/recovery plan.
- Khong push thang `main`.
- Khong commit token, Redis dump, database dump, correct-answer fixture nhay cam
  hoac screenshot co du lieu nguoi dung.
- Moi thay doi concurrency/timer/SSE phai co cancellation/backpressure analysis.

## 18. Gate Bat Buoc Truoc Phase 7

### Contract gate

- OpenAPI lint va HTTP/SSE examples PASS.
- State machine, score, idempotency va error contract freeze.
- Host/player identity route matrix khong mo ho.

### Database va consistency gate

- Fresh/existing/up/down migration PASS.
- Concurrent command khong double transition/double score.
- PostgreSQL + outbox atomic; Redis rebuild duoc.
- Restart Quiz/Redis/NATS co evidence recovery.

### Security gate

- Host chi dieu khien session cua minh.
- Player token bi scope vao dung participant/session.
- Join code khong thay the credential.
- Spoofed identity bi strip; rate limits PASS.
- Correct answer/token khong leak qua API/SSE/log/NATS.

### Realtime gate

- SSE flush, heartbeat, disconnect, replay va snapshot PASS.
- Slow consumer khong block session.
- Reconnect/refresh khong mat identity/state.

### Build/test/performance gate

- Go tests/build/race PASS.
- Phase 4 golden/regression PASS.
- Frontend tests/build/a11y PASS.
- Docker fresh-volume multi-client E2E PASS.
- 100-participant load-smoke dat threshold da freeze.

### UX gate

- Host create/lobby/control/end hoat dong tren API that.
- Player join/answer/reveal/leaderboard hoat dong tren API that.
- Loading/error/reconnecting/expired/ended states day du.
- Countdown khong tao client-authoritative behavior.

## 19. Checklist Tong Hop

- `[ ]` Dong prerequisite Phase 5 fresh-volume gate.
- `[ ]` ADR/state machine/SSE/identity decision freeze.
- `[ ]` OpenAPI Live Quiz + participant auth scheme.
- `[ ]` HTTP/SSE examples va validators.
- `[ ]` `live_sessions` migration.
- `[ ]` `live_session_participants` migration.
- `[ ]` `live_session_answers` migration.
- `[ ]` `live_event_outbox` migration.
- `[ ]` Repository va typed errors.
- `[ ]` Redis key/TTL/CAS/projection rebuild.
- `[ ]` NATS JetStream + outbox publisher.
- `[ ]` Question snapshot va ownership validation.
- `[ ]` State machine/scheduler/recovery.
- `[ ]` Server scoring/tie-break golden tests.
- `[ ]` Participant token va answer idempotency.
- `[ ]` HTTP handlers.
- `[ ]` SSE broadcaster/replay/heartbeat.
- `[ ]` Gateway host/public/player route policy.
- `[ ]` Gateway SSE/security/rate-limit tests.
- `[ ]` Readiness, metrics va structured logs.
- `[ ]` Frontend shared Live API/event client.
- `[ ]` Host create/lobby/control/leaderboard UI.
- `[ ]` Player join/question/submit/reveal/leaderboard UI.
- `[ ]` Refresh/reconnect/accessibility states.
- `[ ]` Backend integration/race/recovery tests.
- `[ ]` Frontend tests/build.
- `[ ]` `infra/scripts/phase6-e2e.sh`.
- `[ ]` Docker fresh-volume/restart/reconnect evidence.
- `[ ]` 100-participant load-smoke report.
- `[ ]` Runbook va Phase 6 release gate report.

## 20. Ket Luan

Phase 6 khong chi la them bon endpoint placeholder. Deliverable la mot Live
Quiz server-authoritative co state machine ro rang, identity host/player tach
biet, persistence va idempotency dung, Redis state co the rebuild, NATS event
khong mat nho outbox, va realtime SSE co reconnect.

Phase chi duoc danh dau GO khi multi-client flow chay tren Docker fresh volume,
restart/reconnect khong lam sai state hay score, correct answer/credential
khong bi leak, va tat ca contract, race, security, frontend, E2E va performance
gate co evidence gan voi commit SHA cu the.
