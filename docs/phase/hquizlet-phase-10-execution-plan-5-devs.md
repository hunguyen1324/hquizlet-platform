# HQuizlet Platform — Phase 10 Execution Plan (5 Developers)

Ngay tao: 2026-09-02
Vai tro: PM Gate Document — Phase 10

---

## 1. Muc Tieu Phase 10

Phase 10 la phase nang cap noi dung hoc tap lon nhat tu Phase 2: doi ten "the
ghi nho" thanh **"the hoc"**, bo sung cac loai noi dung (Quiz, Ngu phap) trong
cung mot Study Set, them am thanh dua tren ngon ngu thuat ngu/dinh nghia, che
do cong khai/rieng tu cho the hoc, mo rong loai cau hoi quiz, va them chuc
nang import the hoc va quiz tu file Excel.

Ket qua cuoi phase:

1. UI doi toan bo "Flashcard / The ghi nho" → **"The hoc"** nhat quan.
2. Creator chon **loai noi dung**: The hoc | Quiz | Ngu phap — giong hquizlet goc.
3. Moi thuat ngu va dinh nghia co **ngon ngu rieng**; TTS server tra audio
   dung ngon ngu tuong ung.
4. **Am thanh cho cau hoi quiz**: URL audio dinh kem cau hoi, audio phat khi
   quiz bat dau cau do.
5. **Che do cong khai / rieng tu**: creator bat/tat, gateway enforce.
6. **5 loai cau hoi quiz**: multiple_choice, true_false, written, paragraph,
   sorting — giong hquizlet.
7. **Import the hoc tu Excel** (`.xlsx`): cot Term, Definition, Example, Hint,
   Image URL; batch upsert.
8. **Import quiz tu Excel** (`.xlsx`): cot Question, Type, Option A–D, Correct,
   Time, Audio URL; batch insert.
9. OpenAPI, migrations, backend tests, frontend tests, Docker fresh-volume E2E
   deu xanh truoc khi danh dau GO.

---

## 2. Baseline Va Dieu Kien Bat Dau

### 2.1 Trang thai repo sau Phase 9

- `services/study` co CRUD study set (chi "flashcard" content_type) va
  flashcard, chua co quiz/grammar content trong study service.
- `services/quiz` co engine Generate/Evaluate voi 4 mode (flashcards, learn,
  test, match), chua luu persistent QuizQuestion, chua co audio per question.
- `services/file` co presign-upload, metadata PostgreSQL, MinIO/S3 backend.
- Frontend co study set editor chi hien thi the hoc (flashcard list), chua co
  tab Quiz / Ngu phap.
- Model `StudySet` trong platform chua co truong `content_type`, `visibility`,
  `term_language`, `definition_language`.
- Chua co Excel import endpoint o gateway/study/quiz.

### 2.2 Dieu kien bat dau Phase 10

- [ ] Phase 9 release gate da dong (fresh-volume E2E co evidence).
- [ ] `services/file` presign-upload hoat dong va co test xanh.
- [ ] `services/study` bulk flashcard endpoint on dinh.
- [ ] `services/quiz` generate/evaluate test xanh.
- [ ] Gateway `/internal/auth/verify` on dinh.

---

## 3. Pham Vi

### 3.1 In Scope

#### 3.1.1 Doi ten "The ghi nho" → "The hoc" (Rename)
- Tat ca label UI tieng Viet: "The ghi nho", "Flashcard", "Tao the ghi nho" →
  "The hoc", "Tao the hoc".
- API response field `flashcards` van giu nguyen (backward compat), chi doi
  label UI.
- OpenAPI description doi tuong ung.

#### 3.1.2 Loai noi dung Study Set (Content Type)
- `StudySet.content_type`: enum `flashcard | quiz | grammar` (da co o hquizlet
  goc), them vao model Go va migration.
- Editor hien thi 3 tab: **The hoc | Quiz | Ngu phap** dua theo content_type.
- Chuyen sang tab khac: warn user neu co data o tab hien tai se mat.

#### 3.1.3 Ngon ngu Thuat Ngu / Dinh Nghia + Am Thanh
- `StudySet` them 2 truong: `term_language` va `definition_language` (BCP-47:
  `vi-VN`, `en-US`, `ja-JP`, v.v.).
