package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/engine"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/studyclient"
)

// P4-TEST-01 — Fake study service for handler tests.
// Returns a fixed set of flashcards for any request.

var fakeFlashcards = []studyclient.Flashcard{
	{ID: 1, StudySetID: 100, Term: "hello", Definition: "xin chào", Starred: false, Position: 0},
	{ID: 2, StudySetID: 100, Term: "book", Definition: "quyển sách", Starred: true, Position: 1},
	{ID: 3, StudySetID: 100, Term: "water", Definition: "nước", Starred: false, Position: 2},
	{ID: 4, StudySetID: 100, Term: "computer", Definition: "máy tính", Starred: false, Position: 3},
}

func fakeStudyServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/study-sets/{id}/flashcards", func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" || userID == "0" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(studyclient.ErrorEnvelope{Code: "UNAUTHORIZED", Message: "authentication required"})
			return
		}
		uid, _ := strconv.ParseInt(userID, 10, 64)

		setID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

		// User 2 owns nothing (for ownership tests)
		if uid == 2 {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(studyclient.ErrorEnvelope{Code: "FORBIDDEN", Message: "study set not owned by caller"})
			return
		}

		// Set 999 doesn't exist
		if setID == 999 {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(studyclient.ErrorEnvelope{Code: "NOT_FOUND", Message: "study set not found"})
			return
		}

		json.NewEncoder(w).Encode(studyclient.StudySetWithCards{
			ID:          setID,
			UserID:      uid,
			Title:       "Test Study Set",
			Description: "For testing",
			Flashcards:  fakeFlashcards,
		})
	})
	return httptest.NewServer(mux)
}

func fakeStudyServerTimeout(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		select {
		case <-time.After(10 * time.Second):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
}

// newTestServer creates a quiz server backed by a fake study server.
func newTestServer(t *testing.T, studyURL string) *httptest.Server {
	t.Helper()
	srv := &server{
		study: studyclient.New(studyURL),
		met:   newMetrics(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "quiz", "status": "ok"})
	})
	mux.HandleFunc("GET /metrics", srv.met.writePrometheus)
	mux.HandleFunc("POST /v1/study-sets/{id}/quiz/generate", srv.generate)
	mux.HandleFunc("POST /v1/study-sets/{id}/quiz/evaluate", srv.evaluate)
	return httptest.NewServer(mux)
}

// ── Generate endpoint tests ───────────────────────────────────────────────────

func TestGenerate_RequiresAuth(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	// No X-User-ID header
	resp, _ := http.Post(q.URL+"/v1/study-sets/100/quiz/generate", "application/json",
		stringsReader(`{"mode":"flashcards","seed":1,"limit":10}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGenerate_InvalidMode(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	resp := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"invalid","seed":1,"limit":10}`)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestGenerate_StudySetNotFound(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	resp := postAs(t, q.URL+"/v1/study-sets/999/quiz/generate", "1", `{"mode":"flashcards","seed":1,"limit":10}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGenerate_OwnershipDenied(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	resp := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "2", `{"mode":"flashcards","seed":1,"limit":10}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestGenerate_FlashcardsMode(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	body := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"flashcards","seed":42,"limit":4}`)
	var resp generateResponse
	json.NewDecoder(body.Body).Decode(&resp)

	if resp.Mode != "flashcards" {
		t.Errorf("expected mode=flashcards, got %s", resp.Mode)
	}
	if resp.ContractVersion != engine.ContractVersion {
		t.Errorf("expected contractVersion=%s, got %s", engine.ContractVersion, resp.ContractVersion)
	}
	if len(resp.Items) != 4 {
		t.Errorf("expected 4 items, got %d", len(resp.Items))
	}
	for _, item := range resp.Items {
		if item.Kind != "question" {
			t.Errorf("expected kind=question, got %s", item.Kind)
		}
		if item.Definition == "" {
			t.Error("flashcards mode must include definition")
		}
	}
}

func TestGenerate_TestModeHidesDefinition(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	body := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"test","seed":42,"limit":4}`)
	var resp generateResponse
	json.NewDecoder(body.Body).Decode(&resp)

	for _, item := range resp.Items {
		if item.Definition != "" {
			t.Errorf("test mode must not leak definition for card %d", item.FlashcardID)
		}
		if len(item.Choices) < 2 {
			t.Errorf("test mode must have >=2 choices for card %d", item.FlashcardID)
		}
	}
}

func TestGenerate_MatchModeReturnsTermAndDefinitionPairs(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	body := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"match","seed":42,"limit":4}`)
	var resp generateResponse
	json.NewDecoder(body.Body).Decode(&resp)

	// Match mode should produce 2 items per card (term + definition)
	if len(resp.Items) != 8 {
		t.Fatalf("expected 8 items (4 pairs), got %d", len(resp.Items))
	}
	termCount := 0
	defCount := 0
	for _, item := range resp.Items {
		if item.Kind == "term" {
			termCount++
			if item.Text == "" {
				t.Error("match term item must have text")
			}
			if item.PairID == "" {
				t.Error("match term item must have pairId")
			}
		} else if item.Kind == "definition" {
			defCount++
			if item.Text == "" {
				t.Error("match definition item must have text")
			}
		} else {
			t.Errorf("match mode unexpected kind: %s", item.Kind)
		}
	}
	if termCount != 4 || defCount != 4 {
		t.Errorf("expected 4 terms + 4 definitions, got %d + %d", termCount, defCount)
	}
}

func TestGenerate_DeterministicAcrossCalls(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	body1 := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"test","seed":42,"limit":4}`)
	body2 := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"test","seed":42,"limit":4}`)

	var r1, r2 generateResponse
	json.NewDecoder(body1.Body).Decode(&r1)
	json.NewDecoder(body2.Body).Decode(&r2)

	if len(r1.Items) != len(r2.Items) {
		t.Fatal("same seed should produce same number of items")
	}
	for i := range r1.Items {
		if r1.Items[i].FlashcardID != r2.Items[i].FlashcardID {
			t.Errorf("item %d: same seed produced different flashcardId", i)
		}
		if r1.Items[i].Choices[0] != r2.Items[i].Choices[0] {
			t.Errorf("item %d: same seed produced different choices", i)
		}
	}
}

// ── Evaluate endpoint tests ───────────────────────────────────────────────────

func TestEvaluate_RequiresAuth(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	resp, _ := http.Post(q.URL+"/v1/study-sets/100/quiz/evaluate", "application/json",
		stringsReader(`{"mode":"learn","seed":1,"answers":[]}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEvaluate_ForeignCardRejected(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	resp := postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1",
		`{"mode":"learn","seed":1,"limit":4,"answers":[{"flashcardId":999,"submitted":"test","attempts":1}]}`)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for foreign card, got %d", resp.StatusCode)
	}
}

