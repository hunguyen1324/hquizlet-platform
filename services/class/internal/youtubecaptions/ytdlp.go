package youtubecaptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// YTDLP downloads subtitle files only. No shell, cookies or arbitrary user URLs.
type YTDLP struct {
	Binary string
	run    func(context.Context, string, []string) ([]byte, error)
}
type toolMetadata struct {
	ID        string                     `json:"id"`
	Language  string                     `json:"language"`
	Subtitles map[string]json.RawMessage `json:"subtitles"`
	Automatic map[string]json.RawMessage `json:"automatic_captions"`
}
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("extractor output limit")
	}
	return b.Buffer.Write(p)
}
func (t *YTDLP) execute(ctx context.Context, dir string, args []string) ([]byte, error) {
	if t.run != nil {
		return t.run(ctx, dir, args)
	}
	cmd := exec.CommandContext(ctx, t.Binary, args...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second
	out := &limitedBuffer{limit: 8 * 1024 * 1024}
	logs := &limitedBuffer{limit: 64 * 1024}
	cmd.Stdout = out
	cmd.Stderr = logs
	err := cmd.Run()
	return out.Bytes(), err // Never expose signed URLs or raw extractor logs to users.
}
func toolArgs() []string {
	return []string{"--ignore-config", "--no-playlist", "--skip-download", "--no-progress", "--no-warnings", "--socket-timeout", "6", "--retries", "0", "--extractor-retries", "0", "--ignore-no-formats-error"}
}
func (t *YTDLP) Fetch(ctx context.Context, id, language, translation string) (Result, error) {
	result := Result{AvailableTracks: []TrackInfo{}}
	if !idPattern.MatchString(id) || !ValidLanguage(language) || !ValidLanguage(translation) || t.Binary == "" {
		return result, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "hquizlet-captions-")
	if err != nil {
		return result, ErrUnavailable
	}
	defer os.RemoveAll(dir)
	args := append(toolArgs(), "--dump-single-json", "--", "https://www.youtube.com/watch?v="+id)
	raw, err := t.execute(ctx, dir, args)
	if err != nil {
		return result, ErrUnavailable
	}
	var metadata toolMetadata
	if json.Unmarshal(raw, &metadata) != nil || metadata.ID != id {
		return result, ErrUnavailable
	}
	keys := map[string]bool{}
	for lang := range metadata.Automatic {
		if ValidLanguage(lang) {
			keys[lang] = true
		}
	}
	for lang := range metadata.Subtitles {
		if ValidLanguage(lang) {
			keys[lang] = false
		}
	}
	ordered := make([]string, 0, len(keys))
	for lang := range keys {
		ordered = append(ordered, lang)
	}
	sort.Strings(ordered)
	if len(ordered) > 500 {
		return result, ErrUnavailable
	}
	for _, lang := range ordered {
		result.AvailableTracks = append(result.AvailableTracks, TrackInfo{Language: lang, Label: lang, Automatic: keys[lang]})
	}
	choose := func(want string) string {
		if _, ok := keys[want]; ok {
			return want
		}
		for _, key := range ordered {
			if strings.TrimSuffix(key, "-orig") == want {
				return key
			}
		}
		return ""
	}
	selected := choose(language)
	if language == "" {
		selected = choose(metadata.Language)
		if selected == "" {
			for _, key := range ordered {
				if strings.HasSuffix(key, "-orig") {
					selected = key
					break
				}
			}
		}
		if selected == "" && len(ordered) > 0 {
			selected = ordered[0]
		}
	}
	if selected == "" {
		return result, ErrUnavailable
	}
	translated := choose(translation)
	langs := "^" + regexp.QuoteMeta(selected) + "$"
	if translated != "" && translated != selected {
		langs += ",^" + regexp.QuoteMeta(translated) + "$"
	}
	infoPath := filepath.Join(dir, "metadata.json")
	if os.WriteFile(infoPath, raw, 0600) != nil {
		return result, ErrUnavailable
	}
	args = append(toolArgs(), "--load-info-json", infoPath, "--write-subs", "--write-auto-subs", "--sub-langs", langs, "--sub-format", "json3", "--output", filepath.Join(dir, "caption.%(ext)s"))
	_, _ = t.execute(ctx, dir, args)
	// A failed translation must not discard an original subtitle already downloaded.
	load := func(lang string) *Track {
		if lang == "" {
			return nil
		}
		f, e := os.Open(filepath.Join(dir, "caption."+lang+".json3"))
		if e != nil {
			return nil
		}
		defer f.Close()
		raw, e := io.ReadAll(io.LimitReader(f, 3*1024*1024+1))
		if e != nil || len(raw) > 3*1024*1024 {
			return nil
		}
		cues, e := decodeJSON3(raw)
		if e != nil {
			return nil
		}
		return &Track{Language: strings.TrimSuffix(lang, "-orig"), Label: lang, Source: "youtube", Cues: cues}
	}
	result.Original = load(selected)
	if result.Original == nil {
		return result, ErrUnavailable
	}
	if translated != selected {
		result.Translation = load(translated)
	}
	if translation != "" && translation != result.Original.Language && result.Translation == nil {
		result.Message = "Đã lấy phụ đề bằng yt-dlp; chưa lấy được bản dịch. Có thể nhập SRT/VTT bản dịch."
	} else {
		result.Message = "Đã lấy phụ đề YouTube bằng yt-dlp."
	}
	return result, nil
}
