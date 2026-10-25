package grid

import (
	"container/list"
	"image"
	"sync"
	"time"
)

const (
	// DefaultCacheSize is the default cache size.
	DefaultCacheSize = 8
	// DefaultGridCacheTTL is the default cache TTL for grid.
	DefaultGridCacheTTL = 1 * time.Hour
)

// CacheKey is a key for the grid cache.
type CacheKey struct {
	characters     string
	rowLabels      string
	colLabels      string
	maxLabelLength int
	displayScale   float64
	width          int
	height         int
}

// CacheEntry is an entry in the grid cache.
type CacheEntry struct {
	cells   []*Cell
	addedAt time.Time
	usedAt  time.Time
}

// Cache is an LRU cache of computed grid cells, so a grid built again for the
// same inputs reuses them.
type Cache struct {
	mu       sync.Mutex
	items    map[CacheKey]*list.Element
	order    *list.List
	capacity int
	ttl      time.Duration
}

var (
	gridCache        = newCache(DefaultCacheSize, DefaultGridCacheTTL)
	gridCacheEnabled = true
)

func newCacheKey(
	alpha gridAlphabet, maxLabelLength int, displayScale float64, bounds image.Rectangle,
) CacheKey {
	return CacheKey{
		characters:     alpha.characters,
		rowLabels:      string(alpha.rowChars),
		colLabels:      string(alpha.colChars),
		maxLabelLength: maxLabelLength,
		displayScale:   displayScale,
		width:          bounds.Dx(),
		height:         bounds.Dy(),
	}
}

func newCache(capacity int, ttl time.Duration) *Cache {
	return &Cache{
		items:    make(map[CacheKey]*list.Element),
		order:    list.New(),
		capacity: capacity,
		ttl:      ttl,
	}
}

func (c *Cache) get(cacheKey CacheKey) ([]*Cell, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, found := c.items[cacheKey]
	if !found {
		return nil, false
	}

	entry, isEntry := element.Value.(*CacheEntry)
	if !isEntry {
		return nil, false
	}

	if time.Since(entry.addedAt) > c.ttl {
		c.order.Remove(element)
		delete(c.items, cacheKey)

		return nil, false
	}

	entry.usedAt = time.Now()

	c.order.MoveToFront(element)

	return entry.cells, true
}

func (c *Cache) put(cacheKey CacheKey, cells []*Cell) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[cacheKey]; ok {
		now := time.Now()
		element.Value = &CacheEntry{cells: cells, addedAt: now, usedAt: now}
		c.order.MoveToFront(element)

		return
	}

	entry := &CacheEntry{cells: cells, addedAt: time.Now(), usedAt: time.Now()}
	element := c.order.PushFront(entry)

	c.items[cacheKey] = element
	if c.order.Len() > c.capacity {
		tail := c.order.Back()
		if tail != nil {
			c.order.Remove(tail)

			for cacheKey, value := range c.items {
				if value == tail {
					delete(c.items, cacheKey)

					break
				}
			}
		}
	}
}
