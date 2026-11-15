// caption-tool inspects/downloads public subtitles without downloading media.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/youtubecaptions"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	video := flag.String("video", "", "YouTube video ID or youtube.com/watch URL")
	language := flag.String("language", "", "Original language, blank detects original")
	translation := flag.String("translation", "vi", "Optional translation language")
	provider := flag.String("provider", "auto", "auto, direct or yt-dlp")
	binary := flag.String("yt-dlp", os.Getenv("YOUTUBE_YTDLP_BIN"), "Path to yt-dlp executable")
	output := flag.String("output", "", "JSON output file, blank writes stdout")
	srt := flag.String("srt", "", "Optional original SRT output file")
	flag.Parse()
	id := *video
	if u, e := url.Parse(id); e == nil && u.Scheme == "https" {
		switch u.Hostname() {
		case "www.youtube.com", "youtube.com":
			if u.Path == "/watch" {
				id = u.Query().Get("v")
			}
		case "youtu.be":
			id = strings.TrimPrefix(u.Path, "/")
		}
	}
	if len(id) != 11 || strings.ContainsAny(id, "/?&= .") || !youtubecaptions.ValidLanguage(*language) || !youtubecaptions.ValidLanguage(*translation) {
		fmt.Fprintln(os.Stderr, "Invalid YouTube ID/URL or language")
		os.Exit(2)
	}
	if *provider != "auto" && *provider != "direct" && *provider != "yt-dlp" {
		fmt.Fprintln(os.Stderr, "Invalid provider")
		os.Exit(2)
	}
	client := youtubecaptions.New()
	client.Extractor = nil
	if *binary != "" && *provider != "direct" {
		client.Extractor = &youtubecaptions.YTDLP{Binary: *binary}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
	defer cancel()
	var result youtubecaptions.Result
	var err error
	if *provider == "yt-dlp" {
		if *binary == "" {
			fmt.Fprintln(os.Stderr, "--yt-dlp is required for yt-dlp provider")
			os.Exit(2)
		}
		result, err = client.Extractor.Fetch(ctx, id, *language, *translation)
	} else {
		result, err = client.Fetch(ctx, id, *language, *translation)
	}
	if err != nil {
		result.Message = "Không tải được phụ đề; kết quả có thể chứa danh sách ngôn ngữ. Kiểm tra yt-dlp đã cài và thử lại hoặc nhập SRT/VTT."
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	data = append(data, '\n')
	if *output != "" {
		if e := os.WriteFile(*output, data, 0600); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	} else {
		os.Stdout.Write(data)
	}
	if *srt != "" && result.Original != nil {
		var b strings.Builder
		for i, cue := range result.Original.Cues {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, timestamp(cue.Start), timestamp(cue.End), cue.Text)
		}
		if e := os.WriteFile(*srt, []byte(b.String()), 0600); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	}
	if err != nil {
		os.Exit(1)
	}
}
func timestamp(seconds float64) string {
	ms := int64(seconds*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
