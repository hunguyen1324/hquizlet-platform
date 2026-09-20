package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

const importChunkSize = 200 // rows per DB transaction chunk

// ImportJobService manages async import jobs.
type ImportJobService struct {
	jobs          repository.ImportJobs
	flashcards    repository.Flashcards
	quizQuestions repository.QuizQuestions
	sets          repository.StudySets
}

// NewImportJobService creates a new service.
func NewImportJobService(
	jobs repository.ImportJobs,
	flashcards repository.Flashcards,
	quizQuestions repository.QuizQuestions,
	sets repository.StudySets,
) *ImportJobService {
	return &ImportJobService{jobs: jobs, flashcards: flashcards, quizQuestions: quizQuestions, sets: sets}
}

// EnqueueFlashcards creates a pending job, spawns a background goroutine, and
// returns the job immediately so the HTTP handler can respond at once.
func (s *ImportJobService) EnqueueFlashcards(ctx context.Context, studySetID, userID int64, r io.Reader) (model.ImportJob, error) {
	if err := requireUserID(userID); err != nil {
		return model.ImportJob{}, err
	}
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return model.ImportJob{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return model.ImportJob{}, fmt.Errorf("read file: %w", err)
	}

	job, err := s.jobs.Create(ctx, model.CreateImportJobInput{
		UserID:     userID,
		StudySetID: studySetID,
		Kind:       model.ImportKindFlashcard,
	})
	if err != nil {
		return model.ImportJob{}, fmt.Errorf("create import job: %w", err)
	}

	// Background goroutine – use a detached context so it survives the HTTP request.
	go s.runFlashcardImport(job.ID, studySetID, data)

	return job, nil
}

// EnqueueQuiz creates a pending quiz-import job and starts processing in background.
func (s *ImportJobService) EnqueueQuiz(ctx context.Context, studySetID, userID int64, r io.Reader) (model.ImportJob, error) {
	if err := requireUserID(userID); err != nil {
		return model.ImportJob{}, err
	}
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return model.ImportJob{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return model.ImportJob{}, fmt.Errorf("read file: %w", err)
	}

	job, err := s.jobs.Create(ctx, model.CreateImportJobInput{
		UserID:     userID,
		StudySetID: studySetID,
		Kind:       model.ImportKindQuiz,
	})
	if err != nil {
		return model.ImportJob{}, fmt.Errorf("create import job: %w", err)
	}

	go s.runQuizImport(job.ID, studySetID, data)

	return job, nil
}

// GetJob returns the current state of a job. Callers should verify ownership separately.
func (s *ImportJobService) GetJob(ctx context.Context, jobID int64) (model.ImportJob, error) {
	return s.jobs.Get(ctx, jobID)
}

// ListJobs returns recent jobs for a user (legacy, no pagination).
func (s *ImportJobService) ListJobs(ctx context.Context, userID int64) ([]model.ImportJob, error) {
	return s.jobs.ListByUser(ctx, userID, 20)
}

// ListJobsWithFilter returns paginated import jobs for a user with optional filters.
func (s *ImportJobService) ListJobsWithFilter(ctx context.Context, userID int64, f model.ImportJobFilter) (model.ImportJobListResult, error) {
	if userID <= 0 {
		return model.ImportJobListResult{}, ErrUnauthorized
	}
	return s.jobs.ListWithFilter(ctx, userID, f)
}

// ---------------------------------------------------------------------------
// Background workers
// ---------------------------------------------------------------------------

func (s *ImportJobService) runFlashcardImport(jobID, studySetID int64, data []byte) {
	ctx := context.Background()
	log := slog.With("job_id", jobID, "study_set_id", studySetID, "kind", "flashcard")

	// Mark running
	if _, err := s.jobs.Update(ctx, jobID, model.UpdateImportJobInput{Status: model.ImportStatusRunning}); err != nil {
		log.Error("mark running failed", "err", err)
		return
	}

	items, parseErrors, err := parseFlashcardExcel(data)
	if err != nil {
		s.failJob(ctx, jobID, err.Error(), log)
		return
	}

	// Tiếp tục import các dòng hợp lệ, bỏ qua dòng lỗi

	// Determine start position
	existing, err := s.flashcards.ListByStudySet(ctx, studySetID)
	if err != nil {
		s.failJob(ctx, jobID, err.Error(), log)
		return
	}
	startPos := len(existing)
	total := len(items)
	imported := 0

	// Process in chunks so progress updates are visible to the frontend.
	for start := 0; start < len(items); start += importChunkSize {
		end := start + importChunkSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[start:end]

		bulk := make([]model.BulkFlashcardItem, len(chunk))
		for i, item := range chunk {
			bi := model.BulkFlashcardItem{
				Term:            item.Term,
				Definition:      item.Definition,
				ExampleSentence: item.ExampleSentence,
				HintExplanation: item.HintExplanation,
				Synonyms:        item.Synonyms,
				Position:        startPos + start + i,
			}
			if item.ImageURL != "" {
				bi.ImageURL = &item.ImageURL
			}
			bulk[i] = bi
		}

		result, err := s.flashcards.BulkSave(ctx, studySetID, bulk)
		if err != nil {
			s.failJob(ctx, jobID, err.Error(), log)
			return
		}
		imported += len(result.Created)

		// Update progress after each chunk.
		if _, err := s.jobs.Update(ctx, jobID, model.UpdateImportJobInput{
			Status:   model.ImportStatusRunning,
			Total:    total,
			Imported: imported,
		}); err != nil {
			log.Warn("progress update failed", "err", err)
		}

		// Small yield between chunks to avoid starving the DB.
		time.Sleep(5 * time.Millisecond)
	}

	_, _ = s.jobs.Update(ctx, jobID, model.UpdateImportJobInput{
		Status:   model.ImportStatusDone,
		Total:    total + len(parseErrors),
		Imported: imported,
		Errors:   parseErrors,
	})
	log.Info("flashcard import done", "imported", imported, "total", total, "skipped", len(parseErrors))
}