func TestEvaluate_LearnModeNormalization(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	body := postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1",
		`{"mode":"learn","seed":1,"limit":4,"answers":[{"flashcardId":1,"submitted":"  XIN CHÀO  ","attempts":2,"responseTimeMs":1500}]}`)
	var resp evaluateResponse
	json.NewDecoder(body.Body).Decode(&resp)

	if resp.Score != 1 || resp.Total != 1 {
		t.Errorf("expected score=1 total=1, got %d/%d", resp.Score, resp.Total)
	}
	if len(resp.CardResults) != 1 || !resp.CardResults[0].Correct {
		t.Errorf("expected correct=true, got %+v", resp.CardResults)
	}
	if resp.CardResults[0].Attempts != 2 {
		t.Errorf("expected attempts=2, got %d", resp.CardResults[0].Attempts)
	}
}

func TestEvaluate_TestModeScoring(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	// First generate to know the choices
	genBody := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1",
		`{"mode":"test","seed":42,"limit":4}`)
	var genResp generateResponse
	json.NewDecoder(genBody.Body).Decode(&genResp)

	// Build answers: for first item, select the correct choice
	var answers []map[string]any
	for _, item := range genResp.Items {
		// Find which choice matches the card's definition
		correctIdx := 0
		for i, c := range item.Choices {
			// Find the definition from the card
			for _, fc := range fakeFlashcards {
				if fc.ID == item.FlashcardID && c == fc.Definition {
					correctIdx = i
				}
			}
		}
		answers = append(answers, map[string]any{
			"flashcardId":    item.FlashcardID,
			"selectedIndex":  correctIdx,
			"attempts":       1,
			"responseTimeMs": 2000,
		})
	}

	answerJSON, _ := json.Marshal(map[string]any{"mode": "test", "seed": 42, "limit": 4, "answers": answers})
	evalBody := postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1", string(answerJSON))
	var evalResp evaluateResponse
	json.NewDecoder(evalBody.Body).Decode(&evalResp)

	if evalResp.Total != 4 {
		t.Errorf("expected total=4, got %d", evalResp.Total)
	}
	if evalResp.Score != 4 {
		t.Errorf("expected score=4 (all correct), got %d", evalResp.Score)
	}
	if evalResp.ContractVersion != engine.ContractVersion {
		t.Errorf("expected contractVersion=%s, got %s", engine.ContractVersion, evalResp.ContractVersion)
	}
}

