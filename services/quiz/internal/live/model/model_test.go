package model

import (
	"regexp"
	"testing"
)

func TestStateTransition(t *testing.T) {
	tests := []struct {
		state, command, want string
		wantErr              bool
	}{
		{StatusLobby, "start", StatusQuestionOpen, false},
		{StatusQuestionOpen, "close", StatusQuestionClosed, false},
		{StatusQuestionClosed, "next", StatusQuestionOpen, false},
		{StatusLobby, "next", "", true},
		{StatusEnded, "start", "", true},
	}
	for _, tc := range tests {
		got, err := StateTransition(tc.state, tc.command)
		if tc.wantErr != (err != nil) || got != tc.want {
			t.Fatalf("StateTransition(%q, %q) = %q, %v; want %q, err=%v", tc.state, tc.command, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestScoreAnswerBoundaries(t *testing.T) {
	if got := ScoreAnswer(false, 20_000, 20_000); got != 0 {
		t.Fatalf("incorrect answer score = %d; want 0", got)
	}
	if got := ScoreAnswer(true, 20_000, 20_000); got != 1500 {
		t.Fatalf("full-time bonus score = %d; want 1500", got)
	}
	if got := ScoreAnswer(true, 0, 20_000); got != 1000 {
		t.Fatalf("deadline score = %d; want 1000", got)
	}
	if got := ScoreAnswer(true, 40_000, 20_000); got != 1500 {
		t.Fatalf("clamped score = %d; want 1500", got)
	}
}

func TestGenerateEventIDReturnsUUID(t *testing.T) {
	id, err := GenerateEventID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatalf("event ID %q is not an RFC 4122 v4 UUID", id)
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	if got := NormalizeDisplayName("  Nguyen   An  "); got != "Nguyen An" {
		t.Fatalf("NormalizeDisplayName = %q", got)
	}
}
