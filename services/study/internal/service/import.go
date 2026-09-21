package service

import (
	"bytes"
	"context"
	"encoding/json"
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
	subQIdx := header["sub questions (json)"] // cột mới cho PG subQuestions
	pgTextIdx := header["paragraph text"]     // cột backup cho PG paragraphText

	var items []model.ImportQuizRow
	errors := []model.ImportError{}

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
		// PG (paragraph/reading) không cần correct_answer vì đây là câu cha chứa sub-questions.
		// WR (written/sorting) dùng correct_answer là chuỗi thứ tự (ví dụ "3241"), cho phép rỗng.
		if correctAnswer == "" && typeCode != "PG" && typeCode != "WR" {
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
			SubQuestionsJSON:  getCell(row, subQIdx),
			ParagraphText:     getCell(row, pgTextIdx),
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

		switch item.Type {
		case "multiple_choice":
			// MC: options A/B/C/D, correct = chữ cái hoặc text khớp
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

		case "sorting":
			// SO: options A/B/C/D là các mục cần sắp xếp (thứ tự đúng từ trên xuống).
			// correctAnswer từ export = "A → B → C" hoặc thứ tự text join.
			// Lưu options theo đúng thứ tự; correctAnswer giữ nguyên để frontend dùng.
			for j, optText := range []string{item.OptionA, item.OptionB, item.OptionC, item.OptionD} {
				if optText != "" {
					opts = append(opts, model.CreateOptionInput{
						Text:     optText,
						Position: j,
					})
				}
			}
			// Nếu correctAnswer rỗng nhưng có options, tự build từ options (thứ tự đã đúng)
			if item.CorrectAnswer == "" && len(opts) > 0 {
				parts := make([]string, len(opts))
				for k, o := range opts {
					parts[k] = o.Text
				}
				item.CorrectAnswer = strings.Join(parts, " → ")
			}
		}

		var correct *string
		if item.CorrectAnswer != "" {
			correctStr := item.CorrectAnswer
			correct = &correctStr
		}

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

		input := model.CreateQuizQuestionInput{
			Position:          i,
			QuestionText:      item.Question,
			QuestionType:      item.Type,
			CorrectAnswer:     correct,
			TimeInSeconds:     timeSec,
			AudioURL:          audioURL,
			AnswerExplanation: explain,
			Options:           opts,
		}

		// PG (paragraph/reading comprehension):
		// - Cột Question chứa nội dung đoạn văn (paragraphText) — từ export hquizlet v2+
		//   hoặc cột "Paragraph Text" nếu file cũ hơn dùng cột riêng.
		// - SubQuestionsJSON chứa mảng câu hỏi con serialized.
		if item.Type == "paragraph" {
			pgText := item.Question // v2+: Question = paragraphText
			if item.ParagraphText != "" {
				pgText = item.ParagraphText // cột riêng có độ ưu tiên cao hơn nếu tồn tại
			}
			if pgText != "" {
				input.ParagraphText = &pgText
			}
			input.QuestionText = "" // PG không dùng questionText

			if item.SubQuestionsJSON != "" {
				input.SubQuestions = json.RawMessage(item.SubQuestionsJSON)
			}
		}

		questions = append(questions, input)
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
