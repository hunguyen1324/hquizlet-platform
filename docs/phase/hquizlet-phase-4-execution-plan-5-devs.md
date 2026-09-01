# HQuizlet Platform – Phase 4 Execution Plan (5 Developers)

## 1. Mục tiêu Phase 4

Phase 4 hoàn thiện bốn learning mode trên dữ liệu thật, đưa thuật toán học/kiểm tra về lớp domain có thể kiểm thử và benchmark, đồng thời biến Go `quiz` service thành cổng API duy nhất cho việc tạo bài và chấm điểm.

Kết quả cuối phase:

- Flashcards, Learn, Match, Test hoạt động end-to-end với study set thật.
- Rust `quiz-core` là đặc tả thuật toán chuẩn cho shuffle, sinh câu hỏi, scoring và match pair.
- Go `quiz` service cung cấp API ổn định; Phase 4 dùng Go port của thuật toán và bắt buộc khớp golden vectors từ Rust. Không dùng Rust FFI trong request path ở phase này.
- Frontend không tự tạo câu hỏi hoặc tự tính kết quả theo một contract khác backend.
- Kết quả hoàn thành tiếp tục lưu qua Progress API đã khóa ở Phase 3.
- CI, cross-language golden tests, benchmark và E2E đều xanh trước khi merge.

## 2. Quyết định kiến trúc

### 2.1 Luồng dữ liệu chuẩn

1. Frontend gọi Gateway với token người dùng.
2. Gateway xác thực, loại bỏ identity header giả và inject `X-User-ID`/`X-Request-ID`.
3. Go `quiz` service lấy study set/cards từ Study service bằng internal API.
4. Quiz service kiểm tra ownership, gọi domain engine để shuffle/generate/evaluate.
5. Frontend chỉ render model trả về và gửi câu trả lời để chấm.
6. Khi hoàn thành, frontend lưu session qua Progress API Phase 3 bằng `flashcardId`.

### 2.2 Rust và Go

- Rust `crates/quiz-core` là nguồn chuẩn cho hành vi thuật toán và golden vectors.
- Go `services/quiz/internal/engine` port cùng thuật toán trong Phase 4 để triển khai đơn giản, dễ vận hành và không phát sinh FFI/subprocess trong HTTP request.
- Rust và Go phải nhận cùng input, seed và trả cùng output chuẩn hóa.
- Chỉ nghiên cứu Rust FFI/WASM/service riêng ở Phase 5 sau khi benchmark chứng minh lợi ích rõ ràng.

### 2.3 API dự kiến

| Method | Endpoint | Mục đích |
| --- | --- | --- |
| `POST` | `/v1/study-sets/{studySetId}/quiz/generate` | Tạo deck/questions/pairs theo mode và seed |
| `POST` | `/v1/study-sets/{studySetId}/quiz/evaluate` | Chấm answers/matches và trả score/card results |

`generate` request gồm `mode`, `seed`, `limit`, `options`. Response có `mode`, `seed`, `items`, `contractVersion`. `evaluate` request gồm `mode`, `seed`, `answers`; response gồm `score`, `total`, `cardResults[]` với `flashcardId`, `correct`, `attempts`, `responseTimeMs`.

Không nhận `userId` trong body. Không cho frontend gửi đáp án đúng để backend tin trực tiếp. Mọi lỗi dùng envelope `{code,message,requestId,details?}`.

## 3. Phân công 5 developer

## Dev 1 – Contract, Gateway và Integration Owner

Vai trò: khóa contract trước khi các dev khác code, quản lý dependency và gate tích hợp.

### Công việc

- `[P4-CON-01]` Viết OpenAPI cho `quiz/generate` và `quiz/evaluate`.
- `[P4-CON-02]` Định nghĩa JSON schemas dùng chung: learning mode, seed, generated item, answer, match result, evaluation result và error envelope.
- `[P4-CON-03]` Viết contract examples/golden JSON cho cả bốn mode.
- `[P4-GW-01]` Thêm route Gateway tới Quiz service; giữ auth, spoofed-header stripping và request ID.
- `[P4-INT-01]` Chốt API nội bộ Quiz → Study để lấy cards theo ownership.
- `[P4-CI-01]` Thêm OpenAPI lint, Go/Rust tests, clippy, frontend tests, cross-language golden test và benchmark smoke vào CI.
- `[P4-DOC-01]` Cập nhật README, sequence flow, curl examples và báo cáo gate Phase 4.

### Deliverables

- OpenAPI hợp lệ và examples được test.
- Gateway routing tests cho 401/403/404/5xx/request ID.
- Contract matrix được cả Dev 2–5 xác nhận trước khi implementation merge.