- TTS endpoint (Go): `GET /v1/tts?text=...&lang=vi-VN` → audio stream (MP3).
  Backend goi Google Cloud TTS hoac eSpeaker; co cache Redis TTL 24h.
- Frontend the hoc: nut loa phat am term (dung `term_language`) va definition
  (`definition_language`).
- Language selector trong editor (dropdown chon tu danh sach co dinh).

#### 3.1.4 Am Thanh Cho Cau Hoi Quiz
- `QuizQuestion` them truong `audio_url TEXT` (da co trong hquizlet schema,
  them vao model Go va migration platform).
- Editor quiz: moi cau hoi co upload audio (dung presign-upload cua
  `services/file`) hoac nhap URL tay.
- Frontend quiz mode: khi bat dau cau, auto-play audio neu `audio_url` ton tai.

#### 3.1.5 Che Do Cong Khai / Rieng Tu
- `StudySet` them truong `visibility` enum `public | private` (default:
  `public`).
- Gateway enforce: the hoc `private` chi cho phep owner xem/hoc; nguoi khac
  tra 403.
- Frontend: toggle cong khai/rieng tu trong editor va trang detail.
- Dashboard: icon khoa hien thi the hoc rieng tu.

#### 3.1.6 Loai Cau Hoi Quiz (5 loai)
- `QuizQuestion.question_type` enum:
  - `multiple_choice`: 4 dap an, chon 1.
  - `true_false`: dung/sai.
  - `written`: nhap van ban tu do.
  - `paragraph`: doan van + cau hoi con (sub_questions JSON).
  - `sorting`: sap xep dap an theo thu tu dung.
- `QuizQuestionOption` table trong study service: `id, question_id, text,
  position, is_correct`.
- Quiz engine Evaluate mo rong de xu ly 5 loai.
- Frontend editor: moi loai cau hoi render UI nhap lieu tuong ung.
- Frontend quiz mode: render 5 loai cau hoi tuong ung.

#### 3.1.7 Import The Hoc Tu Excel
- Endpoint: `POST /v1/study-sets/{id}/import/flashcards`
  - Accept: `multipart/form-data`, file `.xlsx`.
  - Cot bat buoc: `Term`, `Definition`.
  - Cot tuy chon: `Example`, `Hint`, `Synonyms`, `Image URL`.
  - Validate: max 500 rows, co header row, bao loi o row cu the.
  - Upsert batch vao flashcard table (them moi, khong xoa card hien co).
- Frontend: nut "Nhap tu Excel" trong editor the hoc, hien thi preview truoc
  khi luu, bao loi theo tung row.
- Cung cap template Excel de tai ve.

#### 3.1.8 Import Quiz Tu Excel
- Endpoint: `POST /v1/study-sets/{id}/import/quiz`
  - Accept: `multipart/form-data`, file `.xlsx`.
  - Cot: `Question`, `Type`, `Option A`, `Option B`, `Option C`, `Option D`,
    `Correct Answer`, `Time (s)`, `Audio URL`, `Answer Explanation`.
  - Type map: `MC`=multiple_choice, `TF`=true_false, `WR`=written,
    `PG`=paragraph, `SO`=sorting.
  - Validate loai cau hoi, dap an hop le, max 200 rows.
  - Replace toan bo cau hoi hien co cua study set (transaction).
- Frontend: nut "Nhap tu Excel" trong tab Quiz editor.
- Cung cap template Excel de tai ve.

### 3.2 Out of Scope

- Nhap tu Google Sheets truc tiep (chi ho tro file Excel tai ve).
- TTS production billing/quota management (Phase 10 dung key dev/test).
- Khoa noi dung tra phi (da co Phase 8).
- Quiz mode realtime/live (Phase 6).
- Grammar editor (Phase 10 chi tao skeleton, editor day du Phase 11).
- Mobile app.

---

## 4. Team 5 Dev Va Ownership Phase 10

