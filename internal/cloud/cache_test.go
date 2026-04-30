package cloud

import (
	"testing"
)

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache(1024)
	if cache.maxBytes != 1024 {
		t.Errorf("maxBytes = %d, want 1024", cache.maxBytes)
	}
	if cache.currentBytes != 0 {
		t.Errorf("currentBytes = %d, want 0", cache.currentBytes)
	}
}

func TestPutAndGet(t *testing.T) {
	cache := NewLRUCache(1024)
	cache.Put("key1", []byte("value1"))

	data, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(data) != "value1" {
		t.Errorf("got %q, want %q", data, "value1")
	}
}

func TestGetMiss(t *testing.T) {
	cache := NewLRUCache(1024)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestPutUpdateExisting(t *testing.T) {
	cache := NewLRUCache(1024)
	cache.Put("key1", []byte("v1"))
	cache.Put("key1", []byte("v2-updated"))

	data, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(data) != "v2-updated" {
		t.Errorf("got %q, want %q", data, "v2-updated")
	}

	stats := cache.Stats()
	if stats.Entries != 1 {
		t.Errorf("entries = %d, want 1", stats.Entries)
	}
}

func TestEviction(t *testing.T) {
	cache := NewLRUCache(10)
	cache.Put("a", []byte("12345"))  // 5 bytes
	cache.Put("b", []byte("67890"))  // 5 bytes, total 10
	cache.Put("c", []byte("abcde")) // 5 bytes, evicts "a"

	_, ok := cache.Get("a")
	if ok {
		t.Error("expected 'a' to be evicted")
	}

	_, ok = cache.Get("b")
	if !ok {
		t.Error("expected 'b' to be present")
	}

	_, ok = cache.Get("c")
	if !ok {
		t.Error("expected 'c' to be present")
	}
}

func TestEvictionOrder(t *testing.T) {
	cache := NewLRUCache(10)
	cache.Put("a", []byte("12345")) // 5
	cache.Put("b", []byte("67890")) // 5, total 10

	cache.Get("a") // access "a" to make it recent

	cache.Put("c", []byte("xxxxx")) // 5, should evict "b" (least recent)

	_, ok := cache.Get("b")
	if ok {
		t.Error("expected 'b' to be evicted")
	}

	_, ok = cache.Get("a")
	if !ok {
		t.Error("expected 'a' to be present")
	}
}

func TestStats(t *testing.T) {
	cache := NewLRUCache(1024)
	cache.Put("k1", []byte("data1"))
	cache.Put("k2", []byte("data2"))

	cache.Get("k1")       // hit
	cache.Get("k1")       // hit
	cache.Get("missing")  // miss

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("hits = %d, want 2", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("misses = %d, want 1", stats.Misses)
	}
	if stats.Entries != 2 {
		t.Errorf("entries = %d, want 2", stats.Entries)
	}
	if stats.MaxBytes != 1024 {
		t.Errorf("maxBytes = %d, want 1024", stats.MaxBytes)
	}
	expectedRate := 2.0 / 3.0
	if stats.HitRate < expectedRate-0.01 || stats.HitRate > expectedRate+0.01 {
		t.Errorf("hitRate = %f, want ~%f", stats.HitRate, expectedRate)
	}
}

func TestStatsZeroDivision(t *testing.T) {
	cache := NewLRUCache(1024)
	stats := cache.Stats()
	if stats.HitRate != 0 {
		t.Errorf("hitRate = %f, want 0", stats.HitRate)
	}
}

func TestCurrentBytesTracking(t *testing.T) {
	cache := NewLRUCache(1024)
	cache.Put("a", []byte("hello"))     // 5 bytes
	cache.Put("b", []byte("world!!"))   // 7 bytes

	stats := cache.Stats()
	if stats.SizeBytes != 12 {
		t.Errorf("sizeBytes = %d, want 12", stats.SizeBytes)
	}

	cache.Put("a", []byte("hi")) // update: 5 -> 2 bytes
	stats = cache.Stats()
	if stats.SizeBytes != 9 {
		t.Errorf("sizeBytes = %d, want 9", stats.SizeBytes)
	}
}
