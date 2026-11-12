// STT provides a durable App-side queue and single-job HTTPS Whisper workers.
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxAudio = 64 << 20
const maxResult = 4 << 20
const maxPending = 16

var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type job struct {
	ID        string    `json:"id"`
	Language  string    `json:"language"`
	Status    string    `json:"status"`
	Worker    string    `json:"worker,omitempty"`
	Error     string    `json:"error,omitempty"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type app struct {
	root, token, workerToken string
	mu                       sync.Mutex
	jobs                     map[string]job
	queue                    chan string
	slots                    chan struct{}
	client                   *http.Client
	busy                     chan struct{}
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func respond(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, code int, message string) {
	respond(w, code, map[string]string{"error": message})
}
func language(r *http.Request) (string, bool) {
	v := r.URL.Query().Get("language")
	if v == "" {
		v = "auto"
	}
	return v, v == "auto" || v == "vi" || v == "ja" || v == "en"
}
func (a *app) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.token)) != 1 {
			fail(w, 401, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *app) dir(id string) string { return filepath.Join(a.root, id) }

// Caller holds mu; atomic rename leaves either the old or the new job metadata.
func (a *app) save(j job) error {
	j.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	path := filepath.Join(a.dir(j.ID), "job.json")
	if err = os.WriteFile(path+".tmp", data, 0600); err != nil {
		return err
	}
	if err = os.Rename(path+".tmp", path); err != nil {
		return err
	}
	a.jobs[j.ID] = j
	return nil
}
func (a *app) recoverJobs() error {
	entries, err := os.ReadDir(a.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validID.MatchString(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.root, entry.Name(), "job.json"))
		if os.IsNotExist(err) { // Incomplete upload interrupted before acceptance.
			if err = os.RemoveAll(filepath.Join(a.root, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("job metadata unreadable: %s", entry.Name())
		}
		var j job
		if json.Unmarshal(data, &j) != nil || j.ID != entry.Name() {
			return errors.New("invalid job metadata")
		}
		a.jobs[j.ID] = j
		if j.Status == "queued" || j.Status == "running" {
			if len(a.slots) == cap(a.slots) {
				return errors.New("pending queue exceeds capacity")
			}
			a.slots <- struct{}{}
			j.Status = "queued"
			j.Worker = ""
			if err = a.save(j); err != nil {
				return err
			}
			a.queue <- j.ID
		}
	}
	return nil
}
func upload(w http.ResponseWriter, r *http.Request, path string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAudio)
	defer r.Body.Close()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(w, 500, "storage unavailable")
		return false
	}
	n, copyErr := io.Copy(f, r.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n == 0 {
		_ = os.Remove(path)
		var oversized *http.MaxBytesError
		if errors.As(copyErr, &oversized) {
			fail(w, 413, "maximum audio size is 64 MiB")
		} else {
			fail(w, 400, "empty or incomplete audio upload")
		}
		return false
	}
	return true
}
func (a *app) submit(w http.ResponseWriter, r *http.Request) {
	lang, ok := language(r)
	if !ok {
		fail(w, 400, "language must be auto, vi, ja or en")
		return
	}
	select {
	case a.slots <- struct{}{}:
	default:
		fail(w, 429, "queue full; retry later")
		return
	}
	accepted := false
	defer func() {
		if !accepted {
			<-a.slots
		}
	}()
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		fail(w, 500, "ID generation failed")
		return
	}
	id := hex.EncodeToString(bytes)
	if err := os.Mkdir(a.dir(id), 0700); err != nil {
		fail(w, 500, "storage unavailable")
		return
	}
	defer func() {
		if !accepted {
			_ = os.RemoveAll(a.dir(id))
		}
	}()
	if !upload(w, r, filepath.Join(a.dir(id), "audio")) {
		return
	}
	j := job{ID: id, Language: lang, Status: "queued", CreatedAt: time.Now().UTC()}
	a.mu.Lock()
	err := a.save(j)
	a.mu.Unlock()
	if err != nil {
		fail(w, 500, "cannot save job")
		return
	}
	accepted = true
	a.queue <- id
	w.Header().Set("Location", "/v1/transcriptions/"+id)
	respond(w, 202, j)
}
func (a *app) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID.MatchString(id) {
		fail(w, 404, "job not found")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	j, ok := a.jobs[id]
	if !ok {
		fail(w, 404, "job not found")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/srt") {
		if j.Status != "completed" {
			fail(w, 409, "subtitle not ready")
			return
		}
		f, err := os.Open(filepath.Join(a.dir(id), "result.srt"))
		if err != nil {
			fail(w, 500, "subtitle unavailable")
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/x-subrip; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.srt"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.Copy(w, f)
		return
	}
	respond(w, 200, j)
}
func (a *app) coordinator() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/transcriptions", a.submit)
	mux.HandleFunc("GET /v1/transcriptions/{id}", a.get)
	mux.HandleFunc("GET /v1/transcriptions/{id}/srt", a.get)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "pending": len(a.slots), "capacity": maxPending})
	})
	return a.auth(mux)
}
func (a *app) dispatch(ctx context.Context, worker string) {
	for {
		// A down/busy worker must not consume jobs from the shared queue.
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		req, _ := http.NewRequestWithContext(probeCtx, "GET", worker+"/healthz", nil)
		req.Header.Set("Authorization", "Bearer "+a.workerToken)
		res, err := a.client.Do(req)
		ready := false
		if err == nil {
			var state struct {
				Busy bool `json:"busy"`
			}
			ready = res.StatusCode == 200 && json.NewDecoder(res.Body).Decode(&state) == nil && !state.Busy
			res.Body.Close()
		}
		cancel()
		if !ready {
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}
		var id string
		select {
		case <-ctx.Done():
			return
		case id = <-a.queue:
		}
		a.mu.Lock()
		j := a.jobs[id]
		j.Status = "running"
		j.Worker = worker
		j.Attempts++
		err = a.save(j)
		a.mu.Unlock()
		if err != nil {
			log.Fatal("cannot persist running job")
		}
		retry, message := a.transfer(ctx, worker, j)
		a.mu.Lock()
		j = a.jobs[id]
		if message == "" {
			j.Status = "completed"
			j.Error = ""
		} else if retry && j.Attempts < 6 {
			j.Status = "queued"
			j.Error = ""
		} else {
			j.Status = "failed"
			j.Error = message
		}
		if err = a.save(j); err != nil {
			log.Fatal("cannot persist finished job")
		}
		a.mu.Unlock()
		if j.Status == "queued" {
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			a.queue <- id
		} else {
			_ = os.Remove(filepath.Join(a.dir(id), "audio"))
			<-a.slots
		}
	}
}
func (a *app) transfer(ctx context.Context, worker string, j job) (bool, string) {
	f, err := os.Open(filepath.Join(a.dir(j.ID), "audio"))
	if err != nil {
		return false, "audio unavailable"
	}
	defer f.Close()
	runCtx, cancel := context.WithTimeout(ctx, 46*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(runCtx, "POST", worker+"/internal/transcribe?language="+url.QueryEscape(j.Language), f)
	stat, _ := f.Stat()
	req.ContentLength = stat.Size()
	req.Header.Set("Authorization", "Bearer "+a.workerToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := a.client.Do(req)
	if err != nil {
		return true, "worker connection failed"
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return res.StatusCode == 503, "worker could not transcribe this audio (HTTP " + strconv.Itoa(res.StatusCode) + ")"
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResult+1))
	if err != nil || len(data) == 0 || len(data) > maxResult {
		return true, "invalid worker result"
	}
	path := filepath.Join(a.dir(j.ID), "result.srt")
	if os.WriteFile(path+".tmp", data, 0600) != nil || os.Rename(path+".tmp", path) != nil {
		return false, "cannot store subtitle"
	}
	return false, ""
}
func (a *app) cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		a.mu.Lock()
		for id, j := range a.jobs {
			if (j.Status == "completed" || j.Status == "failed") && time.Since(j.UpdatedAt) > 24*time.Hour {
				if os.RemoveAll(a.dir(id)) == nil {
					delete(a.jobs, id)
				}
			}
		}
		a.mu.Unlock()
	}
}

type boundedLog struct{ data []byte }

func (b *boundedLog) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 65536 - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (a *app) worker() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "busy": len(a.busy) > 0})
	})
	mux.HandleFunc("POST /internal/transcribe", func(w http.ResponseWriter, r *http.Request) {
		lang, ok := language(r)
		if !ok {
			fail(w, 400, "invalid language")
			return
		}
		select {
		case a.busy <- struct{}{}:
			defer func() { <-a.busy }()
		default:
			fail(w, 503, "worker busy")
			return
		}
		dir, err := os.MkdirTemp(a.root, "run-")
		if err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		defer os.RemoveAll(dir)
		audio := filepath.Join(dir, "audio")
		if !upload(w, r, audio) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Minute)
		defer cancel()
		formats := "wav,mp3,ogg,flac,mov,matroska,webm,aac"
		probe := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", formats, "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", audio)
		configureProcess(probe)
		data, err := probe.Output()
		seconds, parseErr := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil || parseErr != nil || !(seconds > 0 && seconds <= 1200) {
			fail(w, 422, "audio must have a valid duration of at most 20 minutes")
			return
		}
		// Only local audio demuxers are allowed: playlists cannot fetch network URLs.
		wav := filepath.Join(dir, "input.wav")
		convert := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", formats, "-threads", "1", "-i", audio, "-vn", "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav)
		configureProcess(convert)
		if err = convert.Run(); err != nil {
			fail(w, 422, "audio conversion failed")
			return
		}
		// Keep every temporary file inside dir, including on timeout/process kill.
		// flock shares the same lock as the manual transcribe command.
		cmd := exec.CommandContext(ctx, "flock", "-n", "-E", "75", "/opt/whisper/transcribe.lock", "/opt/whisper/whisper.cpp/build/bin/whisper-cli", "-m", "/opt/whisper/models/ggml-base-q5_1.bin", "-f", wav, "-l", lang, "-t", "1", "-otxt", "-osrt", "-of", filepath.Join(dir, "result"))
		configureProcess(cmd)
		output := &boundedLog{}
		cmd.Stdout = output
		cmd.Stderr = output
		if err = cmd.Run(); err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) && exitError.ExitCode() == 75 {
				fail(w, 503, "worker busy")
				return
			}
			if ctx.Err() != nil {
				fail(w, 504, "transcription timeout")
			} else {
				fail(w, 422, "transcription failed or manual CLI is busy")
			}
			return
		}
		result, err := os.ReadFile(filepath.Join(dir, "result.srt"))
		if err != nil || len(result) == 0 || len(result) > maxResult {
			fail(w, 422, "no subtitles produced")
			return
		}
		w.Header().Set("Content-Type", "application/x-subrip; charset=utf-8")
		w.Write(result)
	})
	return a.auth(mux)
}
func createCert(ip, dir string) error {
	address := net.ParseIP(ip)
	if address == nil {
		return errors.New("invalid certificate IP")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "HQuizlet STT worker " + ip}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(3, 0, 0), IPAddresses: []net.IP{address}, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(dir, "key.pem")); err == nil {
		return errors.New("certificate already exists")
	}
	if err = os.WriteFile(filepath.Join(dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644)
}
func main() {
	certIP := flag.String("cert-ip", "", "create worker certificate for this IP and exit")
	certDir := flag.String("cert-dir", "/etc/hquizlet-stt", "certificate directory")
	flag.Parse()
	if *certIP != "" {
		if err := createCert(*certIP, *certDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	token := os.Getenv("STT_TOKEN")
	if len(token) < 32 {
		log.Fatal("STT_TOKEN must contain at least 32 characters")
	}
	root := env("STT_DATA_DIR", "/var/lib/hquizlet-stt")
	if err := os.MkdirAll(root, 0700); err != nil {
		log.Fatal(err)
	}
	a := &app{root: root, token: token, workerToken: os.Getenv("STT_WORKER_TOKEN"), jobs: map[string]job{}, queue: make(chan string, maxPending), slots: make(chan struct{}, maxPending), busy: make(chan struct{}, 1)}
	role := env("STT_ROLE", "worker")
	var handler http.Handler
	listen := env("STT_LISTEN", "[::]:8090")
	if role == "coordinator" {
		if len(a.workerToken) < 32 {
			log.Fatal("STT_WORKER_TOKEN required")
		}
		ca, err := os.ReadFile(os.Getenv("STT_WORKER_CA"))
		if err != nil {
			log.Fatal("worker CA unavailable")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			log.Fatal("invalid worker CA")
		}
		a.client = &http.Client{Timeout: 46 * time.Minute, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, ResponseHeaderTimeout: 46 * time.Minute}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if err = a.recoverJobs(); err != nil {
			log.Fatal(err)
		}
		workers := strings.Split(os.Getenv("STT_WORKERS"), ",")
		for _, worker := range workers {
			u, err := url.Parse(worker)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				log.Fatal("HTTPS worker URL required")
			}
			go a.dispatch(context.Background(), strings.TrimRight(worker, "/"))
		}
		go a.cleanup(context.Background())
		handler = a.coordinator()
		listen = env("STT_LISTEN", "127.0.0.1:8091")
	} else if role == "worker" {
		handler = a.worker()
	} else {
		log.Fatal("invalid role")
	}
	server := &http.Server{Addr: listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 47 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	if role == "coordinator" && os.Getenv("STT_BRIDGE_LISTEN") != "" {
		bridge := os.Getenv("STT_BRIDGE_LISTEN")
		host, _, err := net.SplitHostPort(bridge)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !ip.IsPrivate() {
			log.Fatal("STT_BRIDGE_LISTEN must bind a private IP")
		}
		go func() {
			internal := &http.Server{Addr: bridge, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 47 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
			log.Printf("STT Docker bridge listening on %s", bridge)
			log.Fatal(internal.ListenAndServe())
		}()
	}
	log.Printf("STT %s listening on %s", role, listen)
	if role == "worker" {
		log.Fatal(server.ListenAndServeTLS(env("STT_TLS_CERT", "/etc/hquizlet-stt/cert.pem"), env("STT_TLS_KEY", "/etc/hquizlet-stt/key.pem")))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}