| Dev | Role | Ownership chinh | Ket qua Phase 10 |
| --- | --- | --- | --- |
| Dev 1 | Backend Go — Auth/TTS | `services/auth/**`, TTS endpoint moi | TTS endpoint + cache, language list API |
| Dev 2 | Backend Go — Study | `services/study/**` | content_type, visibility, language fields, Excel import flashcard/quiz, QuizQuestion model |
| Dev 3 | Frontend — Editor & Content Type | `apps/web/src/features/study-sets/**` | Tab The hoc/Quiz/Ngu phap, language selector, visibility toggle, import Excel UI |
| Dev 4 | Frontend — Learning & Quiz Modes | `apps/web/src/features/learning/**` | Am thanh TTS/audio trong the hoc va quiz, 5 loai cau hoi render, audio auto-play |
| Dev 5 | Fullstack/Integration | `services/gateway/**`, `infra/**`, `packages/api-contracts/**` | OpenAPI 1.8.0, gateway enforce visibility, route TTS/import, Docker E2E gate |

---

## 5. Kien Truc Ky Thuat

### 5.1 Migration Plan (Study Service)

```sql
-- 001: them truong vao study_set
ALTER TABLE study_set
  ADD COLUMN content_type TEXT NOT NULL DEFAULT 'flashcard'
    CHECK (content_type IN ('flashcard','quiz','grammar')),
  ADD COLUMN term_language TEXT NOT NULL DEFAULT 'en-US',
  ADD COLUMN definition_language TEXT NOT NULL DEFAULT 'en-US',
  ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'
    CHECK (visibility IN ('public','private'));

-- 002: quiz_question (native trong study service)
CREATE TABLE quiz_question (
  id            BIGSERIAL PRIMARY KEY,
  study_set_id  BIGINT NOT NULL REFERENCES study_set(id) ON DELETE CASCADE,
  position      INT NOT NULL,
  question_text TEXT NOT NULL DEFAULT '',
  question_type TEXT NOT NULL CHECK (question_type IN
    ('multiple_choice','true_false','written','paragraph','sorting')),
  correct_answer TEXT,
  time_in_seconds INT,
  audio_url     TEXT,
  answer_explanation TEXT,
  paragraph_text TEXT,
  sub_questions  JSONB,
  tags          TEXT[] NOT NULL DEFAULT '{}'
);

-- 003: quiz_question_option
CREATE TABLE quiz_question_option (
  id          BIGSERIAL PRIMARY KEY,
  question_id BIGINT NOT NULL REFERENCES quiz_question(id) ON DELETE CASCADE,
  text        TEXT NOT NULL DEFAULT '',
  position    INT NOT NULL,
  is_correct  BOOLEAN NOT NULL DEFAULT false
);
```

### 5.2 TTS Service (Dev 1)

```
GET /v1/tts?text={encoded}&lang={bcp47}
→ audio/mpeg stream (MP3)

Cache key: sha256(text + lang) → Redis TTL 24h
Backend: Google Cloud TTS (env: GOOGLE_TTS_KEY) hoac espeak fallback
```

### 5.3 Excel Import Flow

```
Client upload .xlsx → Gateway (auth check) → Study service
Study service:
  1. Doc file voi go-excelize.
  2. Validate header row, max rows, data types.
  3. Neu loi: tra 422 voi mang errors [{row, field, reason}].
  4. Neu ok: upsert batch trong transaction.
  5. Tra 200 voi summary {imported, skipped, errors}.
```

### 5.4 Visibility Enforcement (Dev 5 — Gateway)

```
GET /v1/study-sets/{id}  →  Study service tra StudySet voi visibility.
Gateway KHONG enforce visibility (study service enforce vi co user context).
Study service:
  - Neu visibility = 'public': tra data cho moi nguoi.
  - Neu visibility = 'private': chi tra data khi X-User-ID == owner.
  - Nguoi khac: 403 Forbidden.
```

### 5.5 OpenAPI Version

Phase 10 tang `openapi.yaml` tu `1.7.0` len `1.8.0`.
Cac endpoint moi/thay doi:
- `GET /v1/tts`
- `POST /v1/study-sets/{id}/import/flashcards`
- `POST /v1/study-sets/{id}/import/quiz`
- `GET /v1/study-sets/{id}` — them fields `contentType`, `termLanguage`,
  `definitionLanguage`, `visibility`.
- `POST /v1/study-sets/{id}/quiz-questions` (CRUD).
- `GET /v1/languages` — danh sach ngon ngu ho tro TTS.

---

## 6. Cong Viec Chi Tiet Theo Dev

### Dev 1 — Backend Go: TTS & Language

