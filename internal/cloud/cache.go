package cloud

import (
	"container/list"
	"sync"

	"github.com/yingshulu/famfun/internal/model"
)

type SegmentCache interface {
	Get(key string) ([]byte, bool)
	Put(key string, data []byte)
	Stats() model.CacheStats
}

type cacheEntry struct {
	key  string
	data []byte
}

type LRUCache struct {
	mu           sync.Mutex
	maxBytes     int64
	currentBytes int64
	list         *list.List
	items        map[string]*list.Element
	hits         int64
	misses       int64
}

func NewLRUCache(maxBytes int64) *LRUCache {
	return &LRUCache{
		maxBytes: maxBytes,
		list:     list.New(),
		items:    make(map[string]*list.Element),
	}
}

func (c *LRUCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}

	c.list.MoveToFront(elem)
	c.hits++
	return elem.Value.(*cacheEntry).data, true
}

func (c *LRUCache) Put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.updateExisting(elem, data)
		return
	}
	c.addNew(key, data)
	c.evict()
}

func (c *LRUCache) updateExisting(elem *list.Element, data []byte) {
	entry := elem.Value.(*cacheEntry)
	c.currentBytes += int64(len(data)) - int64(len(entry.data))
	entry.data = data
	c.list.MoveToFront(elem)
}

func (c *LRUCache) addNew(key string, data []byte) {
	entry := &cacheEntry{key: key, data: data}
	elem := c.list.PushFront(entry)
	c.items[key] = elem
	c.currentBytes += int64(len(data))
}

func (c *LRUCache) evict() {
	for c.currentBytes > c.maxBytes && c.list.Len() > 0 {
		c.evictOldest()
	}
}

func (c *LRUCache) evictOldest() {
	elem := c.list.Back()
	if elem == nil {
		return
	}
	entry := elem.Value.(*cacheEntry)
	c.list.Remove(elem)
	delete(c.items, entry.key)
	c.currentBytes -= int64(len(entry.data))
}

func (c *LRUCache) Stats() model.CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.hits + c.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return model.CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Entries:   c.list.Len(),
		SizeBytes: c.currentBytes,
		MaxBytes:  c.maxBytes,
		HitRate:   hitRate,
	}
}
