# HQuizlet Platform — Phase 3 Execution Plan (5 Developers)

Ngày lập: 2026-09-01  
Repo: `hunguyen1324/hquizlet-platform`  
Baseline: `main` tại `b5528c3f10216e00c0a5fbdae996db71d6518638`

## 1. Quyết định PM

Phase 3 tập trung vào **Learning Progress và nền tảng thuật toán học**: lưu tiến độ thật vào PostgreSQL, nối bốn chế độ Flashcards/Learn/Test/Match với backend, loại bỏ stub, và đưa Rust vào đúng phần logic thuần có thể kiểm thử/benchmark.

Không bắt đầu phát triển song song trước khi Dev 5 chạy lại Phase 2 gate trên fresh clone + fresh PostgreSQL volume. Commit mới nhất đã sửa thứ tự `pg_trgm`, nhưng Docker, migration và luồng A/B ownership vẫn cần bằng chứng chạy thật.

## 2. Mục tiêu và tiêu chí hoàn thành

### Mục tiêu

1. Lưu được learning session và kết quả từng thẻ.
2. Reload/truy cập lại vẫn thấy tiến độ đã lưu.
3. Bốn learning modes dùng dữ liệu PostgreSQL thật và gọi API progress thật.
4. Có thuật toán deterministic cho shuffle, scoring và sinh câu hỏi.
5. Rust chỉ được tích hợp sau khi contract và test vectors đã khóa.
6. Không phá vỡ auth, ownership, Study CRUD, Folder và Docker hiện có.

### Definition of Done

- User A không đọc/ghi được progress của User B.
- `POST /v1/study-sets/{id}/progress` lưu transaction thành công.
- `GET /v1/study-sets/{id}/progress` trả summary và lịch sử đúng contract.
- Dữ liệu per-card bị giới hạn tối đa 100 kết quả/request.
- Frontend không còn `saveProgress` no-op hoặc `fetchProgress` trả `[]` giả.
- Flashcards, Learn, Test, Match có loading/error/retry/completion state.
- Go unit/integration tests, Rust tests, frontend build/test và Docker gate đều pass.
- OpenAPI, backend JSON và TypeScript types khớp chính xác.
- Không có secret, mock production, placeholder hoặc conflict marker.

## 3. Phạm vi

### In scope

- Learning session/progress schema và migration.
- Progress API, authorization, validation và idempotency.
- Frontend progress client và UI history/summary tối thiểu.
- Tích hợp progress vào 4 learning modes.
- Rust crate cho deterministic quiz engine: shuffle, scoring, question generation.
- Contract tests, integration tests, observability cơ bản và Docker verification.

### Out of scope

- Live Quiz realtime, Redis/NATS production.
- Class, payment, wallet, upload file, mobile app.
- Recommendation/AI adaptive learning phức tạp.
- OAuth production.
- Tách Rust thành network microservice riêng nếu chưa có benchmark chứng minh cần thiết.

## 4. Contract đề xuất cần khóa ở ngày 1

### Schema PostgreSQL

`learning_sessions`

| Cột | Kiểu | Ghi chú |
| --- | --- | --- |
| `id` | BIGSERIAL | PK |
| `user_id` | BIGINT | lấy từ identity đã verify |
| `study_set_id` | BIGINT | FK, owner-scoped |
| `mode` | TEXT | flashcards/learn/test/match |
| `score` | INT | `0 <= score <= total` |
| `total` | INT | `0 <= total <= 100` |
| `started_at` | TIMESTAMPTZ | thời điểm bắt đầu |
| `completed_at` | TIMESTAMPTZ | nullable |
| `idempotency_key` | TEXT | unique theo user |
| `created_at` | TIMESTAMPTZ | audit |

`learning_card_results`

| Cột | Kiểu | Ghi chú |
| --- | --- | --- |
| `id` | BIGSERIAL | PK |
| `session_id` | BIGINT | FK cascade |
| `flashcard_id` | BIGINT | phải thuộc study set của session |
| `correct` | BOOLEAN | kết quả |
| `attempts` | INT | 1..100 |
| `response_time_ms` | INT | nullable, có giới hạn |

Index bắt buộc: `(user_id, study_set_id, created_at DESC)`, `(session_id)`, unique `(user_id, idempotency_key)`.

### API tối thiểu

