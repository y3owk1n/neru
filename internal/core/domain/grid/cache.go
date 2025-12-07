package grid

import (
	"container/list"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultCacheSize is the default cache size.
	DefaultCacheSize = 8
	// DefaultGridCacheTTL is the default cache TTL for grid.
	DefaultGridCacheTTL = 1 * time.Hour
)

// CacheKey is a key for the grid cache.
type CacheKey struct {
	characters string
	rowLabels  string
	colLabels  string
	width      int
	height     int
}

// CacheEntry is an entry in the grid cache.
type CacheEntry struct {
	cells   []*Cell
	addedAt time.Time
	usedAt  time.Time
}

// Cache implements an LRU cache for grid cells to improve performance by reusing previously computed grids.
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

// Prewarm initializes the grid cache with commonly used grid sizes to improve startup performance.
func Prewarm(characters string, sizes []image.Rectangle) {
	if !gridCacheEnabled {
		return
	}

	for _, rect := range sizes {
		if _, ok := gridCache.get(characters, "", "", rect); ok {
			continue
		}

		grid := NewGrid(characters, rect, zap.NewNop())
		_ = grid
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

func (c *Cache) get(
	characters, rowLabels, colLabels string,
	bounds image.Rectangle,
) ([]*Cell, bool) {
	cacheKey := CacheKey{
		characters: characters,
		rowLabels:  rowLabels,
		colLabels:  colLabels,
		width:      bounds.Dx(),
		height:     bounds.Dy(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[cacheKey]

	if ok {
		if entry, ok := element.Value.(*CacheEntry); ok {
			// Check TTL
			if time.Since(entry.addedAt) > c.ttl {
				// Evict expired entry
				c.order.Remove(element)
				delete(c.items, cacheKey)

				return nil, false
			}

			entry.usedAt = time.Now()

			c.order.MoveToFront(element)

			return entry.cells, true
		}
	}

	return nil, false
}

func (c *Cache) put(
	characters, rowLabels, colLabels string,
	bounds image.Rectangle,
	cells []*Cell,
) {
	cacheKey := CacheKey{
		characters: characters,
		rowLabels:  rowLabels,
		colLabels:  colLabels,
		width:      bounds.Dx(),
		height:     bounds.Dy(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[cacheKey]; ok {
		entry, ok := element.Value.(*CacheEntry)

		if ok {
			entry.cells = cells
			entry.usedAt = time.Now()
			entry.addedAt = entry.usedAt

			c.order.MoveToFront(element)

			return
		}
		// replace unexpected type
		newEntry := &CacheEntry{cells: cells, addedAt: time.Now(), usedAt: time.Now()}
		element.Value = newEntry
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