| Ma task | Viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| P10-D1-01 | TTS endpoint `GET /v1/tts` | Audio MP3 stream theo lang | Google TTS key env |
| P10-D1-02 | Redis cache cho TTS | Cache theo sha256(text+lang), TTL 24h | Redis co san |
| P10-D1-03 | `GET /v1/languages` endpoint | JSON danh sach {code, name, flag} | Khong |
| P10-D1-04 | Test TTS cache hit/miss | Unit test | Khong |
| P10-D1-05 | Fallback khi TTS key thieu | Log warn, tra 503 ro rang | Khong |

Fallback khi cho:
- Viet curl collection TTS.
- Espeak fallback neu khong co Google key.

### Dev 2 — Backend Go: Study Content & Import

| Ma task | Viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| P10-D2-01 | Migration them content_type, term_language, definition_language, visibility vao study_set | Migration SQL | Khong |
| P10-D2-02 | Migration tao quiz_question + quiz_question_option | Migration SQL | Khong |
| P10-D2-03 | Update model.go them cac truong moi + QuizQuestion/Option struct | Go model | Khong |
| P10-D2-04 | CRUD QuizQuestion (Create, BulkSave, Delete) | Handler/Service/Repo | Model done |
| P10-D2-05 | Visibility enforcement trong study_set handler | 403 cho private set | Khong |
| P10-D2-06 | Import flashcard tu Excel (POST /import/flashcards) | Endpoint + service | go-excelize |
| P10-D2-07 | Import quiz tu Excel (POST /import/quiz) | Endpoint + service | go-excelize |
| P10-D2-08 | Template Excel endpoint `GET /v1/templates/flashcard.xlsx` | Static file serve | Khong |
| P10-D2-09 | Template Excel endpoint `GET /v1/templates/quiz.xlsx` | Static file serve | Khong |
| P10-D2-10 | Test import flashcard (happy path + error rows) | Unit test | Khong |
| P10-D2-11 | Test import quiz (5 loai question type) | Unit test | Khong |

Fallback khi cho:
- Seed data quiz question.
- SQL index tren study_set_id cua quiz_question.

### Dev 3 — Frontend: Editor & Content Type

| Ma task | Viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| P10-D3-01 | Doi label "Flashcard/The ghi nho" → "The hoc" toan bo UI | Label changes | Khong |
| P10-D3-02 | Content type selector khi tao study set (The hoc / Quiz / Ngu phap) | UI component | API contract Dev 5 |
| P10-D3-03 | Editor tab The hoc (hien tai), tab Quiz, tab Ngu phap (skeleton) | Tab layout | Khong |
| P10-D3-04 | Tab Quiz editor: form nhap question theo tung loai (5 loai) | Quiz editor UI | QuizQuestion API Dev 2 |
| P10-D3-05 | Language selector trong editor (term lang, definition lang) | Dropdown UI | Language API Dev 1 |
| P10-D3-06 | Visibility toggle (cong khai / rieng tu) trong editor va detail | Toggle UI | API contract |
| P10-D3-07 | Import Excel the hoc: upload, preview table, confirm | Import UI | Import API Dev 2 |
| P10-D3-08 | Import Excel quiz: upload, preview, confirm | Import UI | Import API Dev 2 |
| P10-D3-09 | Tai template Excel (link download) | Download link | Template endpoint Dev 2 |
| P10-D3-10 | Dashboard: icon khoa cho the hoc rieng tu | Badge/icon | Khong |

Fallback khi cho:
- Skeleton loading cho quiz editor.
- Error state khi import bi loi row.

### Dev 4 — Frontend: Learning & Audio

| Ma task | Viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| P10-D4-01 | Nut loa phat am term trong the hoc (goi TTS API) | Audio button | TTS API Dev 1 |
| P10-D4-02 | Nut loa phat am definition trong the hoc | Audio button | TTS API Dev 1 |
| P10-D4-03 | Auto-play audio_url khi bat dau cau hoi quiz | Audio player | API |
| P10-D4-04 | Render multiple_choice (4 lua chon) | Quiz mode UI | QuizQuestion data |
| P10-D4-05 | Render true_false (Dung/Sai) | Quiz mode UI | QuizQuestion data |
| P10-D4-06 | Render written (nhap van ban) | Quiz mode UI | QuizQuestion data |
| P10-D4-07 | Render paragraph (doc doan + tra loi cau con) | Quiz mode UI | QuizQuestion data |
| P10-D4-08 | Render sorting (keo tha sap xep) | Quiz mode UI | QuizQuestion data |
| P10-D4-09 | Scoring cho 5 loai cau hoi (local) | Logic | Evaluate API |
| P10-D4-10 | Empty state khi study set la private ma user khong phai owner | Error UI | API 403 |

