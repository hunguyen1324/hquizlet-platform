// Package youtubecaptions reads captions exposed by public YouTube pages.
// It does not authenticate, bypass challenges, or download video/audio media.
package youtubecaptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Cue struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}
type Track struct {
	Language string `json:"language"`
	Label    string `json:"label"`
	Source   string `json:"source"`
	Cues     []Cue  `json:"cues"`
}
type TrackInfo struct {
	Language  string `json:"language"`
	Label     string `json:"label"`
	Automatic bool   `json:"automatic"`
}
type Result struct {
	Original        *Track      `json:"original"`
	Translation     *Track      `json:"translation"`
	AvailableTracks []TrackInfo `json:"availableTracks"`
	Message         string      `json:"message,omitempty"`
}

var ErrUnavailable = errors.New("public YouTube captions unavailable")
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var languagePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,34}$`)
var tags = regexp.MustCompile(`(?s)<[^>]*>`)

func ValidLanguage(s string) bool { return s == "" || languagePattern.MatchString(s) }
func CleanText(s string) string {
	return strings.TrimSpace(html.UnescapeString(tags.ReplaceAllString(s, "")))
}
func ValidateCues(cues []Cue) error {
	if len(cues) == 0 || len(cues) > 10000 {
		return errors.New("phụ đề cần từ 1 đến 10000 câu")
	}
	total := 0
	for i := range cues {
		c := &cues[i]
		c.Text = CleanText(c.Text)
		total += len(c.Text)
		if math.IsNaN(c.Start) || math.IsNaN(c.End) || math.IsInf(c.Start, 0) || math.IsInf(c.End, 0) || c.Start < 0 || c.End <= c.Start || c.End > 43200 || c.Text == "" || len([]rune(c.Text)) > 3000 || strings.ContainsRune(c.Text, 0) {
			return fmt.Errorf("câu %d có nội dung hoặc thời gian không hợp lệ", i+1)
		}
	}
	if total > 1024*1024 {
		return errors.New("nội dung phụ đề tối đa 1 MiB")
	}
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].Start < cues[j].Start })
	return nil
}

type captionTrack struct {
	BaseURL      string `json:"baseUrl"`
	Language     string `json:"languageCode"`
	Kind         string `json:"kind"`
	Translatable bool   `json:"isTranslatable"`
	Name         struct {
		SimpleText string `json:"simpleText"`
		Runs       []struct {
			Text string `json:"text"`
		} `json:"runs"`
	} `json:"name"`
}
type cacheEntry struct {
	result Result
	until  time.Time
}
type Client struct {
	HTTP  *http.Client
	mu    sync.Mutex
	cache map[string]cacheEntry
	slots chan struct{}
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, cache: map[string]cacheEntry{}, slots: make(chan struct{}, 4)}
}
func (c *Client) get(ctx context.Context, raw string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "HQuizlet/1.0 (public caption reader)")
	req.Header.Set("Accept-Language", "en")
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrUnavailable
	}
	return data, nil
}
func extractTracks(page []byte) ([]captionTrack, error) {
	// Decode the balanced player response rather than matching nested JSON with regex.
	text := string(page)
	for _, marker := range []string{"ytInitialPlayerResponse =", `ytInitialPlayerResponse=`} {
		offset := strings.Index(text, marker)
		if offset < 0 {
			continue
		}
		rest := strings.TrimSpace(text[offset+len(marker):])
		var data struct {
			Captions struct {
				Renderer struct {
					Tracks []captionTrack `json:"captionTracks"`
				} `json:"playerCaptionsTracklistRenderer"`
			} `json:"captions"`
		}
		if json.NewDecoder(strings.NewReader(rest)).Decode(&data) == nil && len(data.Captions.Renderer.Tracks) > 0 {
			return data.Captions.Renderer.Tracks, nil
		}
	}
	return nil, ErrUnavailable
}
func safeCaptionURL(raw, id, translation string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Hostname() != "www.youtube.com" && u.Hostname() != "youtube.com") || u.Path != "/api/timedtext" || u.Query().Get("v") != id {
		return "", ErrUnavailable
	}
	q := u.Query()
	q.Set("fmt", "json3")
	if translation != "" {
		q.Set("tlang", translation)
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String(), nil
}
func decodeJSON3(data []byte) ([]Cue, error) {
	var payload struct {
		Events []struct {
			Start    float64 `json:"tStartMs"`
			Duration float64 `json:"dDurationMs"`
			Segments []struct {
				Text string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return nil, ErrUnavailable
	}
	out := []Cue{}
	for _, event := range payload.Events {
		var text strings.Builder
		for _, segment := range event.Segments {
			text.WriteString(segment.Text)
		}
		value := CleanText(text.String())
		if value == "" || event.Duration <= 0 {
			continue
		}
		cue := Cue{event.Start / 1000, (event.Start + event.Duration) / 1000, value}
		if len(out) > 0 && out[len(out)-1].Text == value && cue.Start <= out[len(out)-1].End {
			if cue.End > out[len(out)-1].End {
				out[len(out)-1].End = cue.End
			}
			continue
		}
		out = append(out, cue)
	}
	if ValidateCues(out) != nil {
		return nil, ErrUnavailable
	}
	return out, nil
}
func (c *Client) Fetch(ctx context.Context, id, language, translation string) (Result, error) {
	if !idPattern.MatchString(id) || !ValidLanguage(language) || !ValidLanguage(translation) {
		return Result{}, ErrUnavailable
	}
	key := id + ":" + language + ":" + translation
	c.mu.Lock()
	cached, ok := c.cache[key]
	c.mu.Unlock()
	if ok && time.Now().Before(cached.until) {
		if cached.result.Original == nil {
			return cached.result, ErrUnavailable
		}
		return cached.result, nil
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := c.fetch(ctx, id, language, translation)
	c.mu.Lock()
	if len(c.cache) >= 128 {
		c.cache = map[string]cacheEntry{}
	}
	ttl := 10 * time.Minute
	if err != nil {
		ttl = time.Minute
	}
	c.cache[key] = cacheEntry{result, time.Now().Add(ttl)}
	c.mu.Unlock()
	return result, err
}
func (c *Client) fetch(ctx context.Context, id, language, translation string) (Result, error) {
	result := Result{AvailableTracks: []TrackInfo{}}
	page, err := c.get(ctx, "https://www.youtube.com/watch?v="+url.QueryEscape(id)+"&hl=en", 4*1024*1024)
	if err != nil {
		return result, err
	}
	tracks, err := extractTracks(page)
	if err != nil {
		return result, err
	}
	if len(tracks) > 100 {
		return result, ErrUnavailable
	}
	selected := 0
	for i, track := range tracks {
		label := track.Name.SimpleText
		if label == "" {
			for _, run := range track.Name.Runs {
				label += run.Text
			}
		}
		if label == "" {
			label = track.Language
		}
		result.AvailableTracks = append(result.AvailableTracks, TrackInfo{track.Language, label, track.Kind == "asr"})
		if language != "" && track.Language == language {
			selected = i
		}
	}
	if language != "" && tracks[selected].Language != language {
		return result, ErrUnavailable
	}
	load := func(index int, translate string) (*Track, error) {
		track := tracks[index]
		endpoint, e := safeCaptionURL(track.BaseURL, id, translate)
		if e != nil {
			return nil, e
		}
		data, e := c.get(ctx, endpoint, 3*1024*1024)
		if e != nil {
			return nil, e
		}
		cues, e := decodeJSON3(data)
		if e != nil {
			return nil, e
		}
		lang := track.Language
		label := result.AvailableTracks[index].Label
		if translate != "" {
			lang = translate
			label = translate + " (YouTube tự dịch)"
		}
		return &Track{lang, label, "youtube", cues}, nil
	}
	result.Original, err = load(selected, "")
	if err != nil {
		return result, err
	}
	if translation != "" && translation != result.Original.Language {
		found := -1
		for i, t := range tracks {
			if t.Language == translation {
				found = i
				break
			}
		}
		if found >= 0 {
			result.Translation, _ = load(found, "")
		} else if tracks[selected].Translatable {
			result.Translation, _ = load(selected, translation)
		}
		if result.Translation == nil {
			result.Message = "Không lấy được bản dịch từ YouTube. Có thể nhập file phụ đề dịch."
		}
	}
	return result, nil
}