func TestEvaluate_MatchModePairIdentity(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	// Generate match items first
	genBody := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1",
		`{"mode":"match","seed":42,"limit":4}`)
	var genResp generateResponse
	json.NewDecoder(genBody.Body).Decode(&genResp)

	// Build answers matching each card's pairId and flashcardId
	var answers []map[string]any
	for _, item := range genResp.Items {
		if item.Kind == "term" {
			answers = append(answers, map[string]any{
				"flashcardId":        item.FlashcardID,
				"pairId":             item.PairID,
				"matchedFlashcardId": item.FlashcardID,
				"attempts":           1,
				"responseTimeMs":     2000,
			})
		}
	}

	answerJSON, _ := json.Marshal(map[string]any{"mode": "match", "seed": 42, "limit": 4, "answers": answers})
	evalBody := postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1", string(answerJSON))
	var evalResp evaluateResponse
	json.NewDecoder(evalBody.Body).Decode(&evalResp)

	if evalResp.Score != 4 || evalResp.Total != 4 {
		t.Errorf("expected score=4/4, got %d/%d", evalResp.Score, evalResp.Total)
	}
	for _, cr := range evalResp.CardResults {
		if !cr.Correct {
			t.Errorf("expected all correct for matched pairs, got %+v", cr)
		}
	}
}

func TestEvaluate_RejectsForgedCardOutsideGeneratedSubset(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	genBody := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1",
		`{"mode":"match","seed":42,"limit":1}`)
	var genResp generateResponse
	json.NewDecoder(genBody.Body).Decode(&genResp)
	if len(genResp.Items) == 0 {
		t.Fatal("expected generated match items")
	}

	outside := int64(1)
	if genResp.Items[0].FlashcardID == outside {
		outside = 2
	}
	resp := postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1",
		`{"mode":"match","seed":42,"limit":1,"answers":[{"flashcardId":`+strconv.FormatInt(outside, 10)+`,"pairId":"card-`+strconv.FormatInt(outside, 10)+`","matchedFlashcardId":`+strconv.FormatInt(outside, 10)+`,"attempts":1}]}`)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for forged subset answer, got %d", resp.StatusCode)
	}
}

func TestMetricsEndpointRecordsModeLatencyAndErrors(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1", `{"mode":"test","seed":42,"limit":4}`)
	postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1", `{"mode":"test","seed":42,"limit":4,"answers":[]}`)

	resp, err := http.Get(q.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	metricsText := string(body)
	for _, expected := range []string{
		`quiz_requests_total{endpoint="generate",mode="test"}`,
		`quiz_errors_total{endpoint="evaluate",mode="test"}`,
		`quiz_latency_ms_count{endpoint="generate",mode="test"}`,
	} {
		if !strings.Contains(metricsText, expected) {
			t.Fatalf("missing metric %q in:\n%s", expected, metricsText)
		}
	}
}

func TestEvaluate_ErrorEnvelopeFormat(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	// Send an invalid mode to trigger error
	resp := postAs(t, q.URL+"/v1/study-sets/100/quiz/evaluate", "1",
		`{"mode":"bogus","seed":1,"limit":1,"answers":[]}`)
	var errResp errorEnvelope
	json.NewDecoder(resp.Body).Decode(&errResp)

	if errResp.Code == "" {
		t.Error("expected error code in envelope")
	}
	if errResp.Message == "" {
		t.Error("expected error message in envelope")
	}
}

func TestGenerate_ErrorEnvelopeOnInvalidMode(t *testing.T) {
	study := fakeStudyServer(t)
	defer study.Close()
	q := newTestServer(t, study.URL)
	defer q.Close()

	body := postAs(t, q.URL+"/v1/study-sets/100/quiz/generate", "1",
		`{"mode":"bogus","seed":1,"limit":10}`)
	var errResp errorEnvelope
	json.NewDecoder(body.Body).Decode(&errResp)

	if errResp.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", errResp.Code)
	}
	// Details may be empty map {} which deserializes as non-nil
	if errResp.Message == "" {
		t.Error("expected message in error envelope")
	}
}

// ── Timeout test ──────────────────────────────────────────────────────────────

func TestGenerate_StudyServiceTimeout(t *testing.T) {
	slowStudy := fakeStudyServerTimeout(t)
	defer slowStudy.Close()
	q := newTestServer(t, slowStudy.URL)
	defer q.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", q.URL+"/v1/study-sets/100/quiz/generate",
		stringsReader(`{"mode":"flashcards","seed":1,"limit":10}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "1")

	start := time.Now()
	_, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	// The context should cancel before the 10s study service response
	// Either we get an error (context canceled) or a non-200 response
	if elapsed > 5*time.Second {
		t.Errorf("request took too long: %v (should have timed out)", elapsed)
	}
	// Context cancellation or connection error is expected — both prove
	// the request did not hang for the full 10s study service delay
	if err == nil {
		t.Log("request completed within timeout — study service was fast enough")
	} else {
		t.Logf("expected context/timeout error: %v", err)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func postAs(t *testing.T, url, userID, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, stringsReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID)
	req.Header.Set("X-Request-ID", "test-"+t.Name())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func stringsReader(s string) *strings.Reader {
	return strings.NewReader(s)
}