Fallback khi cho:
- Keyboard navigation cho quiz mode.
- Mobile layout quiz.

### Dev 5 — Integration, OpenAPI, Gateway

| Ma task | Viec | Output | Phu thuoc |
| --- | --- | --- | --- |
| P10-D5-01 | Cap nhat OpenAPI 1.7.0 → 1.8.0: cac endpoint moi | openapi.yaml | Input Dev 1/2 |
| P10-D5-02 | Gateway route `/v1/tts` → Auth service (Dev 1 port) | Gateway Go | Dev 1 ready |
| P10-D5-03 | Gateway route `/v1/study-sets/{id}/import/*` voi auth | Gateway Go | Dev 2 ready |
| P10-D5-04 | Gateway route `/v1/languages` | Gateway Go | Dev 1 ready |
| P10-D5-05 | Gateway route `/v1/templates/*` (no auth) | Gateway Go | Dev 2 ready |
| P10-D5-06 | Cap nhat API client TypeScript cho cac endpoint moi | Frontend types | OpenAPI |
| P10-D5-07 | Them `go-excelize` vao go.mod cua study service | Dep update | Khong |
| P10-D5-08 | Docker Compose: them GOOGLE_TTS_KEY env placeholder | docker-compose.yml | Khong |
| P10-D5-09 | Integration checklist Phase 10 | Checklist doc | Tat ca |
| P10-D5-10 | Phase 10 release gate review | Gate report | Tat ca PR |

Fallback khi cho:
- README: them muc TTS setup.
- PR template update cho Phase 10.

---

## 7. Branch Strategy Phase 10

| Dev | Branch | Merge vao |
| --- | --- | --- |
| Dev 1 | `feature/phase10-tts-language` | `main` qua PR |
| Dev 2 | `feature/phase10-study-content-import` | `main` qua PR |
| Dev 3 | `feature/phase10-editor-ui` | `main` qua PR |
| Dev 4 | `feature/phase10-learning-audio-quiz` | `main` qua PR |
| Dev 5 | `feature/phase10-integration` | `main` qua PR |

Quy dinh giong Phase 2–9:
- Rebase `origin/main` dau moi ngay.
- PR < 500 dong diff.
- PR co migration/API phai tag Dev 5.
- Sau merge, owner xoa branch cu.

---

## 8. Lich Lam Viec (5 Ngay)

### Ngay 1 — Contract & Migration

| Owner | Viec |
| --- | --- |
| Dev 5 | Draft OpenAPI 1.8.0, cap nhat contract TTS + import + content_type |
| Dev 1 | Xay dung TTS endpoint skeleton + language list |
| Dev 2 | Viet 2 migration SQL, cap nhat model.go |
| Dev 3 | Doi label UI, tao content type selector skeleton |
| Dev 4 | Tao audio player component (reuse cho TTS + audio_url) |

### Ngay 2–3 — Feature Song Song

| Owner | Viec |
| --- | --- |
| Dev 1 | TTS cache Redis, test, fallback |
| Dev 2 | CRUD QuizQuestion, visibility enforce, import flashcard |
| Dev 3 | Tab editor (The hoc/Quiz/Ngu phap), language selector, visibility toggle |
| Dev 4 | 5 loai cau hoi render, TTS button trong the hoc |
| Dev 5 | Gateway routes, API client types, go-excelize dep |

### Ngay 4 — Import & Audio Integration

| Owner | Viec |
| --- | --- |
| Dev 1 | Fix TTS bugs tu integration |
| Dev 2 | Import quiz tu Excel, template endpoints |
| Dev 3 | Import Excel UI (upload + preview), tai template |
| Dev 4 | Audio auto-play quiz, scoring 5 loai |
| Dev 5 | Docker Compose env TTS, full integration checklist |

### Ngay 5 — Phase Gate

| Owner | Viec |
| --- | --- |
| PM/Tech Lead | Review gate |
| Dev 5 | Docker + OpenAPI + checklist pass |
| Tat ca | Fix blocker trong ownership |

