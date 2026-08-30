#[derive(Debug, PartialEq, Eq)]
pub struct ImportedCard {
    pub term: String,
    pub definition: String,
}

pub fn parse_tsv(input: &str) -> Vec<ImportedCard> {
    input
        .lines()
        .filter_map(|line| {
            let (term, definition) = line.split_once('\t')?;
            Some(ImportedCard {
                term: term.trim().to_string(),
                definition: definition.trim().to_string(),
            })
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_basic_tsv_cards() {
        assert_eq!(
            parse_tsv("go\tbackend\nrust\tcore"),
            vec![
                ImportedCard {
                    term: "go".into(),
                    definition: "backend".into()
                },
                ImportedCard {
                    term: "rust".into(),
                    definition: "core".into()
                }
            ]
        );
    }
}

