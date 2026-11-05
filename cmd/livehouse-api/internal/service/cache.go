package service

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

type ttlCache[K comparable, V any] struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[K]cacheEntry[V]
}

func newTTLCache[K comparable, V any](ttl time.Duration) *ttlCache[K, V] {
	return &ttlCache[K, V]{ttl: ttl, entries: make(map[K]cacheEntry[V])}
}

func (c *ttlCache[K, V]) get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || !time.Now().Before(entry.expiresAt) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (c *ttlCache[K, V]) set(key K, value V) {
	entry := cacheEntry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	time.AfterFunc(c.ttl, func() {
		c.mu.Lock()
		if current, ok := c.entries[key]; ok && current.expiresAt.Equal(entry.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
	})
}
