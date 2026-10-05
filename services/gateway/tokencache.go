package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type cachedIdentity struct {
	identity verifiedIdentity
	expires  time.Time
}

type tokenCache struct {
	mu  sync.RWMutex
	m   map[string]cachedIdentity
	ttl time.Duration
}

func newTokenCache(ttl time.Duration) *tokenCache {
	c := &tokenCache{m: make(map[string]cachedIdentity), ttl: ttl}
	go c.janitor()
	return c
}

func tokenKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (c *tokenCache) Get(token string) (verifiedIdentity, bool) {
	c.mu.RLock()
	v, ok := c.m[tokenKey(token)]
	c.mu.RUnlock()
	if !ok || time.Now().After(v.expires) {
		return verifiedIdentity{}, false
	}
	return v.identity, true
}

func (c *tokenCache) Set(token string, id verifiedIdentity) {
	c.mu.Lock()
	c.m[tokenKey(token)] = cachedIdentity{id, time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *tokenCache) Delete(token string) {
	c.mu.Lock()
	delete(c.m, tokenKey(token))
	c.mu.Unlock()
}

func (c *tokenCache) janitor() {
	for range time.Tick(time.Minute) {
		now := time.Now()
		c.mu.Lock()
		for k, v := range c.m {
			if now.After(v.expires) {
				delete(c.m, k)
			}
		}
		c.mu.Unlock()
	}
}
