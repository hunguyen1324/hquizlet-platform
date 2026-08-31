# HQuizlet Platform — Phase 2 PM Follow-up Review

Review date: 2026-08-31
Reviewed baseline: `resolved-main` at `6328e808000730bad39f3a5a78e504f71abc7450`
Prior review: `review/phase-2-pm-review-2026-08-31.md` (Decision: NO-GO)
Decision: **STILL NO-GO — one P0 blocker remains, plus unverified gates**

## 1. Executive Summary

Since the prior Phase 2 PM gate review, six of the seven P0 blockers have been
fixed and verified by direct code inspection: gateway identity propagation,
Study detail ownership, the Dashboard pagination contract, the editor's
transactional bulk save, Folder routing, and the Dev5 API-client contract
mismatches. Frontend production build passes cleanly with no regressions.

One P0 blocker remains **unfixed despite appearing fixed**: the `pg_trgm`
migration ordering bug (P0-05). The attempted fix does not work and will
still fail on a fresh PostgreSQL database. `go build`/`go test` and the full
Docker Compose gate could not be executed in this review environment
(missing Go 1.23 toolchain, no network egress to the Go module proxy, no
Docker) and remain unverified, consistent with the prior review's
limitations.

`resolved-main` is currently 4 commits ahead of `origin/main` and has not
been pushed/merged.

## 2. Re-verification of Prior P0 Findings

| ID | Prior status | Current status | Evidence |
| --- | --- | --- | --- |
| P0-01 Gateway does not authenticate Study requests | FAIL | **FIXED** | `services/gateway/main.go`: `authenticatedProxy` calls `/internal/auth/verify`, strips client-supplied `X-User-ID`, injects verified identity, returns 401 on missing/invalid auth |
| P0-02 Study detail leaks data across users | FAIL | **FIXED** | `GetWithCards(ctx, id, userID)` now ownership-scoped; tests assert `ErrUnauthorized`/`ErrForbidden` |
| P0-03 Dashboard response contract incompatible | FAIL | **FIXED** | Dashboard consumes `result.items ?? []`; AbortController guard added against out-of-order responses |
| P0-04 Editor does not use transactional bulk endpoint | FAIL | **FIXED** | `StudySetEditor.tsx` calls `flashcardApi.bulkSave(...)` once per save |
| P0-05 Migration enables `pg_trgm` after using it | FAIL | **STILL BROKEN** | See Section 3 — the "fix" does not resolve the ordering bug |
| P0-06 Folder API not routed by gateway | FAIL | **FIXED** | `/v1/folders` and `/v1/folders/` now proxied through `authenticatedProxy` |
| P0-07 Dev5 PR #16 unmergeable / contract mismatches | FAIL | **FIXED** | `client.ts` now matches backend: `search` param, bulk `POST` + `cards`, folder `name`, profile `PATCH`, error parsing reads both `code/message` and `error` |

Additional checks performed:

- No conflict markers found in `.go`/`.ts`/`.tsx` files.
- No mock data (`mockLogin`, `mockRegister`, `MOCK_SETS`) imported into any
  production code path; the only mock files (`lib/mock/mockData.ts`,
  `features/learning/mockData.ts`) are not exported or referenced by the
  learning feature's public `index.ts`.
- `npm --prefix apps/web run build` — **PASS** (41 modules, no errors).

## 3. P0-05 Is Not Actually Fixed

Current migration code (`services/study/internal/migration/migration.go`):

```sql
-- 013
CREATE INDEX IF NOT EXISTS study_sets_title_trgm_idx ON study_sets USING gin(title gin_trgm_ops)
 WHERE EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')

-- 014
DO $$ BEGIN CREATE EXTENSION IF NOT EXISTS pg_trgm; EXCEPTION WHEN OTHERS THEN NULL; END $$
```

The `WHERE EXISTS (...)` clause is a **partial index predicate** — it
filters which rows are indexed at query time. It does not defer or skip
resolution of the `gin_trgm_ops` operator class, which PostgreSQL must
resolve when the `CREATE INDEX` statement is parsed, regardless of the
predicate. On a fresh database (migration 014 not yet run), migration 013
will still fail with an operator-class-not-found error.

Required fix: reorder so `CREATE EXTENSION IF NOT EXISTS pg_trgm` runs
**before** the index is created, or move the conditional index creation into
application-level Go code that checks extension availability before issuing
the `CREATE INDEX` DDL. Add a migration test starting from an empty database
to catch this class of bug going forward.

## 4. Gates Not Verified in This Review

Same limitation as the prior review, plus toolchain constraints specific to
this environment:

| Check | Result |
| --- | --- |
| `npm --prefix apps/web run build` | PASS |
| Conflict marker / mock-in-production-path search | PASS |
| `go build ./...` / `go test ./...` | NOT VERIFIED — sandbox has Go 1.22 via apt; `go.work` requires Go ≥1.23; no egress to `proxy.golang.org` to fetch the toolchain |
| `docker compose up --build` | NOT VERIFIED — Docker unavailable in this environment |
| Live A/B ownership test over HTTP | NOT VERIFIED — requires running services |

These must be run in an environment with full network access and Docker
before Phase 2 can be declared GO.

## 5. Outstanding P1s

- **P1-01 Error envelopes inconsistent** — frontend `apiFetch` now reads
  both `message`/`error` fields defensively, which mitigates the symptom,
  but Auth and Study backends still do not share one canonical envelope.
  Should still be standardized at the source.
- **P1-02 Shared UI component adoption** — not re-verified this pass; Dev3
  should confirm whether `P2-WEB-05` is complete or intentionally scoped
  down.
- **P1-03 PR evidence quality** — unchanged; PRs should attach reproducible
  command output, not commit-message claims.
- **P1-04 Developer identity attribution** — unchanged, informational only.

## 6. Fix Plan by Developer

### Dev1 — Auth/User
1. Normalize all Auth endpoints (`login`, `register`, `me`, `refresh`,
   `logout`, `profile`) to one `{code, message}` error envelope.
2. Coordinate with Dev2 and Dev5 to lock a single error schema across
   services.

### Dev2 — Study/Folder
1. Fix the P0-05 migration ordering bug per Section 3: create the `pg_trgm`
   extension before creating the trigram index; if extension creation may be
   restricted, gate the index creation in Go rather than static SQL.
2. Add a fresh-database migration test.
3. Align Study's error responses to the schema agreed with Dev1.

### Dev3 — Web Core
1. No open P0s. Confirm or complete shared UI component adoption (P1-02).
2. Re-verify error message rendering once the backend envelope is
   standardized.

### Dev4 — Learning
1. Re-test all four learning modes against real persisted data once the
   error envelope and any other contract changes land.
2. Finalize `progressContract.ts` review with Dev5 ahead of Phase 3.

### Dev5 — Integration/Gateway
1. Push/merge `resolved-main` (`6328e80`) into `origin/main`; close
   superseded PRs #20/#21 once confirmed fully ported.
2. After Dev2's migration fix lands, run the full Mandatory Re-test Gate on
   a machine with Docker and full network access: `go build ./...`,
   `go test ./...`, `docker compose up --build` from a fresh Postgres
   volume, `/healthz/services`, and a live A/B ownership test over HTTP.
3. Finalize the shared error envelope with Dev1/Dev2.
4. Bring `packages/api-contracts/openapi.yaml` into exact alignment with
   backend responses and the TypeScript client.

## 7. Final PM Decision

**NO-GO.** Fix P0-05 (migration ordering), then run and attach evidence for
the full re-test gate from a fresh clone and fresh PostgreSQL volume, before
requesting the next PM gate review.
