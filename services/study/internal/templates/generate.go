package templates

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed *.xlsx
var templateFS embed.FS

// GetFlashcardTemplate returns the flashcard template file.
func GetFlashcardTemplate() (fs.File, error) {
	return templateFS.Open("flashcard_template.xlsx")
}

// GetQuizTemplate returns the quiz template file.
func GetQuizTemplate() (fs.File, error) {
	return templateFS.Open("quiz_template.xlsx")
}

// GetTemplate returns a template file by name.
func GetTemplate(name string) (fs.File, error) {
	switch name {
	case "flashcard_template.xlsx":
		return GetFlashcardTemplate()
	case "quiz_template.xlsx":
		return GetQuizTemplate()
	default:
		return nil, fmt.Errorf("template not found: %s", name)
	}
}
