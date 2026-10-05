package main

import (
	"testing"
	"time"
)

func TestTokenCache(t *testing.T) {
	cache := newTokenCache(100 * time.Millisecond)
	token := "sample-token-123"
	id := verifiedIdentity{
		Authenticated: true,
		UserID:        1,
		Role:          "user",
	}

	// Test miss
	if _, ok := cache.Get(token); ok {
		t.Errorf("expected cache miss")
	}

	// Test set and hit
	cache.Set(token, id)
	if got, ok := cache.Get(token); !ok || got.UserID != id.UserID {
		t.Errorf("expected cache hit with UserID %d, got %v", id.UserID, got)
	}

	// Test delete
	cache.Delete(token)
	if _, ok := cache.Get(token); ok {
		t.Errorf("expected cache miss after delete")
	}

	// Test expire
	cache.Set(token, id)
	time.Sleep(150 * time.Millisecond)
	if _, ok := cache.Get(token); ok {
		t.Errorf("expected cache miss after expiration")
	}
}
