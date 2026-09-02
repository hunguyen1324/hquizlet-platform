//! P4-BENCH-01 — Benchmark quiz-core shuffle at various deck sizes.
//! Run: cargo bench --package quiz-core
//!
//! Baseline: quiz-core v0.1.0, xorshift64 Fisher-Yates
//! Expected: p95 < 200µs for 1000 cards, < 2ms for 10000 cards

use criterion::{black_box, criterion_group, criterion_main, Criterion};

fn bench_seeded_shuffle(c: &mut Criterion) {
    for size in [10, 100, 1_000, 10_000] {
        let items: Vec<u64> = (1..=size as u64).collect();
        c.bench_function(&format!("shuffle_{size}_seed42"), |b| {
            b.iter(|| quiz_core::seeded_shuffle(black_box(&items), black_box(42)))
        });
    }
}

fn bench_score_answer(c: &mut Criterion) {
    c.bench_function("score_answer_correct", |b| {
        b.iter(|| quiz_core::score_answer(black_box("Tokyo"), black_box("tokyo")))
    });
    c.bench_function("score_answer_wrong", |b| {
        b.iter(|| quiz_core::score_answer(black_box("Tokyo"), black_box("Osaka")))
    });
}

fn bench_score_session(c: &mut Criterion) {
    for size in [10, 100, 1_000, 10_000] {
        let results: Vec<quiz_core::CardResult> = (1..=size as u64)
            .map(|id| quiz_core::CardResult {
                card_id: id,
                correct: id % 3 != 0,
                attempts: 1,
                response_time_ms: None,
            })
            .collect();
        c.bench_function(&format!("score_session_{size}"), |b| {
            b.iter(|| quiz_core::score_session(black_box(&results)))
        });
    }
}

fn bench_generate_questions(c: &mut Criterion) {
    for size in [10, 100, 1_000] {
        let items: Vec<quiz_core::Card> = (1..=size as u64)
            .map(|i| quiz_core::Card {
                id: i,
                term: format!("term-{i}"),
                definition: format!("def-{i}"),
            })
            .collect();
        c.bench_function(&format!("generate_questions_{size}_seed42"), |b| {
            b.iter(|| quiz_core::generate_questions(black_box(&items), black_box(42)))
        });
    }
}

criterion_group!(
    benches,
    bench_seeded_shuffle,
    bench_score_answer,
    bench_score_session,
    bench_generate_questions
);
criterion_main!(benches);
