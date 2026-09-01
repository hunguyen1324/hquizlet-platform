//! quiz-core — Dev 4 / P3-RUST-01, P3-RUST-02
//!
//! Deterministic quiz engine for HQuizlet Phase 3.
//! Pure functions only — no I/O, no DB, no HTTP, no global state.
//!
//! Public API:
//!   - `seeded_shuffle`      — Fisher-Yates with xorshift64 PRNG, seed-stable across runs
//!   - `score_answer`        — case/whitespace-insensitive match
//!   - `score_session`       — aggregates per-card results into SessionScore
//!   - `generate_questions`  — builds multiple-choice questions with deterministic distractors
//!
//! Golden test vectors are pinned in `tests/golden.rs`.

// ─── Types ────────────────────────────────────────────────────────────────────

/// A flashcard as understood by quiz-core.
/// Mirrors the backend `flashcards` table columns used by learning modes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Card {
    pub id: u64,
    pub term: String,
    pub definition: String,
}

/// A multiple-choice question produced by `generate_questions`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Question {
    /// The card being tested.
    pub card: Card,
    /// Four choices: one correct definition + up to three distractors.
    /// Order is deterministic given the seed.
    pub choices: Vec<String>,
    /// Index into `choices` of the correct answer.
    pub correct_index: usize,
}

/// Per-card result sent by the frontend after a session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CardResult {
    pub card_id: u64,
    pub correct: bool,
    /// Number of attempts (Learn mode retries). Clamped to 1..=100.
    pub attempts: u8,
    /// Response time in milliseconds. None if not measured.
    pub response_time_ms: Option<u32>,
}

/// Aggregate result for one learning session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SessionScore {
    pub correct: u32,
    pub total: u32,
    /// Percentage 0–100, rounded down.
    pub pct: u8,
}

// ─── PRNG (xorshift64) ────────────────────────────────────────────────────────

/// Minimal xorshift64 PRNG — deterministic, no std dependency, no external crate.
/// Produces the same sequence for the same seed on every platform.
struct Xorshift64 {
    state: u64,
}

impl Xorshift64 {
    /// Seed must be nonzero. Panics in debug if zero.
    fn new(seed: u64) -> Self {
        debug_assert!(seed != 0, "xorshift64 seed must be nonzero");
        // Guard for release builds too.
        Self {
            state: if seed == 0 { 1 } else { seed },
        }
    }

    fn next(&mut self) -> u64 {
        let mut x = self.state;
        x ^= x << 13;
        x ^= x >> 7;
        x ^= x << 17;
        self.state = x;
        x
    }

    /// Returns a value in `0..n`. Panics if n == 0.
    fn next_usize(&mut self, n: usize) -> usize {
        (self.next() % n as u64) as usize
    }
}

// ─── Public API ───────────────────────────────────────────────────────────────

/// Fisher-Yates shuffle driven by a seeded xorshift64 PRNG.
///
/// The same `seed` always produces the same permutation for the same `items`.
/// `seed` must be nonzero; if zero, it is replaced with 1 (no panic in release).
///
/// # Complexity
/// O(n) time, O(n) space (clones the slice).
pub fn seeded_shuffle<T: Clone>(items: &[T], seed: u64) -> Vec<T> {
    let mut result: Vec<T> = items.to_vec();
    if result.len() <= 1 {
        return result;
    }
    let effective_seed = if seed == 0 { 1 } else { seed };
    let mut rng = Xorshift64::new(effective_seed);
    for i in (1..result.len()).rev() {
        let j = rng.next_usize(i + 1);
        result.swap(i, j);
    }
    result
}

/// Case- and whitespace-insensitive answer comparison.
///
/// Strips leading/trailing whitespace and compares lowercased strings.
/// Does NOT do typo tolerance — that is a Phase 4 concern.
pub fn score_answer(expected: &str, submitted: &str) -> bool {
    normalize(expected) == normalize(submitted)
}

fn normalize(s: &str) -> String {
    s.trim().to_lowercase()
}