| Method | Route | Mục đích |
| --- | --- | --- |
| `POST` | `/v1/study-sets/{studySetId}/progress` | lưu một session hoàn tất |
| `GET` | `/v1/study-sets/{studySetId}/progress` | summary + lịch sử có phân trang |
| `GET` | `/v1/study-sets/{studySetId}/progress/latest` | lần học gần nhất theo mode |

Payload POST không nhận `userId`; Gateway xác thực và inject identity. `studySetId` lấy từ path, không lặp trong body. Request có `mode`, `score`, `total`, `startedAt`, `completedAt`, `cardResults`, `idempotencyKey`.

Error envelope thống nhất:

```json
{
  "code": "VALIDATION_ERROR",
  "message": "score must be between 0 and total",
  "requestId": "...",
  "details": {}
}
```

## 5. Phân công 5 dev

### Dev 1 — Auth, identity và security contract

Ownership: `services/auth/**`; chỉ sửa gateway/OpenAPI qua PR phối hợp với Dev 5.

Nhiệm vụ:

- P3-AUTH-01: Chuẩn hóa error envelope Auth theo contract Phase 3.
- P3-AUTH-02: Khóa internal verify contract: user ID canonical, expiry, disabled/revoked session.
- P3-AUTH-03: Bổ sung test token thiếu/sai/hết hạn/revoked.
- P3-AUTH-04: Xác minh logout/logout-all làm progress route mất quyền ngay.
- P3-AUTH-05: Viết security test matrix cho identity spoofing (`X-User-ID` từ client phải bị strip).

Deliverables:

- Auth tests pass; tài liệu verify contract; bảng security cases; review bắt buộc PR backend progress của Dev 2 và middleware của Dev 5.

Không được:

- Đổi public Auth response mà không cập nhật OpenAPI cùng PR.
- Cho Study service tin trực tiếp header do client gửi.

### Dev 2 — Go backend Learning Progress

Ownership: `services/study/**` cho progress domain và migrations.

Nhiệm vụ:

- P3-BE-01: Thiết kế migration append-only cho hai bảng progress; không sửa migration đã chạy.
- P3-BE-02: Tạo model/repository/service/handler tách lớp rõ ràng.
- P3-BE-03: Implement POST transaction: verify owner, validate card thuộc set, insert session + card results atomically.
- P3-BE-04: Implement GET summary/history với pagination có giới hạn.
- P3-BE-05: Implement latest progress theo mode.
- P3-BE-06: Idempotency để retry không tạo session trùng.
- P3-BE-07: Unit tests cho validation, ownership, transaction rollback, duplicate key và pagination.
- P3-BE-08: Integration migration test trên empty DB và upgrade DB.

Deliverables:

- Go code sạch theo handler/service/repository; migration; tests; curl evidence qua gateway.

Không được:

- SQL trong handler; `userID == 0` bypass; tin `studySetId/cardId` mà không owner-check; trả toàn bộ history không phân trang.

### Dev 3 — Frontend Progress UI và API integration

Ownership: `apps/web/src/lib/api/**`, dashboard/progress UI; phối hợp Dev 4 tại learning boundary.

Nhiệm vụ:

- P3-WEB-01: Thay progress stub bằng typed API client thật sau khi OpenAPI freeze.
- P3-WEB-02: Thêm progress summary vào Study Detail/Dashboard: điểm gần nhất, số lần học, mode gần nhất.
- P3-WEB-03: Thêm history tối thiểu có loading/empty/error/retry.
- P3-WEB-04: Xử lý 401, 403, 404, 409/idempotency và validation message thống nhất.
- P3-WEB-05: Tạo shared components cần thiết, không lặp button/alert/loading patterns.
- P3-WEB-06: Frontend tests cho API mapping và trạng thái UI.

Deliverables:

- Không còn no-op trong `progressContract.ts`; build pass; video/screenshot hoặc test evidence cho reload persistence.

Không được:

- Tự định nghĩa response khác OpenAPI; dùng mock trong production flow; chạm thuật toán mode thuộc Dev 4 nếu chưa thống nhất.

### Dev 4 — Learning modes và Rust quiz engine

Ownership: `apps/web/src/features/learning/**`, `crates/quiz-core/**`.

Nhiệm vụ:

- P3-LEARN-01: Nối completion của 4 modes với callback/save progress thật.
- P3-LEARN-02: Gửi per-card results hợp lệ, tối đa 100 items; chống double-submit.
- P3-LEARN-03: Khôi phục trạng thái hợp lý khi save lỗi và cho phép retry idempotent.
- P3-RUST-01: Định nghĩa pure functions và golden test vectors cho seeded shuffle, score, question generation.
- P3-RUST-02: Implement trong `quiz-core`; property/unit tests, không I/O, không DB, không HTTP.
- P3-RUST-03: Benchmark với dataset 10/100/1.000/10.000 cards và so với baseline TypeScript/Go phù hợp.
- P3-RUST-04: Chỉ tích hợp qua WASM hoặc Go binding sau ADR của Dev 5; nếu lợi ích không rõ thì giữ Rust crate độc lập, chưa đưa vào runtime.

Deliverables:

- Bốn modes lưu progress thật; test vectors dùng chung; benchmark report; không còn debug stub.

Không được:

- Đưa Rust vào CRUD; tạo network service mới; dùng random không seed trong test; merge binding không có CI cho target tương ứng.

### Dev 5 — Integration, contract, gateway, CI và release gate

Ownership: `services/gateway/**`, `infra/**`, `packages/api-contracts/**`, CI, docs.

Nhiệm vụ:

- P3-INT-00: Chạy Phase 2 re-test gate trước khi mở merge Phase 3.
- P3-INT-01: Freeze OpenAPI v1.3 cho progress và error envelope trong ngày 1.
- P3-INT-02: Route progress endpoints qua authenticated proxy; strip identity header; request ID xuyên suốt.
- P3-INT-03: Thêm OpenAPI lint/contract check và generated-type drift check nếu khả thi.
- P3-INT-04: Bổ sung Docker health/readiness, migration và end-to-end script.
- P3-INT-05: Tạo ADR lựa chọn Rust integration: WASM, FFI, hay chưa tích hợp runtime.
- P3-INT-06: Chạy security A/B ownership, retry/idempotency và regression gate.
- P3-INT-07: Tổng hợp release evidence và PM GO/NO-GO.

Deliverables:

- OpenAPI chuẩn; gateway route; Docker/CI scripts; ADR; final gate report.

Không được:

- Merge contract-breaking PR riêng lẻ; tự sửa code domain lớn của dev khác; tuyên bố pass nếu thiếu log/lệnh tái hiện.

## 6. Thứ tự thực hiện và dependency

### Sprint 0 — Gate và contract (Ngày 1–2)

1. Dev 5 chạy Phase 2 gate; blocker thì dừng feature work liên quan.
2. Dev 1 + Dev 2 + Dev 5 khóa identity, error và progress API.
3. Dev 4 cung cấp test vectors/payload needs; Dev 3 review frontend usability.
4. Merge contract PR đầu tiên; mọi dev rebase từ `main`.

### Sprint 1 — Backend và UI skeleton (Ngày 3–6)

1. Dev 2 làm migration/repository/service/tests.
2. Dev 1 hoàn thiện auth/security tests độc lập.
3. Dev 3 làm typed client và UI states dựa trên OpenAPI đã khóa.
4. Dev 4 làm Rust pure core và sửa learning completion boundary.
5. Dev 5 làm gateway route, contract checks và Docker integration.

### Sprint 2 — End-to-end (Ngày 7–9)

1. Merge backend progress trước.
2. Dev 3 nối client + summary/history.
3. Dev 4 nối 4 modes + per-card results.
4. Dev 5 chạy E2E liên tục; Dev 1/2 xử lý security/ownership regressions.

### Sprint 3 — Hardening và gate (Ngày 10)

1. Fresh clone, fresh volume, migrations up.
2. Full tests/build/Docker/E2E.
3. Benchmark/ADR quyết định Rust runtime integration.
4. Fix P0/P1; cập nhật README và release note.
5. PM chỉ đánh dấu GO khi toàn bộ checklist có evidence.

## 7. Branch và PR plan

| Dev | Branch gợi ý | PR tối đa |
| --- | --- | --- |
| Dev 1 | `phase3/dev1-auth-security` | 2 PR nhỏ |
| Dev 2 | `phase3/dev2-learning-progress` | migration/model; service/API/tests |
| Dev 3 | `phase3/dev3-progress-ui` | client/types; UI/tests |
| Dev 4 | `phase3/dev4-learning-rust` | mode integration; Rust core/benchmark |
| Dev 5 | `phase3/dev5-integration` | contract/gateway; CI/gate/docs |

Quy tắc:

