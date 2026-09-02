# Phase 5 release gate — 2026-09-02

Scope: Folder contract, persistence, ownership, Gateway routing, web UI, E2E and Rust ADR revisit.

## Automated evidence

| Gate | Command | Result |
| --- | --- | --- |
| Study backend | `go test ./services/study/...` | PASS |
| Gateway security | `go test ./services/gateway/...` | PASS |
| Frontend tests | `npm test --prefix apps/web` | PASS (5 tests) |
| Frontend production build | `npm run build --prefix apps/web` | PASS |
| OpenAPI | `npx --yes @redocly/cli@latest lint packages/api-contracts/openapi.yaml` | PASS with pre-existing style warnings |
| E2E syntax | `bash -n infra/scripts/phase5-e2e.sh` | PASS via Alpine/Bash container |
| Docker API E2E | `wsl -d Ubuntu -- bash -lc "GATEWAY_URL=http://localhost:8080 bash infra/scripts/phase5-e2e.sh"` | PASS (16 checks, existing volume migrated) |

## Runtime evidence required before GO

The existing-volume Docker run passes. Before final GO, run the same script
against a fresh Docker volume and attach the output plus the final commit SHA.
The script covers A/B registration, two
study sets, folder CRUD, add/remove, cross-user ownership and survival of the
source study set after folder deletion.

## Rust decision

ADR-003 selects the Go production runtime. Phase 4 crate-level benchmarks do
not prove an end-to-end benefit sufficient to add FFI, WASM or a sidecar.

Release status: **CONDITIONAL GO** pending fresh-volume Docker E2E evidence.