---

## 9. Definition of Done Phase 10

Mot task chi duoc xem la Done khi:
1. Code nam dung ownership.
2. Build/test lien quan pass.
3. API change cap nhat OpenAPI 1.8.0.
4. Docker Compose khong fail.
5. UI khong con placeholder neu task la user-facing.
6. Co test manual note trong PR.
7. Branch da rebase voi `main` moi nhat truoc merge.

Phase 10 chi qua gate khi:

| Hang muc | Dieu kien pass |
| --- | --- |
| Label UI | Khong con "Flashcard/The ghi nho" nao trong flow chinh |
| Content Type | Tao study set chon The hoc/Quiz/Ngu phap, luu dung DB |
| TTS | Nut loa phat am term/definition trong the hoc dung ngon ngu tuong ung |
| Visibility | Study set private tra 403 cho nguoi khong phai owner |
| Quiz Editor | 5 loai cau hoi tao duoc trong editor, luu DB |
| Quiz Audio | Cau hoi co audio_url phat am tu dong khi bat dau |
| Import Flashcard | Upload file .xlsx hop le → the hoc xuat hien trong study set |
| Import Quiz | Upload file .xlsx hop le → cau hoi xuat hien trong quiz editor |
| Template | Tai duoc file template flashcard.xlsx va quiz.xlsx |
| Docker | `docker compose up --build` pass tu repo root |
| OpenAPI | `openapi.yaml` version 1.8.0 khop backend va frontend types |
| Tests | `go test ./...` xanh trong services/study va services/auth |

---

## 10. Checklist Integration Flow Phase 10

```bash
# 1. Docker up
docker compose -f infra/docker/docker-compose.yml up --build

# 2. Health
curl http://localhost:8080/healthz/services

# 3. Auth
# Register + Login → lay JWT

# 4. Tao study set loai Quiz, visibility private
curl -X POST http://localhost:8080/v1/study-sets \
  -H "Authorization: Bearer $JWT" \
  -d '{"title":"Test","contentType":"quiz","visibility":"private"}'

# 5. Kiem tra user khac bi 403
# Login user khac → GET /v1/study-sets/{id} → ky vong 403

# 6. Import flashcard tu Excel
curl -X POST http://localhost:8080/v1/study-sets/{id}/import/flashcards \
  -H "Authorization: Bearer $JWT" \
  -F "file=@flashcard_template.xlsx"

# 7. Import quiz tu Excel
curl -X POST http://localhost:8080/v1/study-sets/{id}/import/quiz \
  -H "Authorization: Bearer $JWT" \
  -F "file=@quiz_template.xlsx"

# 8. TTS
curl "http://localhost:8080/v1/tts?text=Hello&lang=en-US" --output test.mp3

# 9. Language list
curl http://localhost:8080/v1/languages

# 10. Frontend flow: tao the hoc → chon ngon ngu → click loa → nghe am thanh
# 11. Frontend flow: tao quiz → 5 loai cau hoi → luu → vao che do hoc → phat am
# 12. Frontend flow: import Excel → preview → confirm → kiem tra data
```

---

## 11. Excel Template Spec

### Flashcard Template (`flashcard_template.xlsx`)

| Term | Definition | Example | Hint | Synonyms | Image URL |
| --- | --- | --- | --- | --- | --- |
| hello | xin chao | Hello, how are you? | (tuy chon) | (tuy chon) | https://... |

- Dong 1: header (bat buoc dung ten cot chinh xac, case-insensitive).
- `Term` va `Definition` bat buoc; cac cot khac co the bo trong.
- Max 500 rows (khong tinh header).

### Quiz Template (`quiz_template.xlsx`)

| Question | Type | Option A | Option B | Option C | Option D | Correct Answer | Time (s) | Audio URL | Answer Explanation |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2+2=? | MC | 3 | 4 | 5 | 6 | B | 30 | | So tu 4 |
| Troi xanh? | TF | | | | | True | 15 | | |
| Dich "cat" sang tieng Viet | WR | | | | | meo | 60 | | Con vat |

- Type: `MC` (multiple_choice), `TF` (true_false), `WR` (written), `PG`
  (paragraph), `SO` (sorting).
