pub fn score_answer(expected: &str, submitted: &str) -> bool {
    expected.trim().eq_ignore_ascii_case(submitted.trim())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn scores_case_insensitive_answers() {
        assert!(score_answer("Tokyo", " tokyo "));
    }
}

