package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MaxQuizAudioBytes = 30 << 20

var ErrAudioSource = errors.New("audio source must be a private upload or an explicitly allowed HTTPS host")
var ErrAudioFormat = errors.New("unsupported audio file or file exceeds 30 MB")

// The database retains a recovery copy; MinIO holds a private backup with a
// deterministic key. Never expose either the origin or a public bucket URL.
type QuizAudio struct {
	db     *sql.DB
	hosts  map[string]bool
	client *http.Client
	backup AudioBackup
}

type AudioBackup interface {
	PutAudio(context.Context, string, []byte, string) error
}

func (a *QuizAudio) WithBackup(backup AudioBackup) *QuizAudio { a.backup = backup; return a }

func (a *QuizAudio) backupAudio(ctx context.Context, id string, data []byte, mime string) error {
	if a.backup == nil {
		return nil
	}
	key := "quiz-audio/" + id
	if err := a.backup.PutAudio(ctx, key, data, mime); err != nil {
		return err
	}
	_, err := a.db.ExecContext(ctx, `UPDATE protected_quiz_audio SET object_key=$2, backed_up_at=now() WHERE id=$1`, id, key)
	return err
}

func NewQuizAudio(db *sql.DB, allowedHosts string) *QuizAudio {
	hosts := map[string]bool{}
	for _, h := range strings.Split(allowedHosts, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			hosts[h] = true
		}
	}
	transport := &http.Transport{Proxy: nil, DialContext: publicAudioDial, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second}
	return &QuizAudio{db: db, hosts: hosts, client: &http.Client{Transport: transport, Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrAudioSource }}}
}
func publicAudioIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "2002::/16"} {
		_, n, _ := net.ParseCIDR(cidr)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
func publicAudioDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrAudioSource
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, ErrAudioSource
	}
	for _, ip := range ips {
		if !publicAudioIP(ip.IP) {
			return nil, ErrAudioSource
		}
	}
	// Dial the validated address directly to prevent DNS rebinding between check and connect.
	return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}
func (a *QuizAudio) allowed(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && a.hosts[strings.ToLower(u.Hostname())]
}
func audioMIME(data []byte) (string, error) {
	// MP3 without an ID3 tag starts with an MPEG frame. DetectContentType only
	// recognizes the ID3 variant; many imported listening files have no tag.
	if len(data) >= 4 && data[0] == 0xff && data[1]&0xe0 == 0xe0 &&
		(data[1]>>3)&3 != 1 && (data[1]>>1)&3 != 0 &&
		data[2]>>4 != 0 && data[2]>>4 != 15 && (data[2]>>2)&3 != 3 {
		return "audio/mpeg", nil
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "audio/mpeg", "audio/wave", "audio/x-wav", "audio/aiff", "audio/basic", "audio/midi", "application/ogg", "video/mp4", "video/webm":
		return mime, nil
	}
	if len(data) >= 4 && string(data[:4]) == "fLaC" {
		return "audio/flac", nil
	}
	return "", ErrAudioFormat
}
func (a *QuizAudio) Put(ctx context.Context, setID int64, r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxQuizAudioBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > MaxQuizAudioBytes {
		return "", ErrAudioFormat
	}
	mime, err := audioMIME(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(strconv.FormatInt(setID, 10)+":"), data...))
	id := hex.EncodeToString(sum[:])
	_, err = a.db.ExecContext(ctx, `INSERT INTO protected_quiz_audio(id,study_set_id,content_type,body) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING`, id, setID, mime, data)
	if err == nil {
		err = a.backupAudio(ctx, id, data, mime)
	}
	return "private-audio:" + id, err
}
func (a *QuizAudio) Open(ctx context.Context, setID int64, source string) (*bytes.Reader, string, error) {
	id := strings.TrimPrefix(source, "private-audio:")
	if !strings.HasPrefix(source, "private-audio:") {
		if !a.allowed(source) {
			return nil, "", ErrAudioSource
		}
		// Cache remote imports privately by source URL and set. Never redirect the learner to the origin.
		sum := sha256.Sum256([]byte(strconv.FormatInt(setID, 10) + ":" + source))
		id = hex.EncodeToString(sum[:])
	}
	var data []byte
	var mime string
	var objectKey sql.NullString
	err := a.db.QueryRowContext(ctx, `SELECT body,content_type,object_key FROM protected_quiz_audio WHERE id=$1 AND study_set_id=$2`, id, setID).Scan(&data, &mime, &objectKey)
	if err == nil {
		if !objectKey.Valid {
			if err := a.backupAudio(ctx, id, data, mime); err != nil {
				return nil, "", err
			}
		}
		return bytes.NewReader(data), mime, nil
	}
	if !errors.Is(err, sql.ErrNoRows) || strings.HasPrefix(source, "private-audio:") {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return nil, "", ErrAudioSource
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, "", ErrAudioSource
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.ContentLength > MaxQuizAudioBytes {
		return nil, "", ErrAudioFormat
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, MaxQuizAudioBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > MaxQuizAudioBytes {
		return nil, "", ErrAudioFormat
	}
	mime, err = audioMIME(data)
	if err != nil {
		return nil, "", err
	}
	_, err = a.db.ExecContext(ctx, `INSERT INTO protected_quiz_audio(id,study_set_id,content_type,body,source_url) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, id, setID, mime, data, source)
	if err != nil {
		return nil, "", err
	}
	if err := a.backupAudio(ctx, id, data, mime); err != nil {
		return nil, "", err
	}
	return bytes.NewReader(data), mime, nil
}