- `MC`: yeu cau co it nhat 2 Option, `Correct Answer` la "A"/"B"/"C"/"D".
- `TF`: `Correct Answer` la "True" hoac "False".
- `WR`: `Correct Answer` la chuoi van ban.
- `SO`: Options la cac phan tu, `Correct Answer` la thu tu dung, e.g. "B,A,D,C".
- `PG`: cau hoi phuc hop, `Correct Answer` la JSON hoac plain text.
- Max 200 rows.

---

## 12. Risk Va Mitigation

| Rui ro | Xac suat | Muc do | Mitigation |
| --- | --- | --- | --- |
| Google TTS quota vuot (dev) | Trung binh | Thap | Dung espeak fallback; cache Redis 24h giam so luong goi |
| Excel import row loi lam mat data hien co | Cao | Cao | Flashcard import la upsert-only (khong xoa); quiz import dung transaction rollback toan bo neu co loi |
| 5 loai cau hoi quiz lam phuc tap engine Evaluate | Trung binh | Trung binh | Giu logic evaluate local frontend; backend chi luu QuizQuestion, khong thay doi engine phase nay |
| Frontend tab content type lam lech state editor | Trung binh | Trung binh | Warn user khi chuyen tab co data; chua cho phep chuyen sau khi da luu |
| OpenAPI 1.8.0 lech voi phase truoc | Thap | Cao | Dev 5 dong bang contract ngay 1; tat ca backend/frontend PR phai tag Dev 5 |

---

## 13. File Ownership Matrix Phase 10

| Path | Owner | Reviewer bat buoc |
| --- | --- | --- |
| `services/auth/internal/tts/**` (moi) | Dev 1 | Dev 1, Dev 5 |
| `services/study/internal/model/quiz_question.go` (moi) | Dev 2 | Dev 2, Dev 5 |
| `services/study/internal/*/quiz_*` (moi) | Dev 2 | Dev 2 |
| `services/study/internal/*/import_*` (moi) | Dev 2 | Dev 2, Dev 5 |
| `services/study/internal/migration/` | Dev 2 | Dev 2, Dev 5 |
| `apps/web/src/features/study-sets/**` | Dev 3 | Dev 3, Dev 4 neu cham learning |
| `apps/web/src/features/learning/**` | Dev 4 | Dev 4, Dev 3 |
| `apps/web/src/components/audio/**` (moi) | Dev 4 | Dev 4 |
| `apps/web/src/lib/api/**` | Dev 5 | Dev 5, Dev 3 |
| `apps/web/src/types/**` | Dev 5 | Dev 3, Dev 4 |
| `services/gateway/**` | Dev 5 | Dev 5 |
| `packages/api-contracts/openapi.yaml` | Dev 5 | Dev 1/2/3/4 lien quan |
| `infra/**` | Dev 5 | Dev 5 |

---

## 14. Quy Tac Blocking Phase 10

| Bi block | Lam ngay | Bao ai |
| --- | --- | --- |
| TTS key chua co | Dung text-to-speech browser Web API lam fallback tam | Dev 1 + Dev 4 |
| Excel import chua xong | Mock response `{imported:0, errors:[]}` cho UI dev | Dev 2 + Dev 3 |
| QuizQuestion API chua xong | Hard-code 2–3 cau hoi fake trong `lib/mock` (chi dev mode) | Dev 4 + Dev 5 |
| OpenAPI 1.8.0 chua chot | Dung draft comment trong TypeScript types | Dev 3/4 + Dev 5 |
| Visibility 403 chua enforce | Bao PM, khong release feature den khi fix | Dev 2 + Dev 5 |

---

## 15. Ket Luan

Phase 10 dua HQuizlet Platform len ngang tam noi dung cua hquizlet goc ve:
- Ten thuong hieu the hoc nhat quan.
- Da dang loai noi dung (The hoc, Quiz, Ngu phap).
- Am thanh co nguon goc ngon ngu chinh xac.
- Quyen rieng tu ro rang.
- Cong cu import han che rào can nhap lieu.

Thu tu uu tien:
1. Dev 5 chot OpenAPI 1.8.0 va migration SQL — ngay 1.
2. Dev 2 hoan thanh migration + model + visibility — truoc ngay 3.
3. Dev 1 TTS endpoint + cache — truoc ngay 3.
4. Dev 3/4 build UI theo contract da chot — ngay 2–4.
5. PM/Dev 5 chi cho qua phase khi integration checklist pass.
