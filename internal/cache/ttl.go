// Package cache provides a simple thread-safe in-memory TTL cache backed by
// sync.Map. It is intentionally minimal - no background GC, entries expire
// lazily on the next Get.
package cache

import (
	"sync"
	"time"
)

type entry struct {
	value     interface{}
	expiresAt time.Time
}

// TTLCache is a thread-safe in-memory cache where every entry has the same TTL.
type TTLCache struct {
	m   sync.Map
	ttl time.Duration
}

// New returns a TTLCache whose entries expire after ttl from the time they are Set.
func New(ttl time.Duration) *TTLCache {
	return &TTLCache{ttl: ttl}
}

// Get returns the cached value and true if the key exists and has not expired.
func (c *TTLCache) Get(key string) (interface{}, bool) {
	v, ok := c.m.Load(key)
	if !ok {
		return nil, false
	}
	e := v.(entry)
	if time.Now().After(e.expiresAt) {
		c.m.Delete(key)
		return nil, false
	}
	return e.value, true
}

// Set stores value under key; the entry expires after the cache TTL.
func (c *TTLCache) Set(key string, value interface{}) {
	c.m.Store(key, entry{value: value, expiresAt: time.Now().Add(c.ttl)})
}

// Delete removes a single key from the cache.
func (c *TTLCache) Delete(key string) {
	c.m.Delete(key)
}

// InvalidateAll removes every entry, forcing the next read to re-fetch.
func (c *TTLCache) InvalidateAll() {
	c.m.Range(func(key, _ interface{}) bool {
		c.m.Delete(key)
		return true
	})
}
