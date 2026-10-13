package http

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type hlsProbeStream struct {
	CodecType   string            `json:"codec_type"`
	CodecName   string            `json:"codec_name"`
	Profile     string            `json:"profile"`
	PixelFormat string            `json:"pix_fmt"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Level       int               `json:"level"`
	Channels    int               `json:"channels"`
	Tags        map[string]string `json:"tags"`
	SideData    []struct {
		Rotation float64 `json:"rotation"`
	} `json:"side_data_list"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

type hlsPlan struct{ copyVideo, copyAudio bool }

func chooseHLSPlan(streams []hlsProbeStream) hlsPlan {
	var plan hlsPlan
	videoSeen, audioSeen := false, false
	for _, s := range streams {
		if s.CodecType == "video" && !videoSeen {
			videoSeen = true
			profile := s.Profile == "Baseline" || s.Profile == "Constrained Baseline" || s.Profile == "Main" || s.Profile == "High"
			rotated := false
			if raw := s.Tags["rotate"]; raw != "" {
				angle, err := strconv.ParseFloat(raw, 64)
				rotated = err != nil || angle != 0
			}
			for _, data := range s.SideData {
				if data.Rotation != 0 {
					rotated = true
				}
			}
			// Keep browser compatibility, the existing width cap, and correct orientation.
			// 10-bit/HDR, oversized, rotated and non-H.264 sources need encoding.
			plan.copyVideo = s.CodecName == "h264" && s.PixelFormat == "yuv420p" && profile && s.Level > 0 && s.Level <= 42 && s.Width > 0 && s.Width <= 1280 && s.Width%2 == 0 && s.Height > 0 && s.Height <= 2160 && s.Height%2 == 0 && !rotated && s.Disposition.AttachedPic == 0
		}
		if s.CodecType == "audio" && !audioSeen {
			audioSeen = true
			plan.copyAudio = s.CodecName == "aac" && s.Profile == "LC" && s.Channels > 0 && s.Channels <= 2
		}
	}
	// With no audio, an optional -map creates no audio encoder or output stream.
	if !audioSeen {
		plan.copyAudio = true
	}
	return plan
}

func probeHLSPlan(ctx context.Context, input string) hlsPlan {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,crypto", "-show_streams", "-of", "json", input)
	var output boundedProbeOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return hlsPlan{}
	}
	var result struct {
		Streams []hlsProbeStream `json:"streams"`
	}
	if json.Unmarshal(output.data, &result) != nil {
		return hlsPlan{}
	}
	return chooseHLSPlan(result.Streams)
}

// Reject unexpectedly large probe metadata, rather than retaining it in memory.
type boundedProbeOutput struct{ data []byte }

func (b *boundedProbeOutput) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > 1024*1024 {
		return 0, errors.New("video probe metadata too large")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (p hlsPlan) codecArgs() []string {
	var args []string
	if p.copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-vf", "scale=w='trunc(min(1280,iw)/2)*2':h=-2", "-force_key_frames", "expr:gte(t,n_forced*6)")
	}
	if p.copyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-ac", "2")
	}
	return args
}

func (p hlsPlan) mode() string {
	if p.copyVideo && p.copyAudio {
		return "copy"
	}
	if p.copyVideo {
		return "copy-video/encode-audio"
	}
	return "encode-video"
}

func hlsFFmpegArgs(dir string, plan hlsPlan) []string {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "file,crypto", "-i", filepath.Join(dir, "input"), "-map", "0:v:0", "-map", "0:a:0?"}
	args = append(args, plan.codecArgs()...)
	return append(args, "-f", "hls", "-hls_time", "6", "-hls_playlist_type", "vod", "-hls_key_info_file", filepath.Join(dir, "keyinfo"), "-hls_segment_filename", filepath.Join(dir, "segment%06d.ts"), filepath.Join(dir, "index.m3u8"))
}
