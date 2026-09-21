package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseFlashcardExcelAcceptsKanjiHeaders(t *testing.T) {
	data := flashcardWorkbook(t, []string{"Kanji", "Meaning", "Example Sentence", "Image_URL"}, []string{"日", "sun; day", "日曜日", "https://example.com/sun.png"})

	items, errs, err := parseFlashcardExcel(data)
	if err != nil {
		t.Fatalf("parseFlashcardExcel returned error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no row errors, got %#v", errs)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Term != "日" || items[0].Definition != "sun; day" {
		t.Fatalf("unexpected parsed item: %#v", items[0])
	}
	if items[0].ExampleSentence != "日曜日" || items[0].ImageURL != "https://example.com/sun.png" {
		t.Fatalf("expected optional columns to parse, got %#v", items[0])
	}
}

func TestParseFlashcardExcelRequiresTermAndDefinitionColumns(t *testing.T) {
	data := flashcardWorkbook(t, []string{"Prompt", "Clue"}, []string{"日", "sun; day"})

	_, _, err := parseFlashcardExcel(data)
	if err == nil {
		t.Fatal("expected missing header error")
	}
	if !strings.Contains(err.Error(), "'Term' and 'Definition'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseQuizRowsPreservesParagraphAndSubQuestions(t *testing.T) {
	subQuestions := `[{"id":1,"questionText":"19","questionType":"multiple_choice","options":[{"text":"A","position":0},{"text":"B","position":1}],"correctAnswer":"B"}]`
	rows := [][]string{
		{"Question", "Type", "Option A", "Option B", "Option C", "Option D", "Correct Answer", "Time (s)", "Audio URL", "Answer Explanation", "Sub Questions (JSON)", "Paragraph Text"},
		{"passage fallback", "PG", "", "", "", "", "", "30", "", "", subQuestions, "full passage"},
	}

	items, errs, questions := parseQuizRows(rows)
	if len(errs) != 0 {
		t.Fatalf("expected no row errors, got %#v", errs)
	}
	if len(items) != 1 || len(questions) != 1 {
		t.Fatalf("expected one parsed paragraph, got %d items and %d questions", len(items), len(questions))
	}
	question := questions[0]
	if question.QuestionType != "paragraph" || question.QuestionText != "" {
		t.Fatalf("unexpected paragraph question: %#v", question)
	}
	if question.CorrectAnswer != nil {
		t.Fatalf("paragraph parent must not have a correct answer, got %q", *question.CorrectAnswer)
	}
	if question.ParagraphText == nil || *question.ParagraphText != "full passage" {
		t.Fatalf("expected Paragraph Text column to be preserved, got %#v", question.ParagraphText)
	}
	if string(question.SubQuestions) != subQuestions {
		t.Fatalf("expected sub-questions JSON to be preserved, got %s", question.SubQuestions)
	}
}

func flashcardWorkbook(t *testing.T, header []string, row []string) []byte {
	t.Helper()

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for col, value := range header {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}
	for col, value := range row {
		cell, err := excelize.CoordinatesToCellName(col+1, 2)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
