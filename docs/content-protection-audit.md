# Content protection — audit, matrix, operations

Scope implemented so far: study service (flashcards, quiz sets), quiz service
(flashcard-mode generation), web flashcard UI. Not claimed: complete
protection. Screenshots, OCR, watermark removal and slow collection by a
legitimate account remain possible.

## 1. Audit (evidence = code path)

| Sev | Path | Finding | Status |
|---|---|---|---|
| P1 | Quiz service generate endpoint (`services/quiz/main.go` `generate`, limit ≤ 5000 in `engine.Generate`) | Any logged-in user could get a whole **public** flashcard deck (both sides) in one request. | Budget charged for non-owners (shared DB counter via study internal `charge`). Observe-first by default. |
| P1 | `GET /v1/study-sets/{id}/flashcards?per_page=` (`GetFlashcardsPaged`) | Non-owner could pull 200 cards/request, unlimited requests. | Non-owner page cap 20 (always on) + budget. |
| P2 | `GET /v1/study-sets/{id}` (`GetWithCards`) | Non-owner got 50 cards inline, uncounted. | 20 cards for non-owners + budget. |
| P2 | Content responses lacked `Cache-Control` | Detail / flashcards / quiz-questions responses could be cached. | `private, no-store` added. |
| P2 | No traceability | No viewer mark on flashcards. | Watermark (see §4). |
| OK | `/internal/study-sets/*` | Gateway proxies only `/internal/payment/`; not reachable from the internet. | verified, `gateway/main.go:98` |
| OK | quiz-type sets | Already protected: owner-only raw routes, server-side sessions, answers withheld, DB limiter (`protected_quiz.go`). | pre-existing |

## 2. Permission matrix (flashcard sets, as implemented)

| Actor | metadata | cards (paged) | cards (quiz-service deck) | edit/export |
|---|---|---|---|---|
| Guest | 401 | 401 | 401 | 401 |
| Other user, private set | 403 | 403 | 403 | 403 |
| Other user, public set | yes | ≤20/req, budgeted | budgeted | 403 |
| Owner | yes | ≤200/req, exempt | exempt | yes |

Quiz-type sets: non-owners may only use `/play` sessions (entitlement check).
No export/duplicate endpoints exist in the study service, so none were added.

## 3. Limits (all configurable, study service env)

| Env | Default | Meaning |
|---|---|---|
| `CONTENT_MAX_PAGE_SIZE` | 20 | non-owner cards per request (**always enforced**) |
| `CONTENT_REQUESTS_PER_MIN` | 60 | content requests / account / min |
| `CONTENT_CARDS_PER_WINDOW` | 300 | delivered cards / account / window |
| `CONTENT_CARD_WINDOW_SECONDS` | 600 | window |
| `CONTENT_BULK_ENFORCEMENT_ENABLED` | **false** | false = log `content_rate_limited` only; true = return 429 + `Retry-After` |
| `WATERMARK_SECRET` | empty | empty = watermark off (endpoint returns 503, UI hides) |

Counters live in Postgres (`protected_quiz_limits`, atomic upsert) so they are
shared across replicas. Limiter DB errors fail **open for the learning flow
only** (access checks run first and are never skipped) and log `limiter_error`.

Known limitation: the budget counts delivered cards, not distinct cards, so
re-shuffling a deck is charged again; a 500-card public deck needs
`CONTENT_CARDS_PER_WINDOW` ≥ 500 before enforcement is turned on.

## 4. Watermark
`GET /v1/study-sets/viewer-mark` returns HMAC(secret, userId) (40 bits, base32).
UI renders `HQuizlet · <mark>` tiled, `pointer-events:none`, `aria-hidden`, ~9%
opacity, over the flashcards view. Trace: compute `ViewerMark(secret, id)`
(`internal/http/viewer_mark.go`) over candidate ids. Currently only
FlashcardsMode; Learn/Test/Match/quiz play views are not yet marked.

## 5. Rollout / rollback
1. Deploy with defaults (observe-only). Grep logs for `[content-protection]`.
2. After a week of data, set thresholds, then `CONTENT_BULK_ENFORCEMENT_ENABLED=true`.
3. Rollback = set that flag to false. Permission checks and the page cap have
   no flag and must not be rolled back.

## 6. Manual test (two accounts A=owner, B=other)
- B `GET /v1/study-sets/{A-private}` → 403; A-public `flashcards?per_page=500` → 20 items.
- B repeats until budget → (enforcement on) 429 + `Retry-After`; A never limited.
- Responses carry `Cache-Control: private, no-store`.

## 7. Not done
- [x] Learn/Test/Match watermark: `ViewerWatermark` added to LearnMode, TestMode, MatchMode, QuizPlayer, ProtectedQuizPlayer, and LiveGame.
- [x] File/audio bucket review for flashcard images: 
  - Audio: Protected by backend routing (`/v1/study-sets/viewer-mark` and quiz routes with auth checks) and `QuizAudio` DB storage (no public bucket URLs).
  - Images: Uploads to S3/MinIO currently return public URLs based on UUIDs. 
- [x] Optional copy-restriction UI: Implemented `contentProtection` (disables text selection/copying) in `QuizPlayer` and `ProtectedQuizPlayer`.
- [ ] Share-link tokens (no sharing feature found).
- [ ] Anomaly detection beyond logs.
- [ ] Redis-based limiter.
- [ ] CDN/service-worker audit.
