package storage

import (
	"net"
	"testing"
)

func TestAudioSourceAllowlistAndSSRF(t *testing.T) {
	a := NewQuizAudio(nil, "cdn.example.com")
	for _, raw := range []string{"http://cdn.example.com/a.mp3", "https://cdn.example.com.evil.test/a", "https://user:pass@cdn.example.com/a", "https://cdn.example.com:444/a", "file:///etc/passwd", "https://127.0.0.1/a"} {
		if a.allowed(raw) {
			t.Errorf("allowed unsafe source %s", raw)
		}
	}
	if !a.allowed("https://cdn.example.com/a.mp3?signature=test") {
		t.Fatal("allowed host rejected")
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "2002:7f00:1::"} {
		if publicAudioIP(net.ParseIP(raw)) {
			t.Errorf("SSRF address allowed %s", raw)
		}
	}
	if !publicAudioIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	if _, err := audioMIME([]byte("<html>not audio</html>")); err == nil {
		t.Fatal("HTML accepted as audio")
	}
	if _, err := audioMIME([]byte("ID3\x04\x00\x00\x00\x00\x00\x00")); err != nil {
		t.Fatal(err)
	}
}

func TestMP3WithoutID3(t *testing.T) {
	if mime, err := audioMIME([]byte{0xff, 0xfb, 0x90, 0x60, 0, 0, 3, 4}); err != nil || mime != "audio/mpeg" {
		t.Fatalf("untagged MPEG frame rejected: %s %v", mime, err)
	}
	for _, data := range [][]byte{{0xff, 0xfb}, {0xff, 0xfb, 0xf0, 0}, {0xff, 0xfb, 0x9c, 0}, {0xff, 0xe8, 0x90, 0}} {
		if _, err := audioMIME(data); err == nil {
			t.Fatalf("invalid MPEG header accepted: %x", data)
		}
	}
}

func TestAudioCacheEvictsLeastRecentlyUsedAndSkipsHugeFiles(t *testing.T) {
	c := newAudioCache(100)
	c.put("a", make([]byte, 40), "audio/mpeg")
	c.put("b", make([]byte, 40), "audio/mpeg")
	if _, _, ok := c.get("a"); !ok { // touch a so b becomes the oldest
		t.Fatal("a missing")
	}
	c.put("c", make([]byte, 40), "audio/mpeg")
	if _, _, ok := c.get("b"); ok {
		t.Fatal("least recently used entry was not evicted")
	}
	if _, _, ok := c.get("a"); !ok {
		t.Fatal("recently used entry was evicted")
	}
	c.put("huge", make([]byte, 80), "audio/mpeg") // more than half the budget
	if _, _, ok := c.get("huge"); ok {
		t.Fatal("oversized file must not be cached")
	}
	var nilCache *audioCache
	nilCache.put("x", []byte{1}, "audio/mpeg") // a disabled cache must be a no-op
	if _, _, ok := nilCache.get("x"); ok {
		t.Fatal("nil cache returned data")
	}
}
