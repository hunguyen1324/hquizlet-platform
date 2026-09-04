//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/xuri/excelize/v2"
)

func main() {
	// Generate flashcard template
	if err := generateFlashcardTemplate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating flashcard template: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created flashcard_template.xlsx")

	// Generate quiz template
	if err := generateQuizTemplate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating quiz template: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created quiz_template.xlsx")
}

func generateFlashcardTemplate() error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	f.SetSheetName(sheet, "Flashcards")

	// Headers
	headers := []string{"Term", "Definition", "Example", "Hint", "Synonyms", "Image URL"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Flashcards", cell, h)
	}

	// Example rows
	examples := [][]string{
		{"hello", "xin chào", "Hello, how are you?", "", "", ""},
		{"goodbye", "tạm biệt", "Goodbye, see you later!", "", "", ""},
		{"thank you", "cảm ơn", "Thank you for your help.", "", "", ""},
	}

	for rowIdx, row := range examples {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue("Flashcards", cell, val)
		}
	}

	// Set column widths
	f.SetColWidth("Flashcards", "A", "F", 25)

	return f.SaveAs("flashcard_template.xlsx")
}

func generateQuizTemplate() error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	f.SetSheetName(sheet, "Quiz")

	// Headers
	headers := []string{"Question", "Type", "Option A", "Option B", "Option C", "Option D", "Correct Answer", "Time (s)", "Audio URL", "Answer Explanation"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Quiz", cell, h)
	}

	// Example rows
	examples := [][]string{
		{"What is the capital of Japan?", "MC", "Seoul", "Tokyo", "Bangkok", "Hanoi", "B", "30", "", "Tokyo is the capital city of Japan."},
		{"Water boils at 100°C at sea level.", "TF", "True", "False", "", "", "True", "20", "", "At standard pressure, water boils at 100°C."},
		{"Write the past tense of 'go'.", "WR", "", "", "", "", "went", "45", "", "The irregular past tense of 'go' is 'went'."},
		{"Read the paragraph and answer: Linh studies English every morning. What does Linh study?", "PG", "", "", "", "", "English", "60", "https://example.com/audio/linh-english.mp3", "The paragraph says Linh studies English every morning."},
		{"Sort these words into a correct sentence: / every day / I / English / study", "SO", "I", "study", "English", "every day", "I study English every day", "60", "", "The natural sentence order is subject, verb, object, time expression."},
	}

	for rowIdx, row := range examples {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue("Quiz", cell, val)
		}
	}

	// Set column widths
	f.SetColWidth("Quiz", "A", "J", 25)

	return f.SaveAs("quiz_template.xlsx")
}