func (s *ImportJobService) runQuizImport(jobID, studySetID int64, data []byte) {
	ctx := context.Background()
	log := slog.With("job_id", jobID, "study_set_id", studySetID, "kind", "quiz")

	if _, err := s.jobs.Update(ctx, jobID, model.UpdateImportJobInput{Status: model.ImportStatusRunning}); err != nil {
		log.Error("mark running failed", "err", err)
		return
	}

	// Quiz import: parse then bulk save — ownership was already checked in EnqueueQuiz.
	imported, importErrors, fatalErr := runQuizImportData(ctx, studySetID, data, s.flashcards, s.quizQuestions)
	if fatalErr != nil {
		s.failJob(ctx, jobID, fatalErr.Error(), log)
		return
	}

	total := imported + len(importErrors)
	_, _ = s.jobs.Update(ctx, jobID, model.UpdateImportJobInput{
		Status:   model.ImportStatusDone,
		Total:    total,
		Imported: imported,
		Errors:   importErrors,
	})
	log.Info("quiz import done", "imported", imported)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *ImportJobService) failJob(ctx context.Context, jobID int64, msg string, log *slog.Logger) {
	log.Error("import job failed", "err", msg)
	_, _ = s.jobs.Update(ctx, jobID, model.UpdateImportJobInput{
		Status:   model.ImportStatusFailed,
		ErrorMsg: msg,
	})
}

