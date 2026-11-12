package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *app {
	t.Helper()
	return &app{root: t.TempDir(), token: strings.Repeat("a", 40), workerToken: strings.Repeat("b", 40), jobs: map[string]job{}, queue: make(chan string, maxPending), slots: make(chan struct{}, maxPending), busy: make(chan struct{}, 1)}
}
func request(a *app, method, path string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, path, body)
	r.Header.Set("Authorization", "Bearer "+a.token)
	return r
}

func TestQueueDispatchDownloadAndRestart(t *testing.T) {
	a := fixture(t)
	srt := "1\n00:00:00,000 --> 00:00:01,000\nHello\n"
	worker := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+a.workerToken {
			t.Error("worker token missing")
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/healthz" {
			respond(w, 200, map[string]bool{"busy": false})
			return
		}
		data, _ := io.ReadAll(r.Body)
		if string(data) != "audio bytes" {
			t.Errorf("audio changed: %q", data)
		}
		if r.URL.Query().Get("language") != "vi" {
			t.Error("language lost")
		}
		w.Write([]byte(srt))
	}))
	defer worker.Close()
	a.client = worker.Client()
	w := httptest.NewRecorder()
	a.coordinator().ServeHTTP(w, request(a, "POST", "/v1/transcriptions?language=vi", bytes.NewBufferString("audio bytes")))
	if w.Code != 202 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var j job
	if err := json.Unmarshal(w.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	// Simulate restart before dispatch. Accepted work is recovered from disk.
	recovered := fixture(t)
	recovered.root = a.root
	recovered.client = a.client
	if err := recovered.recoverJobs(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recovered.dispatch(ctx, worker.URL)
	deadline := time.Now().Add(5 * time.Second)
	for {
		recovered.mu.Lock()
		state := recovered.jobs[j.ID].Status
		recovered.mu.Unlock()
		if state == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job stuck: %s", state)
		}
		time.Sleep(10 * time.Millisecond)
	}
	w = httptest.NewRecorder()
	recovered.coordinator().ServeHTTP(w, request(recovered, "GET", "/v1/transcriptions/"+j.ID+"/srt", nil))
	if w.Code != 200 || w.Body.String() != srt {
		t.Fatalf("download: %d %q", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.dir(j.ID), "audio")); !os.IsNotExist(err) {
		t.Fatal("audio retained after completion")
	}
}
func TestAuthenticationValidationAndCapacity(t *testing.T) {
	a := fixture(t)
	h := a.coordinator()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/transcriptions", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request(a, "POST", "/v1/transcriptions?language=../../bad", bytes.NewBufferString("data")))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request(a, "POST", "/v1/transcriptions", bytes.NewReader(nil)))
	if w.Code != 400 || len(a.slots) != 0 {
		t.Fatal("empty upload leaked queue slot")
	}
	for range maxPending {
		a.slots <- struct{}{}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request(a, "POST", "/v1/transcriptions", bytes.NewBufferString("data")))
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
}
func TestRecoveryRemovesIncompleteUpload(t *testing.T) {
	a := fixture(t)
	id := strings.Repeat("f", 32)
	if err := os.Mkdir(a.dir(id), 0700); err != nil {
		t.Fatal(err)
	}
	if err := a.recoverJobs(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.dir(id)); !os.IsNotExist(err) {
		t.Fatal("orphan upload not removed")
	}
}
func TestWorkerBusyAndAuthentication(t *testing.T) {
	a := fixture(t)
	a.busy <- struct{}{}
	w := httptest.NewRecorder()
	a.worker().ServeHTTP(w, request(a, "POST", "/internal/transcribe", bytes.NewBufferString("data")))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
