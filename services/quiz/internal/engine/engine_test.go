package engine

import (
	"reflect"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/studyclient"
)

func testCards() []studyclient.Flashcard {
	return []studyclient.Flashcard{
		{ID: 1, Term: "Một", Definition: "One"},
		{ID: 2, Term: "Hai", Definition: "Two"},
		{ID: 3, Term: "Ba", Definition: "Three"},
		{ID: 4, Term: "Bốn", Definition: "Four"},
	}
}

func TestGenerateDeterministicAndDoesNotLeakTestAnswer(t *testing.T) {
	a, err := Generate(testCards(), "test", 42, 100)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Generate(testCards(), "test", 42, 100)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed produced different questions")
	}
	for _, item := range a {
		if item.Definition != "" {
			t.Fatalf("test response leaked definition for card %d", item.FlashcardID)
		}
		if len(item.Choices) != 4 {
			t.Fatalf("expected four unique choices, got %d", len(item.Choices))
		}
	}
}

func TestEvaluateLearnNormalizesAndRejectsForeignCard(t *testing.T) {
	results, err := Evaluate(testCards(), "learn", 7, 4, []Answer{{FlashcardID: 1, Submitted: "  ONE  ", Attempts: 2}})
	if err != nil || len(results) != 1 || !results[0].Correct {
		t.Fatalf("expected normalized correct answer: %#v, %v", results, err)
	}
	_, err = Evaluate(testCards(), "learn", 7, 4, []Answer{{FlashcardID: 999, Submitted: "One", Attempts: 1}})
	if !errorsIsInvalid(err) {
		t.Fatalf("expected invalid foreign card, got %v", err)
	}
}

func TestEvaluateTestUsesGeneratedChoiceIndex(t *testing.T) {
	items, _ := Generate(testCards(), "test", 42, 100)
	item := items[0]
	card := map[int64]string{1: "One", 2: "Two", 3: "Three", 4: "Four"}[item.FlashcardID]
	selected := -1
	for i, choice := range item.Choices {
		if choice == card {
			selected = i
		}
	}
	results, err := Evaluate(testCards(), "test", 42, 4, []Answer{
		{FlashcardID: item.FlashcardID, SelectedIndex: &selected, Attempts: 1},
		{FlashcardID: items[1].FlashcardID, SelectedIndex: &selected, Attempts: 1},
		{FlashcardID: items[2].FlashcardID, SelectedIndex: &selected, Attempts: 1},
		{FlashcardID: items[3].FlashcardID, SelectedIndex: &selected, Attempts: 1},
	})
	if err != nil || !results[0].Correct {
		t.Fatalf("generated selectedIndex must score correctly: %#v, %v", results, err)
	}
}

func errorsIsInvalid(err error) bool   { return err == ErrInvalid }
func errorsIsDuplicate(err error) bool { return err == ErrDuplicate }

func TestEvaluateRejectsDuplicateFlashcardID(t *testing.T) {
	_, err := Evaluate(testCards(), "learn", 1, 4, []Answer{
		{FlashcardID: 1, Submitted: "One", Attempts: 1},
		{FlashcardID: 1, Submitted: "One", Attempts: 1},
	})
	if !errorsIsDuplicate(err) {
		t.Fatalf("expected ErrDuplicate for repeated flashcardId, got %v", err)
	}
}

func TestMatchEvaluateRejectsSpoofedPair(t *testing.T) {
	// Card exists in study set but pairId is wrong
	cards := testCards()
	items, err := Generate(cards, "match", 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	cardID := items[0].FlashcardID
	results, err := Evaluate(cards, "match", 42, 1, []Answer{
		{FlashcardID: cardID, PairID: "spoofed", MatchedFlashcardID: cardID, Attempts: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Correct {
		t.Errorf("expected incorrect for spoofed pairId, got %+v", results)
	}
}

func TestEvaluateRejectsAnswerOutsideGeneratedSubset(t *testing.T) {
	cards := testCards()
	items, err := Generate(cards, "match", 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	allowed := items[0].FlashcardID
	outside := int64(1)
	if outside == allowed {
		outside = 2
	}
	_, err = Evaluate(cards, "match", 42, 1, []Answer{
		{FlashcardID: outside, PairID: "card-" + id(outside), MatchedFlashcardID: outside, Attempts: 1},
	})
	if !errorsIsInvalid(err) {
		t.Fatalf("expected invalid answer outside generated subset, got %v", err)
	}
}
