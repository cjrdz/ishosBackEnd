package cache

import (
	"testing"
	"time"
)

func TestTTLCache_BasicGetSet(t *testing.T) {
	c := New(time.Minute)

	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected missing key to be absent")
	}

	c.Set("key", "value")
	v, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to be present")
	}
	if v != "value" {
		t.Fatalf("expected value %q, got %v", "value", v)
	}

	c.Set("key", "updated")
	v, ok = c.Get("key")
	if !ok || v != "updated" {
		t.Fatalf("expected updated value, got %v, ok=%v", v, ok)
	}
}

func TestTTLCache_Expiration(t *testing.T) {
	c := New(50 * time.Millisecond)

	c.Set("key", "value")
	if _, ok := c.Get("key"); !ok {
		t.Fatal("expected key to be present before TTL")
	}

	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("key"); ok {
		t.Fatal("expected key to be expired")
	}
}

func TestTTLCache_LRU_Eviction(t *testing.T) {
	c := NewWithSize(time.Minute, 2)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // exceeds maxSize, evicts least recently used (a)

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected 'a' to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("expected 'b' to be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected 'c' to be present")
	}
}

func TestTTLCache_LRU_OrderUpdatedOnGet(t *testing.T) {
	c := NewWithSize(time.Minute, 2)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Get("a")    // touch 'a' so it becomes most recently used
	c.Set("c", 3) // should evict 'b'

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("expected 'a' to be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected 'c' to be present")
	}
}

func TestTTLCache_LRU_OrderUpdatedOnSet(t *testing.T) {
	c := NewWithSize(time.Minute, 2)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("a", 10) // touch 'a'
	c.Set("c", 3)  // should evict 'b'

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}
	v, ok := c.Get("a")
	if !ok || v != 10 {
		t.Fatalf("expected 'a' = 10, got %v, ok=%v", v, ok)
	}
}

func TestTTLCache_Delete(t *testing.T) {
	c := New(time.Minute)

	c.Set("key", "value")
	c.Delete("key")
	if _, ok := c.Get("key"); ok {
		t.Fatal("expected key to be deleted")
	}

	// Deleting a missing key must not panic.
	c.Delete("missing")
}

func TestTTLCache_InvalidateAll(t *testing.T) {
	c := New(time.Minute)

	c.Set("a", 1)
	c.Set("b", 2)
	c.InvalidateAll()

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected 'a' to be cleared")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("expected 'b' to be cleared")
	}

	c.Set("c", 3)
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Fatalf("expected cache to work after invalidate, got %v, ok=%v", v, ok)
	}
}

func TestTTLCache_NoEvictionWhenMaxSizeZero(t *testing.T) {
	c := New(time.Minute)

	for i := 0; i < 100; i++ {
		c.Set(string(rune('a'+i%26)), i)
	}

	// With no max size, no entries should be evicted.
	if len(c.items) < 26 {
		t.Fatalf("expected all distinct keys to remain, got %d", len(c.items))
	}
}

func TestTTLCache_ConcurrentAccess(t *testing.T) {
	c := NewWithSize(time.Minute, 10)
	done := make(chan struct{})

	for i := 0; i < 4; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + (id+j)%26))
				c.Set(key, j)
				c.Get(key)
			}
		}(i)
	}

	for i := 0; i < 4; i++ {
		<-done
	}

	if len(c.items) > 10 {
		t.Fatalf("expected size at most %d, got %d", 10, len(c.items))
	}
}