### Definition of Done

- Không còn field/endpoint do frontend, Go và Rust tự định nghĩa riêng.
- CI fail nếu golden vector hoặc OpenAPI bị lệch.
- Contract merge trước code feature.

## Dev 2 – Rust `quiz-core` Owner

Vai trò: xây domain engine chuẩn, deterministic và có benchmark.

### Công việc

- `[P4-RUST-01]` Chuẩn hóa model thuần Rust, không phụ thuộc HTTP/DB.
- `[P4-RUST-02]` Implement Fisher–Yates shuffle bằng seeded RNG; cùng seed phải cho cùng output.
- `[P4-RUST-03]` Sinh Test questions, distractors không trùng, không lộ đáp án và xử lý deck nhỏ.
- `[P4-RUST-04]` Scoring cho Flashcards/Learn/Test, attempts và retry semantics.
- `[P4-RUST-05]` Match pair generation/evaluation; term và definition của cùng card phải giữ identity.
- `[P4-RUST-06]` Xuất golden vectors JSON cho input/output chuẩn.
- `[P4-RUST-07]` Property tests: determinism, no duplicate, score bounds, foreign card rejection.
- `[P4-RUST-08]` Benchmark 10/100/1.000/10.000 cards, lưu baseline và ngưỡng regression.

### Edge cases bắt buộc

- 0/1 card; duplicate definitions; Unicode/tiếng Việt; empty strings; limit lớn hơn deck.
- Seed biên (`0`, max `u64`); 100+ answers; retry đúng sau lần sai.
- Match subset phải chính là subset dùng để lưu `cardResults`.

### Definition of Done

- `cargo test --workspace` và `cargo clippy --workspace --all-targets -- -D warnings` PASS.
- Golden vectors versioned và không đổi ngầm.
- Benchmark report có số liệu, máy chạy và commit SHA.

## Dev 3 – Go Quiz Service Owner

Vai trò: triển khai API production, ownership, validation và Go port khớp Rust.

### Công việc

- `[P4-GO-01]` Tạo handler/service/repository-client theo clean architecture.
- `[P4-GO-02]` Implement Study service client với timeout, context cancellation và typed errors.
- `[P4-GO-03]` Enforce user authentication và A/B ownership cho mọi endpoint.
- `[P4-GO-04]` Port seeded shuffle, question generation, scoring và match logic từ Rust sang Go.
- `[P4-GO-05]` Chạy cùng golden vectors Rust; output chuẩn hóa phải giống 100%.
- `[P4-GO-06]` Validate mode/seed/limit/answers/card membership; không trả `err.Error()` ra client.
- `[P4-GO-07]` Thêm unit tests, handler tests và integration tests với Study service fake/real container.
- `[P4-GO-08]` Thêm metrics: request count, latency, error count theo endpoint/mode; log có request ID.

### Definition of Done

- `go test`/`go build` toàn workspace PASS.
- User B không tạo/chấm quiz của study set User A.
- Timeout Study service trả lỗi chuẩn, không treo request.
- Golden vectors Go/Rust khớp hoàn toàn.

## Dev 4 – Frontend Flashcards & Learn Owner

Vai trò: hoàn thiện UX và tích hợp API thật cho hai mode Flashcards/Learn.

### Công việc

- `[P4-FE-FLASH-01]` Flashcards: flip, next/previous, shuffle bằng seed backend, starred filter, keyboard/mobile accessibility.
- `[P4-FE-FLASH-02]` Hoàn thành deck đúng một lần; restart tạo seed/`startedAt` mới.
- `[P4-FE-LEARN-01]` Learn: answer input, normalization hiển thị, retry queue, attempts chính xác.
- `[P4-FE-LEARN-02]` Retry đúng phải cập nhật kết quả cuối và score; không giữ kết quả sai cũ.
- `[P4-FE-INT-01]` Dùng `quiz/generate`/`quiz/evaluate`; không tự chấm theo logic riêng.
- `[P4-FE-PROG-01]` Lưu Progress API với `flashcardId`; handle saving/saved/retryable/conflict.
- `[P4-FE-TEST-01]` Component/API tests cho loading, empty, error, success, restart và double-submit.

### Definition of Done

- Không có mock/no-op/stub trong production client.
- Refresh/restart không gây idempotency 409 ngoài retry cùng session.
- Keyboard, mobile và screen-reader smoke tests PASS.

## Dev 5 – Frontend Match, Test & E2E Owner

Vai trò: hoàn thiện Match/Test và làm owner cho E2E toàn Phase 4.

### Công việc

