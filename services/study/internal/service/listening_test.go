package service

import (
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"testing"
)

func TestListeningSubtitleRequiresReveal(t *testing.T) {
	item := model.SessionItem{Question: model.QuizQuestion{SRTText: "private listening transcript"}}
	hidden := quizPageItem(item, 0, false, false, false)
	if hidden.Question.SRTText != "" {
		t.Fatal("transcript leaked before reveal")
	}
	shown := quizPageItem(item, 0, true, true, false)
	if shown.Question.SRTText != item.Question.SRTText {
		t.Fatal("transcript not revealed")
	}
	if item.Question.SRTText == "" {
		t.Fatal("snapshot mutated")
	}
}
