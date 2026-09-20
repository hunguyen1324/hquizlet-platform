package engine

import (
	"fmt"
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
	a, err := Generate(testCards(), "test", 42, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Generate(testCards(), "test", 42, 100, 0)
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
	items, _ := Generate(testCards(), "test", 42, 100, 0)
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
	items, err := Generate(cards, "match", 42, 1, 0)
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
	items, err := Generate(cards, "match", 42, 1, 0)
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

func TestGenerateOffsetBatching(t *testing.T) {
	// Tạo 10 thẻ giả
	cards := make([]studyclient.Flashcard, 10)
	for i := range cards {
		cards[i] = studyclient.Flashcard{ID: int64(i + 1), Term: fmt.Sprintf("Term%d", i+1), Definition: fmt.Sprintf("Def%d", i+1)}
	}

	seed := uint64(12345)
	batch1, err := Generate(cards, "flashcards", seed, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	batch2, err := Generate(cards, "flashcards", seed, 5, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Hai batch không được có thẻ trùng
	ids1 := make(map[int64]bool)
	for _, item := range batch1 {
		ids1[item.FlashcardID] = true
	}
	for _, item := range batch2 {
		if ids1[item.FlashcardID] {
			t.Fatalf("batch2 chứa thẻ trùng với batch1: flashcardId=%d", item.FlashcardID)
		}
	}

	// Tổng cộng phải cover toàn bộ 10 thẻ
	if len(batch1)+len(batch2) != 10 {
		t.Fatalf("expected 10 total items, got %d+%d", len(batch1), len(batch2))
	}

	// Deterministic: cùng seed+offset cho cùng kết quả
	batch1b, _ := Generate(cards, "flashcards", seed, 5, 0)
	if !reflect.DeepEqual(batch1, batch1b) {
		t.Fatal("offset=0 không deterministic với cùng seed")
	}

	// Offset vượt quá deck → trả rỗng, không lỗi
	empty, err := Generate(cards, "flashcards", seed, 5, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatal("expected empty slice khi offset >= len(cards)")
	}
}
