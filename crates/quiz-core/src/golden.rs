//! Golden vector generation for cross-validation between Rust and Go engines.
//!
//! Run: `cargo test golden_vector_export -- --nocapture` to regenerate JSON.
//! The output JSON is the source of truth for Go-vs-Rust comparison tests.

use crate::{seeded_shuffle, Card};
use serde::Serialize;

#[derive(Serialize)]
pub struct GoldenVector {
    pub algorithm: &'static str,
    pub version: &'static str,
    pub cases: Vec<ShuffleCase>,
}

#[derive(Serialize)]
pub struct ShuffleCase {
    pub label: &'static str,
    pub seed: u64,
    pub input: Vec<u64>,
    pub output: Vec<u64>,
}

#[derive(Serialize)]
pub struct ScoreCase {
    pub expected: &'static str,
    pub submitted: &'static str,
    pub correct: bool,
}

#[derive(Serialize)]
pub struct GenerateQuestionCase {
    pub label: &'static str,
    pub seed: u64,
    pub cards: Vec<GoldenCard>,
    pub result_count: usize,
    pub first_card_id: u64,
    pub first_choices: Vec<String>,
    pub first_correct_index: usize,
}

#[derive(Serialize)]
pub struct GoldenCard {
    pub id: u64,
    pub term: String,
    pub definition: String,
}

/// Shuffle golden vectors for cross-validation with Go.
/// These use the same xorshift64 PRNG: x ^= x << 13; x ^= x >> 7; x ^= x << 17
pub fn shuffle_golden_vectors() -> GoldenVector {
    let cases = vec![
        ShuffleCase {
            label: "seed=0 (mapped to 1)",
            seed: 0,
            input: (0..8).collect(),
            output: seeded_shuffle(&(0..8).collect::<Vec<u64>>(), 0),
        },
        ShuffleCase {
            label: "seed=1",
            seed: 1,
            input: (0..8).collect(),
            output: seeded_shuffle(&(0..8).collect::<Vec<u64>>(), 1),
        },
        ShuffleCase {
            label: "seed=42",
            seed: 42,
            input: (0..8).collect(),
            output: seeded_shuffle(&(0..8).collect::<Vec<u64>>(), 42),
        },
        ShuffleCase {
            label: "seed=12345",
            seed: 12345,
            input: (0..8).collect(),
            output: seeded_shuffle(&(0..8).collect::<Vec<u64>>(), 12345),
        },
        ShuffleCase {
            label: "seed=999999",
            seed: 999999,
            input: (0..8).collect(),
            output: seeded_shuffle(&(0..8).collect::<Vec<u64>>(), 999999),
        },
        ShuffleCase {
            label: "4 items seed=42",
            seed: 42,
            input: vec![1, 2, 3, 4],
            output: seeded_shuffle(&[1, 2, 3, 4], 42),
        },
        ShuffleCase {
            label: "2 items seed=1",
            seed: 1,
            input: vec![10, 20],
            output: seeded_shuffle(&[10, 20], 1),
        },
        ShuffleCase {
            label: "100 items seed=42",
            seed: 42,
            input: (1..=100).collect(),
            output: seeded_shuffle(&(1..=100).collect::<Vec<u64>>(), 42),
        },
    ];

    GoldenVector {
        algorithm: "xorshift64_fisher_yates",
        version: env!("CARGO_PKG_VERSION"),
        cases,
    }
}

/// Score answer golden vectors for cross-validation with Go normalize().
pub fn score_answer_golden_vectors() -> Vec<ScoreCase> {
    vec![
        ScoreCase { expected: "Tokyo", submitted: "Tokyo", correct: true },
        ScoreCase { expected: "Tokyo", submitted: "tokyo", correct: true },
        ScoreCase { expected: "Tokyo", submitted: "  TOKYO  ", correct: true },
        ScoreCase { expected: "  hello  ", submitted: "hello", correct: true },
        ScoreCase { expected: "Hai Based", submitted: " hai   based ", correct: true },
        ScoreCase { expected: "Tokyo", submitted: "Osaka", correct: false },
        ScoreCase { expected: "Tokyo", submitted: "", correct: false },
        ScoreCase { expected: "xin chào", submitted: "Xin chào", correct: true },
        ScoreCase { expected: "Ngôn ngữ", submitted: "ngôn ngữ", correct: true },
    ]
}

/// Generate questions golden vector for cross-validation with Go engine.
pub fn generate_questions_golden() -> GenerateQuestionCase {
    let cards = vec![
        Card { id: 1, term: "Một".into(), definition: "One".into() },
        Card { id: 2, term: "Hai".into(), definition: "Two".into() },
        Card { id: 3, term: "Ba".into(), definition: "Three".into() },
        Card { id: 4, term: "Bốn".into(), definition: "Four".into() },
    ];

    let questions = crate::generate_questions(&cards, 42);
    let first = &questions[0];

    GenerateQuestionCase {
        label: "4 Vietnamese cards seed=42",
        seed: 42,
        cards: cards
            .into_iter()
            .map(|c| GoldenCard {
                id: c.id,
                term: c.term,
                definition: c.definition,
            })
            .collect(),
        result_count: questions.len(),
        first_card_id: first.card.id,
        first_choices: first.choices.clone(),
        first_correct_index: first.correct_index,
    }
}

/// Export all golden vectors to JSON (for cross-validation with Go tests).
pub fn export_golden_vectors() -> serde_json::Value {
    serde_json::json!({
        "version": env!("CARGO_PKG_VERSION"),
        "shuffle": shuffle_golden_vectors(),
        "score_answer": score_answer_golden_vectors(),
        "generate_questions": generate_questions_golden(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::score_answer;

    #[test]
    fn golden_vectors_export() {
        let vectors = export_golden_vectors();
        // Print to stdout for regeneration
        println!(
            "{}",
            serde_json::to_string_pretty(&vectors).unwrap()
        );

        // Basic sanity checks
        let shuffle = vectors["shuffle"]["cases"].as_array().unwrap();
        assert!(shuffle.len() >= 6, "Need at least 6 shuffle cases");

        let score_cases = vectors["score_answer"].as_array().unwrap();
        assert!(score_cases.len() >= 8, "Need at least 8 score cases");
    }

    #[test]
    fn golden_shuffle_seed0_matches_seed1() {
        let items: Vec<u64> = (0..8).collect();
        let a = seeded_shuffle(&items, 0);
        let b = seeded_shuffle(&items, 1);
        assert_eq!(a, b, "seed=0 must map to seed=1");
    }

    #[test]
    fn golden_score_answer_consistency() {
        assert!(score_answer("Tokyo", "tokyo"));
        assert!(score_answer("Xin chào", "xin chào"));
        assert!(!score_answer("Tokyo", "Osaka"));
    }
}
