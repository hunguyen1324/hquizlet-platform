package storage

import (
	"container/list"
	"sync"
)

// audioCache is a small in-process LRU of decoded audio bodies. It sits behind the
// authorization checks (Open/Ready are only called after the session and entitlement
// were verified), so it never serves audio to a caller who was not allowed to ask.
// Reading a 30 MB bytea from Postgres on every Range request was the main cost.
type audioCache struct {
	mu      sync.Mutex
	max     int
	size    int
	order   *list.List // front = most recently used
	entries map[string]*list.Element
}

type audioCacheEntry struct {
	key  string
	data []byte
	mime string
}

func newAudioCache(maxBytes int) *audioCache {
	return &audioCache{max: maxBytes, order: list.New(), entries: map[string]*list.Element{}}
}

func (c *audioCache) get(key string) ([]byte, string, bool) {
	if c == nil {
		return nil, "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil, "", false
	}
	c.order.MoveToFront(el)
	e := el.Value.(*audioCacheEntry)
	return e.data, e.mime, true
}

func (c *audioCache) put(key string, data []byte, mime string) {
	// Keep any single file below half the budget so one big file cannot flush everything.
	if c == nil || len(data) == 0 || len(data) > c.max/2 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.order.MoveToFront(el)
		return
	}
	c.entries[key] = c.order.PushFront(&audioCacheEntry{key: key, data: data, mime: mime})
	c.size += len(data)
	for c.size > c.max {
		last := c.order.Back()
		if last == nil {
			break
		}
		e := c.order.Remove(last).(*audioCacheEntry)
		delete(c.entries, e.key)
		c.size -= len(e.data)
	}
}
