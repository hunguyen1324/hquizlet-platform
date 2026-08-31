# HQuizlet Platform — Phase 2 PM Gate Review

Review date: 2026-08-31  
Reviewed baseline: `main` at `65afcdf75aba72924da7264014599ca641fbe389`  
Decision: **NO-GO — Phase 2 is not ready to move to Phase 3**

## 1. Executive Summary

Phase 2 has substantial implementation from Dev1–Dev4, and the frontend production build passes. However, the main end-to-end flow is not acceptable for release because authentication identity is not integrated into the Study service, ownership enforcement can be bypassed, the Study list contract does not match the frontend, the editor does not use the transactional bulk endpoint, and the Dev5 integration PR is unmerged and conflicts with Dev3's API client.

Phase 3 must not begin until all P0 blockers in this report are fixed and the complete Docker/integration gate passes.

## 2. Review Scope and Evidence

Reviewed:

- Phase 2 execution plan and Definition of Done.
- `main` and all Phase 2 branches.
- Phase 2 pull requests #12–#17.
- Auth, gateway, Study, frontend core, learning modes, OpenAPI, migrations, and Docker configuration.
- Production mock/placeholder and conflict-marker searches.

Commands/results available in the review environment:

| Check | Result |
| --- | --- |
| `npm --prefix apps/web ci` | PASS |
| `npm --prefix apps/web run build` | PASS — TypeScript + Vite, 41 modules |
| Conflict marker search on `main` | PASS — none found |
| Production flow mock search | PASS — no mock import found in the main flow |
| `go test ./...` | NOT VERIFIED — Go unavailable in review runtime |
| `docker compose up --build` | NOT VERIFIED — Docker unavailable in review runtime |

The static contract, authorization, routing, and migration findings below are independently sufficient to block the phase gate.

## 3. Status by Developer

| Developer | Status | PM assessment |
| --- | --- | --- |
| Dev1 — Auth/User | PARTIAL | Auth lifecycle, profile, verify endpoint, and unit tests exist. Shared identity is not integrated by the gateway. |
| Dev2 — Study/Folder | FAIL | APIs exist, but ownership is bypassable, detail reads are unprotected, pagination breaks the frontend, and the migration order is unsafe. |
| Dev3 — Web Core | FAIL | Frontend builds, but Dashboard expects the wrong response shape and the editor does not use bulk transactional save. |
| Dev4 — Learning | CONDITIONAL PASS | Four modes use real props and the build passes. The Phase 3 progress contract still needs final Dev5 contract review and integration evidence. |
| Dev5 — Integration | FAIL | PR #16 is unmerged, conflicts with Dev3, and contains several API contract mismatches. The phase gate has not been executed successfully. |

## 4. P0 Blockers

### P0-01 — Gateway does not authenticate Study requests

The Auth service exposes `GET /internal/auth/verify`, but the gateway never calls it and never injects a verified user ID into Study requests. It only forwards incoming headers.

The Study handler reads `X-User-ID`, defaults to `0` when absent, and the service layer treats `userID == 0` as a bypass:

- List returns all study sets.
- Ownership checks return success.
- Create writes records with `user_id = 0`.

Impact: authenticated users are not isolated and authorization is ineffective.

Required fix:

1. Add gateway authentication middleware for protected Study/Folder routes.
2. Call Auth verify using the bearer token.
3. Strip any client-supplied identity headers.
4. Inject a verified canonical identity header.
5. Return `401` when identity is missing or invalid.
6. Remove every `userID == 0` authorization bypass from Study services.

Owners: Dev5 gateway, Dev1 verify contract, Dev2 Study enforcement.

### P0-02 — Study detail leaks data across users

`GET /v1/study-sets/{id}` calls `GetWithCards(ctx, id)` without a user ID or ownership check.

Impact: any authenticated or unauthenticated caller who knows an ID can read another user's set and cards.

Required fix: make detail lookup owner-scoped and return `404` or `403` consistently according to the approved contract.

Owner: Dev2.

### P0-03 — Dashboard response contract is incompatible

