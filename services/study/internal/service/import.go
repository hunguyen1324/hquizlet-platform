package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

const (
	maxFlashcardImportRows = 500
	maxQuizImportRows      = 200
)

// ImportService handles Excel import operations.
type ImportService struct {
	flashcards  repository.Flashcards
	quizQuestions repository.QuizQuestions
	sets        repository.StudySets
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

	// Read the entire file into a temp file for excelize
	data, err := io.ReadAll(r)
	if err != nil {
		return model.ImportFlashcardResult{}, fmt.Errorf("failed to read file: %w", err)
	}

	f, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		return model.ImportFlashcardResult{}, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get the first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return model.ImportFlashcardResult{}, fmt.Errorf("Excel file has no sheets")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return model.ImportFlashcardResult{}, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(rows) < 2 {
		return model.ImportFlashcardResult{}, fmt.Errorf("Excel file must have a header row and at least one data row")
	}

	// Parse header row (case-insensitive)
	header := make(map[string]int)
	for i, cell := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(cell))] = i
	}

	// Validate required columns
	termIdx, hasTerm := header["term"]
	defIdx, hasDef := header["definition"]
	if !hasTerm || !hasDef {
		return model.ImportFlashcardResult{}, fmt.Errorf("Excel file must have 'Term' and 'Definition' columns")
	}

	// Optional column indices
	exIdx := header["example"]
	hintIdx := header["hint"]
	synIdx := header["synonyms"]
	imgIdx := header["image url"]

	var items []model.ImportFlashcardRow
	var errors []model.ImportError

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue // skip empty rows
		}

		if len(items) >= maxFlashcardImportRows {
			errors = append(errors, model.ImportError{
				Row:    rowIdx + 1,
				Field:  "row",
				Reason: fmt.Sprintf("exceeds maximum of %d rows", maxFlashcardImportRows),
			})
			break
		}

		term := getCell(row, termIdx)
		def := getCell(row, defIdx)

		if term == "" {
			errors = append(errors, model.ImportError{Row: rowIdx + 1, Field: "Term", Reason: "Term is required"})
			continue
		}
		if def == "" {
			errors = append(errors, model.ImportError{Row: rowIdx + 1, Field: "Definition", Reason: "Definition is required"})
			continue
		}

		item := model.ImportFlashcardRow{
			Row:             rowIdx + 1,
			Term:            term,
			Definition:      def,
			ExampleSentence: getCell(row, exIdx),
			HintExplanation: getCell(row, hintIdx),
			Synonyms:        getCell(row, synIdx),
			ImageURL:        getCell(row, imgIdx),
		}
		items = append(items, item)
	}

	// Get current max position
	existing, err := s.flashcards.ListByStudySet(ctx, studySetID)
	if err != nil {
		return model.ImportFlashcardResult{}, err
	}
	startPos := len(existing)

	// Batch create flashcards
	for i, item := range items {
		in := model.CreateFlashcardInput{
			Term:            item.Term,
			Definition:      item.Definition,
			ExampleSentence: item.ExampleSentence,
			HintExplanation: item.HintExplanation,
			Synonyms:        item.Synonyms,
		}
		if item.ImageURL != "" {
			in.ImageURL = &item.ImageURL
		}
		_ = startPos + i // position tracking
		if _, err := s.flashcards.Create(ctx, studySetID, in); err != nil {
			errors = append(errors, model.ImportError{
				Row:    item.Row,
				Field:  "create",
				Reason: err.Error(),
			})
		}
	}

	return model.ImportFlashcardResult{
		Imported: len(items) - countErrors(errors, "create"),
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

	f, err := excelize.OpenReader(strings.NewReader(string(data)))
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

	// Parse header
	header := make(map[string]int)
	for i, cell := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(cell))] = i
	}

	qIdx, hasQ := header["question"]
	typeIdx, hasType := header["type"]
	correctIdx, hasCorrect := header["correct answer"]
	if !hasQ || !hasType || !hasCorrect {
		return model.ImportQuizResult{}, fmt.Errorf("Excel file must have 'Question', 'Type', and 'Correct Answer' columns")
	}

	optAIdx := header["option a"]
	optBIdx := header["option b"]
	optCIdx := header["option c"]
	optDIdx := header["option d"]
	timeIdx := header["time (s)"]
	audioIdx := header["audio url"]
	explainIdx := header["answer explanation"]

	var items []model.ImportQuizRow
	var errors []model.ImportError

	typeMap := map[string]string{
		"MC": "multiple_choice",
		"TF": "true_false",
		"WR": "written",
		"PG": "paragraph",
		"SO": "sorting",
	}

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		if len(items) >= maxQuizImportRows {
			errors = append(errors, model.ImportError{
				Row:    rowIdx + 1,
				Field:  "row",
				Reason: fmt.Sprintf("exceeds maximum of %d rows", maxQuizImportRows),
			})
			break
		}

		question := getCell(row, qIdx)
		typeCode := strings.ToUpper(strings.TrimSpace(getCell(row, typeIdx)))
		correctAnswer := getCell(row, correctIdx)

		if question == "" {
			errors = append(errors, model.ImportError{Row: rowIdx + 1, Field: "Question", Reason: "Question is required"})
			continue
		}
		if _, ok := typeMap[typeCode]; !ok {
			errors = append(errors, model.ImportError{Row: rowIdx + 1, Field: "Type", Reason: fmt.Sprintf("Unknown type '%s'. Use MC, TF, WR, PG, or SO", typeCode)})
			continue
		}
		if correctAnswer == "" {
			errors = append(errors, model.ImportError{Row: rowIdx + 1, Field: "Correct Answer", Reason: "Correct Answer is required"})
			continue
		}

		item := model.ImportQuizRow{
			Row:               rowIdx + 1,
			Question:          question,
			Type:              typeMap[typeCode],
			OptionA:           getCell(row, optAIdx),
			OptionB:           getCell(row, optBIdx),
			OptionC:           getCell(row, optCIdx),
			OptionD:           getCell(row, optDIdx),
			CorrectAnswer:     correctAnswer,
			AudioURL:          getCell(row, audioIdx),
			AnswerExplanation: getCell(row, explainIdx),
		}
		if t := getCell(row, timeIdx); t != "" {
			n := 0
			fmt.Sscanf(t, "%d", &n)
			item.TimeSeconds = n
		}
		items = append(items, item)
	}

	// Convert to CreateQuizQuestionInput
	var questions []model.CreateQuizQuestionInput
	for i, item := range items {
		var opts []model.CreateOptionInput
		if item.Type == "multiple_choice" {
			for j, optText := range []string{item.OptionA, item.OptionB, item.OptionC, item.OptionD} {
				if optText != "" {
					isCorrect := strings.EqualFold(optText, item.CorrectAnswer) ||
						strings.EqualFold(fmt.Sprintf("%c", 'A'+j), item.CorrectAnswer)
					opts = append(opts, model.CreateOptionInput{
						Text:      optText,
						Position:  j,
						IsCorrect: isCorrect,
					})
				}
			}
		}

		var correct *string
		correctStr := item.CorrectAnswer
		correct = &correctStr

		var timeSec *int
		if item.TimeSeconds > 0 {
			timeSec = &item.TimeSeconds
		}

		var audioURL *string
		if item.AudioURL != "" {
			audioURL = &item.AudioURL
		}

		var explain *string
		if item.AnswerExplanation != "" {
			explain = &item.AnswerExplanation
		}

		questions = append(questions, model.CreateQuizQuestionInput{
			Position:          i,
			QuestionText:      item.Question,
			QuestionType:      item.Type,
			CorrectAnswer:     correct,
			TimeInSeconds:     timeSec,
			AudioURL:          audioURL,
			AnswerExplanation: explain,
			Options:           opts,
		})
	}

	// Bulk save (replaces all existing questions)
	if err := s.quizQuestions.BulkSave(ctx, studySetID, questions); err != nil {
		return model.ImportQuizResult{}, err
	}

	return model.ImportQuizResult{
		Imported: len(questions),
		Errors:   errors,
	}, nil
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