- `[P4-FE-MATCH-01]` Render đúng subset/pairs backend trả; không shuffle lần hai làm lệch identity.
- `[P4-FE-MATCH-02]` Timer, mismatch attempts, completion và restart state.
- `[P4-FE-TEST-02]` Render câu hỏi/distractors backend; submit/evaluate và review đáp án.
- `[P4-FE-TEST-03]` Chống double-submit, stale state và score client khác score server.
- `[P4-E2E-01]` E2E cho cả bốn mode: generate → interact → evaluate → save progress → history hiển thị.
- `[P4-E2E-02]` Docker fresh-volume test với PostgreSQL; user A/B ownership và service restart.
- `[P4-E2E-03]` Network/error matrix: Quiz offline, Study timeout, 401, 403, 422, 500, retry.
- `[P4-QA-01]` Lập Phase 4 release checklist và evidence links.

### Definition of Done

- Match lưu đúng card vừa hiển thị.
- Test review và server score luôn khớp.
- E2E chạy trong CI hoặc có script tái lập một lệnh với evidence rõ ràng.

## 4. Lịch thực hiện 4 tuần

| Tuần | Dev 1 | Dev 2 | Dev 3 | Dev 4 | Dev 5 |
| --- | --- | --- | --- | --- | --- |
| Tuần 1 | Freeze OpenAPI/examples | Rust models, RNG, vectors v0 | Quiz skeleton, Study client | UX/state audit Flash/Learn | UX/state audit Match/Test, E2E skeleton |
| Tuần 2 | Gateway routes, contract tests | Generate/scoring/match logic | Go port + validation | Flashcards/Learn API integration | Match/Test API integration |
| Tuần 3 | CI cross-language gate | Property tests + benchmarks | Handler/integration/ownership tests | Component tests, accessibility | Docker/E2E/error matrix |
| Tuần 4 | Contract audit, docs, release gate | Regression fixes | Observability/performance fixes | UX/regression fixes | Full E2E evidence, release report |

## 5. Dependency và thứ tự merge

1. Dev 1 merge contract/examples trước.
2. Dev 2 merge Rust models + golden vectors.
3. Dev 3 merge Go engine/API chỉ khi golden vectors khớp.
4. Dev 4 và Dev 5 rebase theo contract đã khóa rồi tích hợp frontend.
5. Dev 5 chạy E2E; Dev 1 kiểm tra toàn bộ gate và lập GO/NO-GO.

Không merge song song các thay đổi cùng OpenAPI hoặc shared types. Mọi thay đổi contract sau freeze cần PR riêng, migration note và approval Dev 1 + dev bị ảnh hưởng.

## 6. Quy tắc branch và PR

- Branch: `phase4/dev{n}-{task-id}-{short-name}`.
- Một PR chỉ nên giải quyết một nhóm task có cùng contract.
- PR bắt buộc ghi task ID, contract impact, test commands, evidence và rollback plan.
- Không push thẳng `main`; dùng squash merge sau khi branch protection xanh.
- Không dùng token trong URL, commit, log, screenshot hoặc tài liệu.

## 7. Gate bắt buộc trước Phase 5

### Contract gate

- OpenAPI syntax/lint PASS; examples parse được.
- Frontend/Go/Rust dùng đúng `flashcardId`, mode enum, seed và error envelope.

### Build/test gate

- Go test/build toàn bộ 6 modules PASS.
- Rust test/clippy PASS.
- Frontend `npm ci`, test, production build PASS.
- Cross-language golden vectors PASS.

### Integration gate

- Docker fresh-volume PASS.
- PostgreSQL migrations up/down/upgrade evidence PASS.
- Bốn mode E2E và progress history PASS.
- User A/B ownership PASS; spoofed identity test PASS.
- Timeout/retry/5xx không lộ lỗi nội bộ.

### Performance gate

- Benchmark 10/100/1.000/10.000 cards có baseline.
- Không có regression vượt 15% so với baseline đã duyệt.
- API p95 mục tiêu dưới 200 ms cho deck 1.000 cards trong môi trường test đã ghi rõ.

## 8. Definition of Done chung

Một task chỉ Done khi code, tests, docs, contract và evidence đều có trong PR; không còn production stub; không bỏ qua lỗi bằng fallback giả; không dùng hai nguồn scoring; và CI của merge commit trên `main` xanh.

Phase 4 chỉ được đánh dấu **GO** khi tất cả gate bắt buộc xanh. Nếu Docker/PostgreSQL/E2E/benchmark chưa có evidence thì kết luận vẫn là **NO-GO**, dù unit tests đã PASS.
