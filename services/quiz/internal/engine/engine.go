package engine

import (
	"errors"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/studyclient"
)

const ContractVersion = "1.4.0"

type Item struct {
	ID          string   `json:"id"`
	FlashcardID int64    `json:"flashcardId"`
	Kind        string   `json:"kind"`
	Text        string   `json:"text,omitempty"`
	Term        string   `json:"term,omitempty"`
	Definition  string   `json:"definition,omitempty"`
	Choices     []string `json:"choices,omitempty"`
	PairID      string   `json:"pairId,omitempty"`
	Starred     bool     `json:"starred"`
	Position    int      `json:"position"`
}

type Answer struct {
	FlashcardID        int64  `json:"flashcardId"`
	Submitted          string `json:"submitted,omitempty"`
	SelectedIndex      *int   `json:"selectedIndex,omitempty"`
	PairID             string `json:"pairId,omitempty"`
	MatchedFlashcardID int64  `json:"matchedFlashcardId,omitempty"`
	Attempts           int    `json:"attempts"`
	ResponseTimeMS     int    `json:"responseTimeMs,omitempty"`
}

type CardResult struct {
	FlashcardID    int64 `json:"flashcardId"`
	Correct        bool  `json:"correct"`
	Attempts       int   `json:"attempts"`
	ResponseTimeMS int   `json:"responseTimeMs,omitempty"`
}

var (
	ErrInvalid   = errors.New("invalid quiz input")
	ErrDuplicate = errors.New("duplicate flashcardId in answers")
)

func Generate(cards []studyclient.Flashcard, mode string, seed uint64, limit int) ([]Item, error) {
	if !validMode(mode) || limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	deck := shuffle(cards, seed)
	if len(deck) > limit {
		deck = deck[:limit]
	}
	items := make([]Item, 0, len(deck)*2)
	for i, card := range deck {
		base := Item{ID: id(card.ID), FlashcardID: card.ID, Term: card.Term, Definition: card.Definition, Starred: card.Starred, Position: i}
		switch mode {
		case "flashcards", "learn":
			base.Kind = "question"
			items = append(items, base)
		case "test":
			base.Kind = "question"
			base.Definition = ""
			base.Choices = choices(cards, card, seed+uint64(i)+1)
			items = append(items, base)
		case "match":
			pair := "card-" + id(card.ID)
			items = append(items,
				Item{ID: pair + "-term", FlashcardID: card.ID, Kind: "term", Text: card.Term, PairID: pair, Position: i},
				Item{ID: pair + "-definition", FlashcardID: card.ID, Kind: "definition", Text: card.Definition, PairID: pair, Position: i})
		}
	}
	if mode == "match" {
		items = shuffle(items, seed^0x9e3779b97f4a7c15)
	}
	return items, nil
}

func Evaluate(cards []studyclient.Flashcard, mode string, seed uint64, limit int, answers []Answer) ([]CardResult, error) {
	if !validMode(mode) || limit < 1 || limit > 100 || len(answers) > 100 {
		return nil, ErrInvalid
	}

	generated, err := Generate(cards, mode, seed, limit)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]bool, len(generated))
	expectedCards := 0
	for _, item := range generated {
		if !allowed[item.FlashcardID] {
			expectedCards++
		}
		allowed[item.FlashcardID] = true
	}
	if mode != "learn" && len(answers) != expectedCards {
		return nil, ErrInvalid
	}

	seen := make(map[int64]bool, len(answers))
	for _, a := range answers {
		if seen[a.FlashcardID] {
			return nil, ErrDuplicate
		}
		if !allowed[a.FlashcardID] {
			return nil, ErrInvalid
		}
		seen[a.FlashcardID] = true
	}

	byID := make(map[int64]studyclient.Flashcard, len(cards))
	for _, c := range cards {
		byID[c.ID] = c
	}
	results := make([]CardResult, 0, len(answers))
	testChoices := map[int64][]string{}
	if mode == "test" {
		for _, item := range generated {
			testChoices[item.FlashcardID] = item.Choices
		}
	}
	for _, a := range answers {
		card, ok := byID[a.FlashcardID]
		if !ok || a.Attempts < 1 || a.Attempts > 100 || a.ResponseTimeMS < 0 {
			return nil, ErrInvalid
		}
		correct := false
		switch mode {
		case "flashcards":
			correct = true
		case "learn":
			correct = normalize(a.Submitted) == normalize(card.Definition)
		case "test":
			if a.SelectedIndex == nil {
				return nil, ErrInvalid
			}
			cs := testChoices[card.ID]
			correct = *a.SelectedIndex >= 0 && *a.SelectedIndex < len(cs) && cs[*a.SelectedIndex] == card.Definition
		case "match":
			correct = a.PairID == "card-"+id(card.ID) && a.MatchedFlashcardID == card.ID
		}
		results = append(results, CardResult{FlashcardID: card.ID, Correct: correct, Attempts: a.Attempts, ResponseTimeMS: a.ResponseTimeMS})
	}
	return results, nil
}

func validMode(mode string) bool {
	return mode == "flashcards" || mode == "learn" || mode == "test" || mode == "match"
}
func normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func id(n int64) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	b := make([]byte, 0, 20)
	for n > 0 {
		b = append(b, digits[n%10])
		n /= 10
	}
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}
func choices(cards []studyclient.Flashcard, target studyclient.Flashcard, seed uint64) []string {
	unique := []string{target.Definition}
	seen := map[string]bool{target.Definition: true}
	for _, c := range shuffle(cards, seed) {
		if c.ID != target.ID && !seen[c.Definition] {
			unique = append(unique, c.Definition)
			seen[c.Definition] = true
			if len(unique) == 4 {
				break
			}
		}
	}
	return shuffle(unique, seed^0xd1b54a32d192ed03)
}

func shuffle[T any](in []T, seed uint64) []T {
	out := append([]T(nil), in...)
	if seed == 0 {
		seed = 1
	}
	for i := len(out) - 1; i > 0; i-- {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		j := int(seed % uint64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	return out
}
