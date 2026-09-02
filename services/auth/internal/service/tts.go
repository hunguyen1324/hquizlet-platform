package service

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Language represents a supported TTS language.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Flag string `json:"flag"`
}

// Languages is the list of supported TTS languages.
var Languages = []Language{
	{Code: "en-US", Name: "English (US)", Flag: "🇺🇸"},
	{Code: "vi-VN", Name: "Tiếng Việt", Flag: "🇻🇳"},
	{Code: "ja-JP", Name: "日本語", Flag: "🇯🇵"},
	{Code: "ko-KR", Name: "한국어", Flag: "🇰🇷"},
	{Code: "zh-CN", Name: "中文 (简体)", Flag: "🇨🇳"},
	{Code: "zh-TW", Name: "中文 (繁體)", Flag: "🇹🇼"},
	{Code: "fr-FR", Name: "Français", Flag: "🇫🇷"},
	{Code: "de-DE", Name: "Deutsch", Flag: "🇩🇪"},
	{Code: "es-ES", Name: "Español", Flag: "🇪🇸"},
	{Code: "th-TH", Name: "ไทย", Flag: "🇹🇭"},
	{Code: "id-ID", Name: "Bahasa Indonesia", Flag: "🇮🇩"},
}

// TTSService handles text-to-speech operations.
type TTSService struct {
	googleKey string
	cache     map[string][]byte
	cacheMu   sync.RWMutex
	cacheTTL  time.Duration
}

// NewTTSService creates a new TTS service.
func NewTTSService() *TTSService {
	return &TTSService{
		googleKey: os.Getenv("GOOGLE_TTS_KEY"),
		cache:     make(map[string][]byte),
		cacheTTL:  24 * time.Hour,
	}
}

// GetAudio returns MP3 audio for the given text and language.
// It first checks the in-memory cache, then tries Google Cloud TTS,
// and falls back to espeak if no API key is configured.
func (s *TTSService) GetAudio(text, lang string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	key := s.cacheKey(text, lang)

	// Check cache
	s.cacheMu.RLock()
	if cached, ok := s.cache[key]; ok {
		s.cacheMu.RUnlock()
		return cached, nil
	}
	s.cacheMu.RUnlock()

	var audio []byte
	var err error

	if s.googleKey != "" {
		audio, err = s.googleTTS(text, lang)
		if err != nil {
			log.Printf("[tts] google TTS failed, falling back to espeak: %v", err)
			audio, err = s.espeakFallback(text, lang)
		}
	} else {
		log.Printf("[tts] no GOOGLE_TTS_KEY configured, using espeak fallback")
		audio, err = s.espeakFallback(text, lang)
	}

	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cacheMu.Lock()
	s.cache[key] = audio
	s.cacheMu.Unlock()

	return audio, nil
}

// cacheKey generates a SHA-256 cache key from text + lang.
func (s *TTSService) cacheKey(text, lang string) string {
	h := sha256.Sum256([]byte(text + "|" + lang))
	return fmt.Sprintf("%x", h)
}

// googleTTS calls Google Cloud Text-to-Speech API.
// This is a placeholder implementation — in production, use the Google Cloud SDK.
func (s *TTSService) googleTTS(text, lang string) ([]byte, error) {
	// TODO: Implement actual Google Cloud TTS API call
	// For now, return an error to trigger espeak fallback
	return nil, fmt.Errorf("google TTS not implemented yet")
}

// espeakFallback uses the espeak command-line tool to generate speech.
func (s *TTSService) espeakFallback(text, lang string) ([]byte, error) {
	// Map BCP-47 codes to espeak voice names
	voiceMap := map[string]string{
		"en-US": "en-us",
		"vi-VN": "vi",
		"ja-JP": "ja",
		"ko-KR": "ko",
		"zh-CN": "cmn",
		"zh-TW": "cmn",
		"fr-FR": "fr",
		"de-DE": "de",
		"es-ES": "es",
		"th-TH": "th",
		"id-ID": "id",
	}

	voice := voiceMap[lang]
	if voice == "" {
		voice = "en-us"
	}

	// Generate WAV with espeak, then convert to raw MP3-like output
	cmd := exec.Command("espeak", "-v", voice, "-w", "/dev/stdout", text)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create espeak pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start espeak: %w", err)
	}

	audio, err := io.ReadAll(stdout)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to read espeak output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("espeak failed: %w", err)
	}

	return audio, nil
}
