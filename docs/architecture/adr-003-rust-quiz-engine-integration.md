# ADR-003: Rust quiz engine runtime integration

- Status: **Deferred / no runtime binding yet**
- Date: 2026-09-01
- Owner: Dev5
- Scope: `crates/quiz-core`

## Context

Phase 3 introduces deterministic learning algorithms for seeded shuffle,
scoring and question generation. The plan allows Rust to remain a pure crate
unless a benchmark proves that a runtime binding is useful.

The current platform already has Go services and a TypeScript frontend. Adding
WASM or FFI before the API contract and golden vectors are stable would add
build and deployment coupling without evidence of a production benefit.

## Decision

Keep `crates/quiz-core` as an independent Rust workspace crate during the first
Phase 3 integration. Do **not** add a network service, WASM bundle, or Go FFI
binding in this phase until all of the following evidence exists:

1. Golden test vectors are frozen and shared with the frontend/backend tests.
2. Benchmarks cover 10, 100, 1,000 and 10,000 cards.
3. The benchmark compares the Rust implementation with the existing
   TypeScript/Go baseline using equivalent seeded inputs.
4. The measured improvement is material enough to justify an additional
   runtime boundary.
5. The target CI build can compile and test the chosen binding deterministically.

If the benchmark does not demonstrate a material benefit, Rust remains a pure
library and is consumed only by tests/tools where appropriate.

## Options considered

### WASM

Pros: natural browser boundary, portable deterministic engine.
Cons: adds bundle/build complexity and a second generated artifact.

### Go FFI

Pros: keeps algorithm execution near the Study service.
Cons: adds CGO/toolchain complexity and complicates container builds.

### Rust crate only — selected for now

Pros: zero runtime coupling, easy unit/property tests and benchmarks, no new
service boundary. Cons: no immediate runtime speedup.

## Consequences

- Phase 3 can lock the algorithm contract without prematurely coupling the
  production runtime to Rust.
- Dev4 can benchmark and test `quiz-core` independently.
- Dev5 owns the final runtime decision after benchmark evidence is attached to
  the release gate.

## Revisit trigger

Revisit this ADR after the Rust benchmark report and golden vectors are merged.
A runtime binding may be proposed only in a separate PR with target-specific
CI coverage and a rollback path.
