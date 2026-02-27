package render

import (
	"sync"
)

// PageCache is an in-memory LRU cache for rendered HTML pages.
// It is automatically invalidated when templates are reloaded.
type PageCache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	order    []string // LRU order (most recent at end)
	maxItems int
	version  int64 // incremented on invalidation
}

type cacheEntry struct {
	html    string
	version int64
}

// NewPageCache creates a cache with the given maximum number of entries.
func NewPageCache(maxItems int) *PageCache {
	if maxItems <= 0 {
		maxItems = 200
	}
	return &PageCache{
		entries:  make(map[string]*cacheEntry, maxItems),
		order:    make([]string, 0, maxItems),
		maxItems: maxItems,
	}
}

// Get returns a cached page if present and not stale. Returns ("", false) on miss.
func (c *PageCache) Get(key string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	ver := c.version
	c.mu.RUnlock()

	if !ok || entry.version != ver {
		return "", false
	}

	// Move to end of LRU (promote)
	c.mu.Lock()
	c.promote(key)
	c.mu.Unlock()

	return entry.html, true
}

// Set stores a rendered page in the cache.
func (c *PageCache) Set(key, html string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already exists, update it
	if _, ok := c.entries[key]; ok {
		c.entries[key] = &cacheEntry{html: html, version: c.version}
		c.promote(key)
		return
	}

	// Evict LRU if at capacity
	if len(c.order) >= c.maxItems {
		evict := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, evict)
	}

	c.entries[key] = &cacheEntry{html: html, version: c.version}
	c.order = append(c.order, key)
}

// Invalidate clears all cached entries by incrementing the version.
func (c *PageCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.version++
	// Clear the map to free memory
	c.entries = make(map[string]*cacheEntry, c.maxItems)
	c.order = c.order[:0]
}

// Size returns the number of cached entries.
func (c *PageCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *PageCache) promote(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}
