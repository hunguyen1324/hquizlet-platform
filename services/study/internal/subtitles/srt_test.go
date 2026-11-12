package subtitles

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	good := "\ufeff1\r\n00:00:00,000 --> 00:00:01,100\r\nXin chào\r\nこんにちは\r\n"
	normalized, err := Normalize(good)
	if err != nil || strings.Contains(normalized, "\r") || !strings.Contains(normalized, "こんにちは") {
		t.Fatalf("%q %v", normalized, err)
	}
	for _, raw := range []string{"bad", "1\n00:00:02,000 --> 00:00:01,000\ntext", "1\n00:70:00,000 --> 00:71:00,000\ntext", "1\n00:00:00,000 --> 00:00:01,000\n", strings.Repeat("日", 400000)} {
		if _, err = Normalize(raw); err == nil {
			t.Error("invalid SRT accepted")
		}
	}
	if value, err := Normalize(""); err != nil || value != "" {
		t.Fatal("cannot clear SRT")
	}
}
