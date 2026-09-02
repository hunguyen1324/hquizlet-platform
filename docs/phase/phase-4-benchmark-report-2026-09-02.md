# Phase 4 Benchmark Report - 2026-09-02

Commit: `1389791`
Machine: Windows amd64, 12th Gen Intel(R) Core(TM) i5-12500H

## Summary

Phase 4 keeps Rust as the deterministic spec/golden crate and keeps Go on the quiz request path.

The Go service benchmark covers realistic API behavior: decks may contain 10,000 cards, but generated/evaluated sessions are contract-capped at 100 items. The measured 10,000-card generate path is slower because distractor generation scans/shuffles the large source deck, but it remains outside the Phase 5 runtime-integration threshold for adding Rust FFI/WASM/service complexity.

## Go Request-Path Baseline

Command:

```powershell
$env:GOCACHE=(Resolve-Path -LiteralPath '.tmp-go-cache').Path
go test ./services/quiz/internal/engine -run '^$' -bench . -benchtime=100ms
```

Results:

| Benchmark | ns/op |
| --- | ---: |
| Generate/10_cards_limit_10 | 73,260 |
| Generate/100_cards_limit_100 | 242,903 |
| Generate/1000_cards_limit_100 | 3,272,700 |
| Generate/10000_cards_limit_100 | 60,108,350 |
| Evaluate/10_cards_limit_10 | 7,754 |
| Evaluate/100_cards_limit_100 | 62,135 |
| Evaluate/1000_cards_limit_100 | 127,459 |
| Evaluate/10000_cards_limit_100 | 1,457,165 |

## Rust Spec-Crate Baseline

Command:

```powershell
cargo bench --package quiz-core --bench shuffle_bench -- --measurement-time 1 --warm-up-time 1
```

Results captured from Criterion:

| Benchmark | Typical time |
| --- | ---: |
| shuffle_10_seed42 | 95 ns |
| shuffle_100_seed42 | 596 ns |
| shuffle_1000_seed42 | 4.99 us |
| shuffle_10000_seed42 | 52.4 us |
| score_answer_correct | 148 ns |
| score_answer_wrong | 155 ns |
| generate_questions_10_seed42 | 11.9 us |
| generate_questions_100_seed42 | 248 us |
| generate_questions_1000_seed42 | 11.4 ms |
| score_session_10 | 13.6 ns |
| score_session_100 | 81.0 ns |
| score_session_1000 | 74.5 ns |
| score_session_10000 | 75.1 ns |

`score_session` caps at 100 results by contract, so 1,000 and 10,000 input cases have similar timing.

## Regression Threshold

Phase 5 should treat a sustained regression above 25% on the Go request-path benchmark as a performance review trigger. Rust runtime integration should require a separate benchmark showing a clear end-to-end request-path gain, including serialization and deployment cost.