func (s *ImportJobService) checkOwner(ctx context.Context, studySetID, userID int64) error {
	ok, err := s.sets.IsOwner(ctx, studySetID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// runQuizImportData parses and saves quiz questions without an ownership check.
// It is the internal implementation used by the async worker.
func runQuizImportData(
	ctx context.Context,
	studySetID int64,
	data []byte,
	fc repository.Flashcards,
	qq repository.QuizQuestions,
) (imported int, importErrors []model.ImportError, fatalErr error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return 0, nil, fmt.Errorf("open excel: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return 0, nil, fmt.Errorf("excel file has no sheets")
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, nil, fmt.Errorf("read sheet: %w", err)
	}
	if len(rows) < 2 {
		return 0, nil, fmt.Errorf("excel file must have a header row and at least one data row")
	}

	items, errs, questions := parseQuizRows(rows)
	if len(errs) > 0 {
		return 0, errs, nil
	}

	if err := qq.BulkSave(ctx, studySetID, questions); err != nil {
		return 0, nil, err
	}
	existingCards, err := fc.ListByStudySet(ctx, studySetID)
	if err != nil {
		return 0, nil, err
	}
	if _, err := fc.BulkSave(ctx, studySetID, quizRowsToFlashcards(items, existingCards)); err != nil {
		return 0, nil, err
	}
	return len(questions), nil, nil
}

// parseQuizRows is a package-level helper shared by ImportService and the async worker.
func parseQuizRows(rows [][]string) (items []model.ImportQuizRow, errs []model.ImportError, questions []model.CreateQuizQuestionInput) {
	header := make(map[string]int)
	for i, cell := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(cell))] = i
	}

	qIdx, hasQ := header["question"]
	typeIdx, hasType := header["type"]
	correctIdx, hasCorrect := header["correct answer"]
	if !hasQ || !hasType || !hasCorrect {
		errs = append(errs, model.ImportError{Row: 1, Field: "header", Reason: "must have 'Question', 'Type', and 'Correct Answer' columns"})
		return
	}

	optAIdx := header["option a"]
	optBIdx := header["option b"]
	optCIdx := header["option c"]
	optDIdx := header["option d"]
	timeIdx := header["time (s)"]
	audioIdx := header["audio url"]
	explainIdx := header["answer explanation"]

	typeMap := map[string]string{
		"MC": "multiple_choice", "TF": "true_false",
		"WR": "written", "PG": "paragraph", "SO": "sorting",
	}

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}
		if len(items) >= maxQuizImportRows {
			errs = append(errs, model.ImportError{Row: rowIdx + 1, Field: "row", Reason: fmt.Sprintf("exceeds maximum of %d rows", maxQuizImportRows)})
			break
		}

		question := getCell(row, qIdx)
		typeCode := strings.ToUpper(strings.TrimSpace(getCell(row, typeIdx)))
		correctAnswer := getCell(row, correctIdx)

		if question == "" {
			errs = append(errs, model.ImportError{Row: rowIdx + 1, Field: "Question", Reason: "Question is required"})
			continue
		}
		mappedType, ok := typeMap[typeCode]
		if !ok {
			errs = append(errs, model.ImportError{Row: rowIdx + 1, Field: "Type", Reason: fmt.Sprintf("unknown type '%s'. Use MC, TF, WR, PG, or SO", typeCode)})
			continue
		}
		if correctAnswer == "" {
			errs = append(errs, model.ImportError{Row: rowIdx + 1, Field: "Correct Answer", Reason: "Correct Answer is required"})
			continue
		}

		item := model.ImportQuizRow{
			Row: rowIdx + 1, Question: question, Type: mappedType,
			OptionA: getCell(row, optAIdx), OptionB: getCell(row, optBIdx),
			OptionC: getCell(row, optCIdx), OptionD: getCell(row, optDIdx),
			CorrectAnswer: correctAnswer, AudioURL: getCell(row, audioIdx),
			AnswerExplanation: getCell(row, explainIdx),
		}
		if t := getCell(row, timeIdx); t != "" {
			n := 0
			fmt.Sscanf(t, "%d", &n)
			item.TimeSeconds = n
		}
		items = append(items, item)
	}

	if len(errs) > 0 {
		return items, errs, nil
	}

	// Convert to CreateQuizQuestionInput
	for i, item := range items {
		var opts []model.CreateOptionInput
		if item.Type == "multiple_choice" {
			for j, optText := range []string{item.OptionA, item.OptionB, item.OptionC, item.OptionD} {
				if optText != "" {
					isCorrect := strings.EqualFold(optText, item.CorrectAnswer) ||
						strings.EqualFold(fmt.Sprintf("%c", 'A'+j), item.CorrectAnswer)
					opts = append(opts, model.CreateOptionInput{Text: optText, Position: j, IsCorrect: isCorrect})
				}
			}
		}
		correctStr := item.CorrectAnswer
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
			Position: i, QuestionText: item.Question, QuestionType: item.Type,
			CorrectAnswer: &correctStr, TimeInSeconds: timeSec,
			AudioURL: audioURL, AnswerExplanation: explain, Options: opts,
		})
	}
	return
}

// parseFlashcardExcel parses the Excel bytes and returns valid items + row errors.
func parseFlashcardExcel(data []byte) ([]model.ImportFlashcardRow, []model.ImportError, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("open excel: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, nil, fmt.Errorf("excel file has no sheets")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("read sheet: %w", err)
	}
	if len(rows) < 2 {
		return nil, nil, fmt.Errorf("excel file must have a header row and at least one data row")
	}

	header := make(map[string]int)
	for i, cell := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(cell))] = i
	}

	termIdx, hasTerm := header["term"]
	defIdx, hasDef := header["definition"]
	if !hasTerm || !hasDef {
		return nil, nil, fmt.Errorf("excel file must have 'Term' and 'Definition' columns")
	}

	exIdx := header["example"]
	hintIdx := header["hint"]
	synIdx := header["synonyms"]
	imgIdx := header["image url"]

	var items []model.ImportFlashcardRow
	var errs []model.ImportError

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}
		if len(items) >= maxFlashcardImportRows {
			errs = append(errs, model.ImportError{
				Row:    rowIdx + 1,
				Field:  "row",
				Reason: fmt.Sprintf("exceeds maximum of %d rows", maxFlashcardImportRows),
			})
			break
		}

		term := getCell(row, termIdx)
		def := getCell(row, defIdx)

		if term == "" {
			errs = append(errs, model.ImportError{Row: rowIdx + 1, Field: "Term", Reason: "Term is required"})
			continue
		}
		if def == "" {
			errs = append(errs, model.ImportError{Row: rowIdx + 1, Field: "Definition", Reason: "Definition is required"})
			continue
		}

		items = append(items, model.ImportFlashcardRow{
			Row:             rowIdx + 1,
			Term:            term,
			Definition:      def,
			ExampleSentence: getCell(row, exIdx),
			HintExplanation: getCell(row, hintIdx),
			Synonyms:        getCell(row, synIdx),
			ImageURL:        getCell(row, imgIdx),
		})
	}

	return items, errs, nil
}
