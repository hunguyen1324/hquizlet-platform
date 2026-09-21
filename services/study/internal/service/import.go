package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

const (
	maxFlashcardImportRows = 5000 // kanji đầy đủ ~3200 rows
	maxQuizImportRows      = 200
)

// ImportService handles Excel import operations.
type ImportService struct {
	flashcards    repository.Flashcards
	quizQuestions repository.QuizQuestions
	sets          repository.StudySets
}

// NewImportService creates a new import service.
func NewImportService(flashcards repository.Flashcards, quizQuestions repository.QuizQuestions, sets repository.StudySets) *ImportService {
	return &ImportService{flashcards: flashcards, quizQuestions: quizQuestions, sets: sets}
}

// ImportFlashcards reads an Excel file and imports flashcards into a study set.
func (s *ImportService) ImportFlashcards(ctx context.Context, studySetID, userID int64, r io.Reader) (model.ImportFlashcardResult, error) {
	// Check ownership
	if err := requireUserID(userID); err != nil {
		return model.ImportFlashcardResult{}, err
	}
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return model.ImportFlashcardResult{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return model.ImportFlashcardResult{}, fmt.Errorf("failed to read file: %w", err)
	}

	items, errors, err := parseFlashcardExcel(data)
	if err != nil {
		return model.ImportFlashcardResult{}, err
	}

	// Bỏ qua dòng lỗi, vẫn import các dòng hợp lệ

	// Lấy max position hiện tại để append đúng thứ tự
	existing, err := s.flashcards.ListByStudySet(ctx, studySetID)
	if err != nil {
		return model.ImportFlashcardResult{}, err
	}
	startPos := len(existing)

	// Chuyển toàn bộ items thành BulkFlashcardItem rồi insert 1 transaction duy nhất
	// Trước đây: 1 INSERT/row → N round-trips DB
	// Sau: tất cả trong 1 transaction, tránh timeout và giảm latency đáng kể
	bulk := make([]model.BulkFlashcardItem, 0, len(items))
	for i, item := range items {
		bi := model.BulkFlashcardItem{
			Term:            item.Term,
			Definition:      item.Definition,
			ExampleSentence: item.ExampleSentence,
			HintExplanation: item.HintExplanation,
			Synonyms:        item.Synonyms,
			Position:        startPos + i,
		}
		if item.ImageURL != "" {
			bi.ImageURL = &item.ImageURL
		}
		bulk = append(bulk, bi)
	}

	result, err := s.flashcards.BulkSave(ctx, studySetID, bulk)
	if err != nil {
		return model.ImportFlashcardResult{}, err
	}

	return model.ImportFlashcardResult{
		Imported: len(result.Created),
		Errors:   errors,
	}, nil
}

// ImportQuiz reads an Excel file and imports quiz questions into a study set.
func (s *ImportService) ImportQuiz(ctx context.Context, studySetID, userID int64, r io.Reader) (model.ImportQuizResult, error) {
	if err := requireUserID(userID); err != nil {
		return model.ImportQuizResult{}, err
	}
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return model.ImportQuizResult{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return model.ImportQuizResult{}, fmt.Errorf("failed to read file: %w", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return model.ImportQuizResult{}, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return model.ImportQuizResult{}, fmt.Errorf("Excel file has no sheets")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return model.ImportQuizResult{}, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(rows) < 2 {
		return model.ImportQuizResult{}, fmt.Errorf("Excel file must have a header row and at least one data row")
	}

	items, errors, questions := parseQuizRows(rows)
	if len(questions) == 0 && len(errors) > 0 {
		return model.ImportQuizResult{Errors: errors}, nil
	}

	// Bulk save (replaces all existing questions)
	if err := s.quizQuestions.BulkSave(ctx, studySetID, questions); err != nil {
		return model.ImportQuizResult{}, err
	}
	existingCards, err := s.flashcards.ListByStudySet(ctx, studySetID)
	if err != nil {
		return model.ImportQuizResult{}, err
	}
	if _, err := s.flashcards.BulkSave(ctx, studySetID, quizRowsToFlashcards(items, existingCards)); err != nil {
		return model.ImportQuizResult{}, err
	}

	return model.ImportQuizResult{
		Imported: len(questions),
		Errors:   errors,
	}, nil
}

func quizRowsToFlashcards(items []model.ImportQuizRow, existing []model.Flashcard) []model.BulkFlashcardItem {
	cards := make([]model.BulkFlashcardItem, 0, len(existing)+len(items))
	for _, card := range existing {
		cards = append(cards, model.BulkFlashcardItem{
			ID:         card.ID,
			Term:       card.Term,
			Definition: card.Definition,
			Delete:     true,
		})
	}
	for i, item := range items {
		cards = append(cards, model.BulkFlashcardItem{
			Term:       item.Question,
			Definition: quizDefinition(item),
			Position:   i,
		})
	}
	return cards
}

func quizDefinition(item model.ImportQuizRow) string {
	for i, opt := range []string{item.OptionA, item.OptionB, item.OptionC, item.OptionD} {
		if opt == "" {
			continue
		}
		if strings.EqualFold(opt, item.CorrectAnswer) ||
			strings.EqualFold(fmt.Sprintf("%c", 'A'+i), item.CorrectAnswer) {
			return opt
		}
	}
	return item.CorrectAnswer
}

// getCell safely returns a cell value, returning "" if out of bounds.
func getCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// countErrors counts errors with a specific field name.
func countErrors(errors []model.ImportError, field string) int {
	count := 0
	for _, e := range errors {
		if e.Field == field {
			count++
		}
	}
	return count
}

func (s *ImportService) checkOwner(ctx context.Context, id, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	ok, err := s.sets.IsOwner(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
