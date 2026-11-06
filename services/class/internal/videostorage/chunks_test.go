package videostorage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestChunkUploadRetryAndAtomicFinish(t *testing.T) {
	cfg := testSFTPServer(t)
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, close, err := s.connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Mkdir(cfg.Root)
	close()
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("b", 32) + ".mp4"
	if err = s.WriteChunk(ctx, key, 0, strings.NewReader("abcBAD")); err != nil {
		t.Fatal(err)
	}
	// Lost DB commit: truncate the uncommitted suffix when resending the same offset.
	if err = s.WriteChunk(ctx, key, 0, strings.NewReader("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Open(ctx, key); err == nil {
		t.Fatal("incomplete video is visible")
	}
	if err = s.WriteChunk(ctx, key, 9, strings.NewReader("gap")); err == nil {
		t.Fatal("accepted missing chunk")
	}
	if err = s.WriteChunk(ctx, key, 3, strings.NewReader("def")); err != nil {
		t.Fatal(err)
	}
	if err = s.FinishUpload(ctx, key, 7); err == nil {
		t.Fatal("accepted incorrect total")
	}
	if err = s.FinishUpload(ctx, key, 6); err != nil {
		t.Fatal(err)
	}
	if err = s.FinishUpload(ctx, key, 6); err != nil {
		t.Fatalf("finish retry: %v", err)
	}
	f, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "abcdef" {
		t.Fatalf("%q %v", data, err)
	}
	if err = s.AbortUpload(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err = s.WriteChunk(ctx, "../bad", 0, strings.NewReader("x")); err == nil {
		t.Fatal("accepted traversal")
	}
}
