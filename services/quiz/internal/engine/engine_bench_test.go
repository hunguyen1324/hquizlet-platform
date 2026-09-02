package engine

import (
	"fmt"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/studyclient"
)

func BenchmarkGenerate(b *testing.B) {
	for _, size := range []int{10, 100, 1_000, 10_000} {
		cards := benchCards(size)
		limit := size
		if limit > 100 {
			limit = 100
		}
		b.Run(fmt.Sprintf("%d_cards_limit_%d", size, limit), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				items, err := Generate(cards, "test", 42, limit)
				if err != nil {
					b.Fatal(err)
				}
				if len(items) == 0 {
					b.Fatal("expected generated items")
				}
			}
		})
	}
}

func BenchmarkEvaluate(b *testing.B) {
	for _, size := range []int{10, 100, 1_000, 10_000} {
		cards := benchCards(size)
		limit := size
		if limit > 100 {
			limit = 100
		}
		items, err := Generate(cards, "match", 42, limit)
		if err != nil {
			b.Fatal(err)
		}
		answers := make([]Answer, 0, limit)
		seen := map[int64]bool{}
		for _, item := range items {
			if item.Kind != "term" || seen[item.FlashcardID] {
				continue
			}
			seen[item.FlashcardID] = true
			answers = append(answers, Answer{
				FlashcardID:        item.FlashcardID,
				PairID:             item.PairID,
				MatchedFlashcardID: item.FlashcardID,
				Attempts:           1,
			})
		}
		b.Run(fmt.Sprintf("%d_cards_limit_%d", size, limit), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				results, err := Evaluate(cards, "match", 42, limit, answers)
				if err != nil {
					b.Fatal(err)
				}
				if len(results) != limit {
					b.Fatalf("expected %d results, got %d", limit, len(results))
				}
			}
		})
	}
}

func benchCards(size int) []studyclient.Flashcard {
	cards := make([]studyclient.Flashcard, size)
	for i := range cards {
		id := int64(i + 1)
		cards[i] = studyclient.Flashcard{
			ID:         id,
			StudySetID: 1,
			Term:       fmt.Sprintf("term-%d", id),
			Definition: fmt.Sprintf("definition-%d", id),
			Position:   i,
		}
	}
	return cards
}