- Không push trực tiếp `main`.
- Rebase `main` trước khi bắt đầu mỗi ngày và trước khi request review.
- PR nên dưới 500 dòng thay đổi logic; lớn hơn phải giải thích hoặc tách.
- Mỗi PR cần: scope, contract/DB impact, commands đã chạy, output tóm tắt, manual evidence, risks/rollback.
- Ít nhất 1 reviewer; PR contract/security/migration cần đúng owner review.
- Không merge khi CI đỏ hoặc có P0/P1 chưa xử lý.

## 8. Merge order

1. Dev 5: OpenAPI/error/identity contract.
2. Dev 1: Auth verify/error compatibility.
3. Dev 2: migration + backend progress.
4. Dev 5: gateway routing + integration script.
5. Dev 3: API client + progress UI.
6. Dev 4: learning mode integration.
7. Dev 4/5: Rust core + benchmark + ADR; runtime binding chỉ merge nếu được duyệt.
8. Dev 5: final gate report/docs.

## 9. Test matrix bắt buộc

### Functional

- Lưu progress cho từng mode; reload thấy dữ liệu.
- Score 0, full score, empty set, 100 card results.
- Pagination boundary và latest-by-mode.
- Retry cùng idempotency key không duplicate.

### Security

- Không token, token sai, token hết hạn, revoked token.
- Client tự gửi `X-User-ID` phải bị bỏ.
- User A không xem/lưu progress set của User B.
- Card ID ngoài set bị reject và rollback toàn transaction.

### Reliability

- DB error giữa transaction không để lại half-written session.
- Double-click/double-submit không tạo duplicate.
- Gateway timeout trả error envelope và request ID.
- Migration từ empty DB và từ baseline Phase 2.

### Build/gate commands

```bash
go test ./...
go build ./...
cargo test --workspace
cargo fmt --check
cargo clippy --workspace -- -D warnings
npm --prefix apps/web ci
npm --prefix apps/web run build
docker compose -f infra/docker/docker-compose.yml up --build
```

Sau khi stack healthy: test health, auth lifecycle, Study/Folder regression, progress happy path, A/B ownership và 4 learning modes.

## 10. Rủi ro và biện pháp

| Rủi ro | Mức | Biện pháp |
| --- | --- | --- |
| Phase 2 chưa được chạy gate thật | P0 | Dev 5 gate trước feature merge |
| Contract drift giữa Go/TS/OpenAPI | P0 | contract-first + check trong CI |
| Progress ghi một phần | P0 | một DB transaction + rollback test |
| Cross-user data leak | P0 | owner scope trong query + A/B E2E |
| Double submit | P1 | idempotency key + unique index |
| Rust tăng độ phức tạp | P1 | pure crate, benchmark, ADR, feature flag |
| Conflict Dev 3/4 ở learning boundary | P1 | khóa callback/types trong contract PR |
| Migration extension bị hạn chế | P1 | test target DB; document fallback/no-index strategy |

## 11. Phase 3 GO/NO-GO checklist

- [ ] Phase 2 fresh-environment gate có evidence.
- [ ] Progress migrations empty/upgrade đều pass.
- [ ] Backend progress tests pass.
- [ ] Auth/identity/security tests pass.
- [ ] OpenAPI/Go/TypeScript contract đồng nhất.
- [ ] Bốn modes lưu dữ liệu thật; không stub/mock production.
- [ ] A/B ownership và spoof-header tests pass.
- [ ] Idempotency và transaction rollback pass.
- [ ] Frontend production build pass.
- [ ] Go build/test pass toàn workspace.
- [ ] Rust fmt/clippy/test pass.
- [ ] Docker Compose fresh build/start pass.
- [ ] Benchmark và ADR Rust được review.
- [ ] README, migration notes và rollback notes được cập nhật.

## 12. Công việc đầu tiên giao ngay

1. **Dev 5:** chạy Phase 2 gate và mở PR contract Phase 3.
2. **Dev 1:** chuẩn bị verify/error security matrix trên contract draft.
3. **Dev 2:** viết migration design + repository interfaces, chưa code handler trước khi contract freeze.
4. **Dev 3:** dựng progress UI states và review response shape, chưa hard-code API.
5. **Dev 4:** tạo golden test vectors cho shuffle/scoring/generation và chốt completion payload.

Điểm đồng bộ đầu tiên: cuối Ngày 2. Chỉ khi Phase 2 gate không còn P0 và OpenAPI v1.3 đã khóa thì team mới bước sang Sprint 1.
