package http

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHLSPlanPreservesBrowserCompatibility(t *testing.T) {
	compatible := hlsProbeStream{CodecType: "video", CodecName: "h264", Profile: "High", PixelFormat: "yuv420p", Width: 1280, Height: 720, Level: 31}
	aac := hlsProbeStream{CodecType: "audio", CodecName: "aac", Profile: "LC", Channels: 2}
	if p := chooseHLSPlan([]hlsProbeStream{compatible, aac}); !p.copyVideo || !p.copyAudio {
		t.Fatal("compatible source should remux")
	}
	if p := chooseHLSPlan([]hlsProbeStream{compatible, {CodecType: "audio", CodecName: "opus", Channels: 2}}); !p.copyVideo || p.copyAudio {
		t.Fatal("only incompatible audio should encode")
	}
	for _, tc := range []struct {
		name   string
		change func(*hlsProbeStream)
	}{
		{"VP9", func(s *hlsProbeStream) { s.CodecName = "vp9" }},
		{"10 bit", func(s *hlsProbeStream) { s.PixelFormat = "yuv420p10le" }},
		{"oversized", func(s *hlsProbeStream) { s.Width = 1920 }},
		{"unknown dimensions", func(s *hlsProbeStream) { s.Width = 0 }},
		{"odd width", func(s *hlsProbeStream) { s.Width = 1279 }},
		{"high level", func(s *hlsProbeStream) { s.Level = 51 }},
		{"unsupported profile", func(s *hlsProbeStream) { s.Profile = "High 4:4:4 Predictive" }},
		{"rotation", func(s *hlsProbeStream) { s.Tags = map[string]string{"rotate": "90"} }},
		{"cover art", func(s *hlsProbeStream) { s.Disposition.AttachedPic = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := compatible
			tc.change(&s)
			if chooseHLSPlan([]hlsProbeStream{s, aac}).copyVideo {
				t.Fatal("unsafe copy")
			}
		})
	}
	if p := chooseHLSPlan(nil); p.copyVideo {
		t.Fatal("unknown source cannot copy")
	}
	if p := chooseHLSPlan([]hlsProbeStream{compatible}); !p.copyAudio {
		t.Fatal("silent video needs no audio encoding")
	}
}

func TestHLSCodecPlanOnlyEncodesNecessaryStreams(t *testing.T) {
	for _, tc := range []struct {
		plan         hlsPlan
		video, audio string
	}{{hlsPlan{true, true}, "copy", "copy"}, {hlsPlan{true, false}, "copy", "aac"}, {hlsPlan{false, true}, "libx264", "copy"}} {
		args := strings.Join(tc.plan.codecArgs(), " ")
		if !strings.Contains(args, "-c:v "+tc.video) || !strings.Contains(args, "-c:a "+tc.audio) {
			t.Fatal(args)
		}
		if tc.plan.copyVideo && (strings.Contains(args, "-vf") || strings.Contains(args, "-force_key_frames")) {
			t.Fatal("copy cannot filter or force frames")
		}
	}
}

func TestHLSProbeAndVP9EncryptedFallback(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe required")
	}
	file := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(file, realVideoFixture(t), 0600); err != nil {
		t.Fatal(err)
	}
	if p := probeHLSPlan(context.Background(), file); !p.copyVideo {
		t.Fatal("real H264 should use fast path")
	}
	if p := probeHLSPlan(context.Background(), file+".missing"); p.copyVideo {
		t.Fatal("probe failure must encode")
	}
	webm := filepath.Join(t.TempDir(), "source.webm")
	cmd := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=160x120:r=10", "-t", "1", "-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", webm)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	if p := probeHLSPlan(context.Background(), webm); p.copyVideo {
		t.Fatal("VP9 must be encoded")
	}
	data, err := os.ReadFile(webm)
	if err != nil {
		t.Fatal(err)
	}
	checkHLSEncryption(t, data)
}

func BenchmarkHLSProcessing(b *testing.B) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		b.Skip("ffmpeg required")
	}
	source := filepath.Join(b.TempDir(), "source.mp4")
	cmd := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=s=1280x720:r=30", "-t", "4", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-g", "60", source)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("fixture: %v %s", err, out)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		plan hlsPlan
	}{{"encode", hlsPlan{}}, {"copy", hlsPlan{true, true}}} {
		b.Run(tc.name, func(b *testing.B) {
			dir := b.TempDir()
			for name, body := range map[string][]byte{"input": data, "key": make([]byte, 16), "keyinfo": []byte("key\n" + filepath.Join(dir, "key") + "\n")} {
				if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
					b.Fatal(err)
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				args := append([]string{"-y"}, hlsFFmpegArgs(dir, tc.plan)...)
				cmd := exec.Command("ffmpeg", args...)
				if out, err := cmd.CombinedOutput(); err != nil {
					b.Fatalf("processing: %v %s", err, out)
				}
			}
		})
	}
}
