# Phase 3 Dev5 Integration Gate — 2026-09-01

Owner: **Dev5**  
Branch: `phase3/dev5-integration`  
Target: `main`

## Scope completed

- OpenAPI contract advanced from v1.2.0 to v1.3.0.
- Added Learning Progress schemas and routes:
  - `POST /v1/study-sets/{studySetId}/progress`
  - `GET /v1/study-sets/{studySetId}/progress`
  - `GET /v1/study-sets/{studySetId}/progress/latest`
- Locked progress constraints: modes, score/total bounds, max 100 card results,
  per-card attempts 1..100 and idempotency key.
- Gateway continues to authenticate every `/v1/study-sets/**` request and therefore
  covers the progress endpoints. Client-supplied `X-User-ID` is stripped and replaced
  with the verified identity from Auth.
- Gateway error responses now use `{code,message,requestId,details}` for gateway-generated
  errors and propagate the request ID downstream.
- Added gateway tests for verified identity propagation, spoofed `X-User-ID`, request ID,
  and missing bearer behavior.
- Added CI contract/build gate for Go, Rust, frontend and OpenAPI lint.
- Added fresh-volume Docker/A-B ownership gate script.
- Added ADR-003: Rust quiz engine remains a pure crate until benchmark evidence justifies
  WASM or FFI runtime integration.

## Evidence status

| Gate | Status | Notes |
| --- | --- | --- |
| Static Phase 2 ownership review | PASS by code inspection | Gateway already authenticated Study routes and strips client identity. |
| OpenAPI v1.3 contract | IMPLEMENTED | Requires Dev1/Dev2 backend response alignment before final GO. |
| Gateway unit tests | ADDED | Must execute in CI/runner with Go 1.23. |
| Go build/test | NOT RUN HERE | Local sandbox does not have the required Go 1.23 workspace/toolchain. |
| Rust test | NOT RUN HERE | CI gate added. |
| Frontend build | NOT RUN HERE | Previously known PASS on Phase 2; CI gate added for current branch. |
| Docker fresh-volume gate | NOT RUN HERE | Docker daemon unavailable in this execution environment. |
| Live A/B ownership | NOT RUN HERE | Requires Docker/services; reproducible script added. |
| Progress E2E | BLOCKED BY DEPENDENCY | Dev2 progress domain must merge before final E2E smoke can pass. |

## Decision

**NO-GO for final Phase 3 release gate.** This is intentional: Dev5 must not claim
runtime/Docker/Progress E2E evidence that was not actually executed.

Next gate order:

1. Dev1/Dev2 lock the shared error response and progress backend.
2. Merge backend progress.
3. Run `infra/scripts/phase3-e2e-gate.sh` from a machine with Docker.
4. Run full Go/Rust/frontend/OpenAPI CI.
5. Add live progress POST/GET/latest and idempotency assertions to the final E2E run.
6. Re-review A/B ownership and produce the final PM GO/NO-GO report.