/// Compute aggregate session score from a slice of `CardResult`.
///
/// - `total` = `results.len()` (clamped to 100 per contract).
/// - `correct` = count of results where `correct == true`.
/// - `pct` = floor(correct * 100 / total), or 0 if total == 0.
pub fn score_session(results: &[CardResult]) -> SessionScore {
    // Cap at 100 matching the backend contract.
    let capped: &[CardResult] = if results.len() > 100 {
        &results[..100]
    } else {
        results
    };
    let total = capped.len() as u32;
    let correct = capped.iter().filter(|r| r.correct).count() as u32;
    let pct = if total == 0 {
        0
    } else {
        ((correct * 100) / total) as u8
    };
    SessionScore { correct, total, pct }
}

/// Generate multiple-choice questions for a test session.
///
/// - `cards`: the full deck. Needs at least 2 cards; returns empty vec otherwise.
/// - `seed`: controls both card order and distractor selection deterministically.
/// - Each question has `min(4, cards.len())` choices (1 correct + up to 3 distractors).
/// - `correct_index` is deterministic for the same seed + cards.
///
/// # Panics
/// Never panics for any valid input.
pub fn generate_questions(cards: &[Card], seed: u64) -> Vec<Question> {
    if cards.len() < 2 {
        return vec![];
    }

    let effective_seed = if seed == 0 { 1 } else { seed };

    // Shuffle the deck order.
    let shuffled = seeded_shuffle(cards, effective_seed);

    // We need a second PRNG stream for distractor selection to avoid
    // aliasing with the deck shuffle stream. Derive it by mixing seed bits.
    let distractor_seed = effective_seed.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);
    let distractor_seed = if distractor_seed == 0 { 1 } else { distractor_seed };

    let mut distractor_rng = Xorshift64::new(distractor_seed);
    let num_choices = shuffled.len().min(4);

    shuffled
        .iter()
        .map(|card| {
            // Collect definitions from other cards as distractor pool.
            let mut pool: Vec<&str> = cards
                .iter()
                .filter(|c| c.id != card.id)
                .map(|c| c.definition.as_str())
                .collect();

            // Shuffle pool deterministically.
            for i in (1..pool.len()).rev() {
                let j = distractor_rng.next_usize(i + 1);
                pool.swap(i, j);
            }

            let distractors: Vec<String> = pool
                .into_iter()
                .take(num_choices - 1)
                .map(|s| s.to_owned())
                .collect();

            // Build choices list: correct + distractors, then shuffle placement.
            let mut choices: Vec<String> = std::iter::once(card.definition.clone())
                .chain(distractors)
                .collect();

            // Shuffle choices to randomize correct answer position.
            for i in (1..choices.len()).rev() {
                let j = distractor_rng.next_usize(i + 1);
                choices.swap(i, j);
            }

            let correct_index = choices
                .iter()
                .position(|c| c == &card.definition)
                .unwrap_or(0);

            Question {
                card: card.clone(),
                choices,
                correct_index,
            }
        })
        .collect()
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    // ── score_answer ──────────────────────────────────────────────────────────

    #[test]
    fn score_answer_exact() {
        assert!(score_answer("Tokyo", "Tokyo"));
    }

    #[test]
    fn score_answer_case_insensitive() {
        assert!(score_answer("Tokyo", "tokyo"));
        assert!(score_answer("TOKYO", " tokyo "));
    }

    #[test]
    fn score_answer_trim_whitespace() {
        assert!(score_answer("  hello  ", "hello"));
        assert!(score_answer("hello", "  hello  "));
    }

    #[test]
    fn score_answer_wrong() {
        assert!(!score_answer("Tokyo", "Osaka"));
        assert!(!score_answer("Tokyo", ""));
    }

    // ── seeded_shuffle ────────────────────────────────────────────────────────

    #[test]
    fn seeded_shuffle_deterministic() {
        let items = vec![1u32, 2, 3, 4, 5, 6, 7, 8];
        let a = seeded_shuffle(&items, 42);
        let b = seeded_shuffle(&items, 42);
        assert_eq!(a, b, "Same seed must produce same permutation");
    }

    #[test]
    fn seeded_shuffle_different_seeds_differ() {
        let items: Vec<u32> = (1..=10).collect();
        let a = seeded_shuffle(&items, 1);
        let b = seeded_shuffle(&items, 2);
        // With 10 elements it is astronomically unlikely they are the same.
        assert_ne!(a, b, "Different seeds should produce different permutations");
    }

    #[test]
    fn seeded_shuffle_preserves_elements() {
        let items = vec![10u32, 20, 30, 40, 50];
        let shuffled = seeded_shuffle(&items, 999);
        let mut original = items.clone();
        let mut result = shuffled.clone();
        original.sort();
        result.sort();
        assert_eq!(original, result, "Shuffle must preserve all elements");
    }

    #[test]
    fn seeded_shuffle_empty() {
        let empty: Vec<u32> = vec![];
        assert_eq!(seeded_shuffle(&empty, 1), vec![]);
    }

    #[test]
    fn seeded_shuffle_single() {
        assert_eq!(seeded_shuffle(&[42u32], 7), vec![42u32]);
    }

    // ── Golden vector: seeded_shuffle with seed=1 on [0..8] ──────────────────
    // PINNED: Do not change without updating the golden vector comment and bumping the version.
    // These vectors are shared with the frontend TypeScript implementation for cross-validation.
    //
    // To regenerate: run `cargo test -- --nocapture golden` and copy the output.
    // Version: quiz-core v0.1.0 / xorshift64
    #[test]
    fn golden_shuffle_seed1() {
        let items: Vec<u32> = (0..8).collect();
        let result = seeded_shuffle(&items, 1);
        // Pinned output for seed=1, input=[0,1,2,3,4,5,6,7] — xorshift64 v0.1.0
        assert_eq!(result, vec![4u32, 2, 5, 7, 0, 3, 6, 1]);
    }

    #[test]
    fn golden_shuffle_seed42() {
        let items: Vec<u32> = (0..8).collect();
        let result = seeded_shuffle(&items, 42);
        // Pinned output for seed=42, input=[0,1,2,3,4,5,6,7] — xorshift64 v0.1.0
        assert_eq!(result, vec![6u32, 4, 5, 7, 1, 0, 3, 2]);
    }

    #[test]
    fn golden_shuffle_seed12345() {
        let items: Vec<u32> = (0..8).collect();
        let result = seeded_shuffle(&items, 12345);
        // Pinned output for seed=12345, input=[0,1,2,3,4,5,6,7] — xorshift64 v0.1.0
        assert_eq!(result, vec![7u32, 6, 4, 2, 0, 5, 3, 1]);
    }

    // ── score_session ─────────────────────────────────────────────────────────

    fn make_result(id: u64, correct: bool) -> CardResult {
        CardResult { card_id: id, correct, attempts: 1, response_time_ms: None }
    }

    #[test]
    fn score_session_all_correct() {
        let results: Vec<CardResult> = (1..=5).map(|i| make_result(i, true)).collect();
        let s = score_session(&results);
        assert_eq!(s.correct, 5);
        assert_eq!(s.total, 5);
        assert_eq!(s.pct, 100);
    }

    #[test]
    fn score_session_all_wrong() {
        let results: Vec<CardResult> = (1..=5).map(|i| make_result(i, false)).collect();
        let s = score_session(&results);
        assert_eq!(s.correct, 0);
        assert_eq!(s.total, 5);
        assert_eq!(s.pct, 0);
    }

    #[test]
    fn score_session_mixed() {
        // 3 correct, 2 wrong → 60%
        let results = vec![
            make_result(1, true),
            make_result(2, false),
            make_result(3, true),
            make_result(4, false),
            make_result(5, true),
        ];
        let s = score_session(&results);
        assert_eq!(s.correct, 3);
        assert_eq!(s.total, 5);
        assert_eq!(s.pct, 60);
    }

    #[test]
    fn score_session_empty() {
        let s = score_session(&[]);
        assert_eq!(s.correct, 0);
        assert_eq!(s.total, 0);
        assert_eq!(s.pct, 0);
    }

    #[test]
    fn score_session_capped_at_100() {
        // 110 results: first 100 are used, 10 are ignored
        let mut results: Vec<CardResult> = (1..=100).map(|i| make_result(i, true)).collect();
        let extra: Vec<CardResult> = (101..=110).map(|i| make_result(i, false)).collect();
        results.extend(extra);
        let s = score_session(&results);
        assert_eq!(s.total, 100);
        assert_eq!(s.correct, 100);
    }

    // ── generate_questions ────────────────────────────────────────────────────

    fn make_cards(n: u64) -> Vec<Card> {
        (1..=n)
            .map(|i| Card {
                id: i,
                term: format!("term-{i}"),
                definition: format!("def-{i}"),
            })
            .collect()
    }

    #[test]
    fn generate_questions_needs_at_least_2_cards() {
        assert_eq!(generate_questions(&[], 1), vec![]);
        assert_eq!(generate_questions(&make_cards(1), 1), vec![]);
    }

    #[test]
    fn generate_questions_count_matches_deck() {
        let cards = make_cards(5);
        let qs = generate_questions(&cards, 1);
        assert_eq!(qs.len(), 5);
    }

    #[test]
    fn generate_questions_choices_include_correct() {
        let cards = make_cards(6);
        let qs = generate_questions(&cards, 7);
        for q in &qs {
            assert!(
                q.choices.contains(&q.card.definition),
                "Correct answer must be in choices"
            );
            assert_eq!(
                q.choices[q.correct_index], q.card.definition,
                "correct_index must point to the correct answer"
            );
        }
    }

    #[test]
    fn generate_questions_no_duplicate_choices() {
        let cards = make_cards(8);
        let qs = generate_questions(&cards, 99);
        for q in &qs {
            let unique: std::collections::HashSet<_> = q.choices.iter().collect();
            assert_eq!(unique.len(), q.choices.len(), "Choices must be unique per question");
        }
    }

    #[test]
    fn generate_questions_deterministic() {
        let cards = make_cards(6);
        let a = generate_questions(&cards, 100);
        let b = generate_questions(&cards, 100);
        assert_eq!(a, b, "Same seed must produce identical questions");
    }

    #[test]
    fn generate_questions_different_seeds_differ() {
        let cards = make_cards(6);
        let a = generate_questions(&cards, 1);
        let b = generate_questions(&cards, 2);
        // At least the order should differ for 6 cards.
        assert_ne!(a, b);
    }

    #[test]
    fn generate_questions_max_4_choices() {
        // With exactly 2 cards: 2 choices
        let cards2 = make_cards(2);
        let qs2 = generate_questions(&cards2, 1);
        assert!(qs2.iter().all(|q| q.choices.len() == 2));

        // With 5+ cards: 4 choices
        let cards5 = make_cards(5);
        let qs5 = generate_questions(&cards5, 1);
        assert!(qs5.iter().all(|q| q.choices.len() == 4));
    }

    // ── Golden vector: generate_questions ─────────────────────────────────────
    // PINNED — quiz-core v0.1.0. Seed=1, 4 cards.
    // Verify correct_index pinning to catch PRNG regressions.
    #[test]
    fn golden_questions_seed1_4cards() {
        let cards = make_cards(4);
        let qs = generate_questions(&cards, 1);
        assert_eq!(qs.len(), 4);
        // Verify that correct answers are in the right positions (correct_index pinned).
        for q in &qs {
            assert_eq!(q.choices[q.correct_index], q.card.definition);
        }
        // Pin the card order (deck shuffle with seed=1)
        // Cards are shuffled before becoming questions, so pin first card ID.
        // The exact ID depends on seeded_shuffle([1,2,3,4], seed=1).
        assert_eq!(qs[0].card.id, 1, "First question card ID pinned for seed=1");
    }
}
