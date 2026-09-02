package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// P4-GOLDEN-01 — Golden vectors exported from Rust quiz-core v0.1.0.
// If these tests fail, the Go and Rust engines have diverged.

type goldenCase struct {
	Label  string  `json:"label"`
	Seed   uint64  `json:"seed"`
	Input  []int64 `json:"input"`
	Output []int64 `json:"output"`
}

type goldenFile struct {
	Version string       `json:"version"`
	Cases   []goldenCase `json:"cases"`
}

// TestGoldenShuffleMatchesRust verifies that the Go shuffle produces
// identical output to Rust for all pinned golden vectors.
// The golden file is at packages/api-contracts/examples/quiz/golden/v1.4.0/shuffle-vectors.json
func TestGoldenShuffleMatchesRust(t *testing.T) {
	data, err := os.ReadFile("../../../../packages/api-contracts/examples/quiz/golden/v1.4.0/shuffle-vectors.json")
	if err != nil {
		t.Fatalf("failed to read golden vectors: %v", err)
	}

	var golden goldenFile
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("failed to parse golden vectors: %v", err)
	}

	for _, c := range golden.Cases {
		t.Run(c.Label, func(t *testing.T) {
			// Convert input to strings for shuffle
			input := make([]string, len(c.Input))
			for i, v := range c.Input {
				input[i] = intToStr(v)
			}
			expected := make([]string, len(c.Output))
			for i, v := range c.Output {
				expected[i] = intToStr(v)
			}

			got := shuffle(input, c.Seed)
			if !reflect.DeepEqual(got, expected) {
				t.Errorf("shuffle mismatch for seed=%d\n  got:  %v\n  want: %v", c.Seed, got, expected)
			}
		})
	}
}

// TestGoldenScoreAnswerNormalization verifies that the Go normalize()
// function matches Rust's score_answer() for pinned cases.
func TestGoldenScoreAnswerNormalization(t *testing.T) {
	type scoreCase struct {
		Expected string `json:"expected"`
		Submitted string `json:"submitted"`
		Correct  bool   `json:"correct"`
	}

	cases := []scoreCase{
		{Expected: "Tokyo", Submitted: "Tokyo", Correct: true},
		{Expected: "Tokyo", Submitted: "tokyo", Correct: true},
		{Expected: "Tokyo", Submitted: "  TOKYO  ", Correct: true},
		{Expected: "  hello  ", Submitted: "hello", Correct: true},
		{Expected: "Hai Based", Submitted: " hai   based ", Correct: true},
		{Expected: "Tokyo", Submitted: "Osaka", Correct: false},
		{Expected: "Tokyo", Submitted: "", Correct: false},
		{Expected: "xin chào", Submitted: "Xin chào", Correct: true},
		{Expected: "Ngôn ngữ", Submitted: "ngôn ngữ", Correct: true},
	}

	for _, c := range cases {
		got := normalize(c.Submitted) == normalize(c.Expected)
		if got != c.Correct {
			t.Errorf("normalize(%q) == normalize(%q): got %v, want %v",
				c.Submitted, c.Expected, got, c.Correct)
		}
	}
}

// TestGoldenSeedZeroMapsToOne verifies that seed=0 maps to seed=1
// (matching Rust's behavior: effective_seed = if seed == 0 { 1 } else { seed })
func TestGoldenSeedZeroMapsToOne(t *testing.T) {
	input := []int64{0, 1, 2, 3, 4, 5, 6, 7}
	inputStr := make([]string, len(input))
	for i, v := range input {
		inputStr[i] = intToStr(v)
	}
	got0 := shuffle(inputStr, 0)
	got1 := shuffle(inputStr, 1)
	if !reflect.DeepEqual(got0, got1) {
		t.Errorf("seed=0 should map to seed=1\n  seed=0: %v\n  seed=1: %v", got0, got1)
	}
}

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	const digits = "0123456789"
	b := make([]byte, 0, 20)
	for n > 0 {
		b = append(b, digits[n%10])
		n /= 10
	}
	if negative {
		b = append(b, '-')
	}
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}
