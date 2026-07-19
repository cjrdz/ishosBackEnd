// Package cache provides a simple thread-safe in-memory TTL cache. Entries expire
// lazily on the next Get. An optional max-size LRU eviction can be enabled with
// NewWithSize.
package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	value     interface{}
	expiresAt time.Time
}

// TTLCache is a thread-safe in-memory cache where every entry has the same TTL.
// When created with a positive maxSize, it evicts the least-recently-used entry
// once the cache grows beyond maxSize.
type TTLCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxSize int
	items   map[string]*list.Element
	lru     *list.List
}

// New returns a TTLCache whose entries expire after ttl from the time they are Set.
func New(ttl time.Duration) *TTLCache {
	return NewWithSize(ttl, 0)
}

// NewWithSize returns a TTLCache with TTL expiration and LRU eviction. Once the
// number of entries exceeds maxSize, the least-recently-used entry is removed.
// A maxSize of zero disables size-based eviction.
func NewWithSize(ttl time.Duration, maxSize int) *TTLCache {
	return &TTLCache{
		ttl:     ttl,
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lru:     list.New(),
	}
}

// Get returns the cached value and true if the key exists and has not expired.
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*lruEntry).entry
	if time.Now().After(e.expiresAt) {
		c.removeElement(el)
		return nil, false
	}
	c.lru.MoveToFront(el)
	return e.value, true
}

// Set stores value under key; the entry expires after the cache TTL.
func (c *TTLCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(c.ttl)
	if el, ok := c.items[key]; ok {
		le := el.Value.(*lruEntry)
		le.entry.value = value
		le.entry.expiresAt = expiresAt
		c.lru.MoveToFront(el)
		return
	}

	le := &lruEntry{key: key, entry: entry{value: value, expiresAt: expiresAt}}
	el := c.lru.PushFront(le)
	c.items[key] = el
	c.evictIfNeeded()
}

// Delete removes a single key from the cache.
func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// InvalidateAll removes every entry, forcing the next read to re-fetch.
func (c *TTLCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lru = list.New()
}

func (c *TTLCache) removeElement(el *list.Element) {
	c.lru.Remove(el)
	delete(c.items, el.Value.(*lruEntry).key)
}

func (c *TTLCache) evictIfNeeded() {
	for c.maxSize > 0 && len(c.items) > c.maxSize {
		el := c.lru.Back()
		if el == nil {
			break
		}
		c.removeElement(el)
	}
}

// lruEntry ties a cache key to its entry so the list can be maintained without
// a second lookup.
type lruEntry struct {
	key   string
	entry entry
}