Backend `GET /v1/study-sets` returns a paginated object:

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "perPage": 20,
  "totalPages": 0
}
```

The frontend declares `Promise<StudySet[]>` and stores the complete response as an array. Dashboard later calls array operations such as `.filter()`, which can fail at runtime with `sets.filter is not a function`.

Required fix: define one paginated response in OpenAPI, backend, shared TypeScript types, and the API client. Dashboard must consume `response.items` and use backend query parameters where appropriate.

Owners: Dev2, Dev3, Dev5.

### P0-04 — Editor does not use the transactional bulk endpoint

Dev2 implemented bulk save, but Dev3's editor performs sequential create/update/delete requests. A mid-sequence failure leaves partially updated data.

Required fix: the editor must call the approved bulk endpoint once per save after saving the study-set header. The backend transaction must reject invalid or cross-set card IDs instead of silently skipping them.

Owners: Dev2, Dev3, Dev5 contract.

### P0-05 — Study migration enables `pg_trgm` after using it

Migration 013 creates an index with `gin_trgm_ops`; migration 014 enables `pg_trgm`. On a fresh PostgreSQL database, operator-class resolution may fail before migration 014 executes.

Required fix:

1. Enable `pg_trgm` before creating the trigram index.
2. If extension creation is unavailable, skip the trigram index safely and retain functional search.
3. Add a fresh-database migration test and an upgrade test.

Owner: Dev2.

### P0-06 — Folder API is not routed by the gateway

Study registers `/v1/folders`, but the gateway only proxies Auth, Study Sets, Flashcards, and Live Sessions.

Required fix: proxy `/v1/folders` and `/v1/folders/`, protect them with the same verified identity middleware, and add them to integration checks.

Owner: Dev5.

### P0-07 — Dev5 PR #16 cannot be merged as-is

The branch conflicts with Dev3 in:

- `apps/web/src/lib/api/client.ts`
- `apps/web/src/lib/api/index.ts`

Known contract mismatches in the branch:

| Dev5 branch/client | Backend implementation |
| --- | --- |
| Search parameter `q` | `search` |
| Bulk method `PUT` | `POST` |
| Bulk request field `flashcards` | `cards` |
| Profile update `PUT` | `PATCH` |
| Folder field `title` | `name` |
| Error client reads `error` | Auth returns `code` + `message` |
| Study list typed as `StudySet[]` | Backend returns paginated object |

Required fix: rebase from the latest `main`, resolve API-client ownership with Dev3, align the contract to actual approved behavior, then rerun every gate.

Owner: Dev5, with Dev1–Dev4 contract review.

## 5. Additional Findings

### P1-01 — Error envelopes are inconsistent

Auth uses `{code, message}` while Study uses `{error}`. The frontend client only reads `error`, so Auth errors can degrade to generic `Request failed <status>` messages.

Required fix: adopt one error envelope and map it consistently across services and frontend.

### P1-02 — Shared UI task is not clearly satisfied

Core screens still use raw buttons, inputs, alerts, and duplicated class patterns rather than a consistently adopted shared component layer. Dev3 must document whether `P2-WEB-05` is intentionally limited or finish adoption.

### P1-03 — PR evidence does not meet the mandatory template

Several Phase 2 PRs lack complete evidence for Docker, Go build/test, manual flow, API/DB changes, and risks. Commit messages claiming a pass are not a substitute for reproducible evidence.

### P1-04 — Developer identity is not independently verifiable

All GitHub pushes are associated with the same repository account. Dev1–Dev5 attribution is inferred from commit author/email and message labels, not separate GitHub identities.

## 6. Required Remediation Order

1. Dev5 implements verified gateway identity propagation and Folder routing.
2. Dev2 removes unauthenticated bypasses and adds owner-scoped detail reads.
3. Dev2 fixes migration order and adds migration verification.
4. Dev2/Dev5 freeze the Study list and bulk-save contract.
5. Dev3 updates Dashboard to the paginated response.
6. Dev3 switches StudySetEditor to one bulk-save request.
7. Dev1/Dev2/Dev5 standardize errors and update OpenAPI.
8. Dev5 rebases PR #16 and resolves API-client conflicts with Dev3.
9. Dev4 rechecks all four modes against real data after the contract changes.
10. Run the complete Phase 2 gate from a fresh clone and fresh PostgreSQL volume.

## 7. Mandatory Re-test Gate

Phase 2 can change to GO only when all checks pass:

- [ ] Frontend clean install and production build.
- [ ] `go test ./...` for Auth and Study.
- [ ] `go build ./...` for every Go service.
- [ ] Docker Compose builds and starts from a fresh clone.
- [ ] Fresh PostgreSQL migrations succeed.
- [ ] `/healthz/services` reports every required service as `ok`.
- [ ] Register, login, `/me`, refresh, logout, and logout-all work through gateway.
- [ ] User A cannot list, read, edit, delete, star, or folder-link User B's data.
- [ ] Study create/edit saves all cards atomically through bulk API.
- [ ] Dashboard pagination, search, sort, loading, empty, and error states work.
- [ ] Flashcards, Learn, Test, and Match use persisted real data.
- [ ] Folder CRUD and add/remove set work through gateway.
- [ ] OpenAPI, backend JSON, and TypeScript client match exactly.
- [ ] No production mock, user-facing placeholder, or conflict marker remains.

## 8. Final PM Decision

**NO-GO.** Do not merge the current Dev5 integration branch and do not start Phase 3. Fix all P0 findings, supply reproducible test evidence, and request a new PM gate review.
