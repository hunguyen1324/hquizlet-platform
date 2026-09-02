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
		{"2+2=?", "MC", "3", "4", "5", "6", "B", "30", "", "2+2=4"},
		{"Is the sky blue?", "TF", "", "", "", "", "True", "15", "", "The sky appears blue due to Rayleigh scattering"},
		{"What is the capital of France?", "WR", "", "", "", "", "Paris", "60", "", "Paris is the capital and most populous city of France"},
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
