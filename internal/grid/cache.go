// Package grid provides grid mode functionality for the Neru application.
package grid

// Package grid provides grid-based navigation functionality for the Neru application.
// including grid generation, caching, and overlay rendering.

import (
	"container/list"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"
)

type gridCacheKey struct {
	characters string
	width      int
	height     int
}

type gridCacheEntry struct {
	cells   []*Cell
	addedAt time.Time
	usedAt  time.Time
}

// Cache implements a cache for grid cells to improve performance.
type Cache struct {
	mu       sync.Mutex
	items    map[gridCacheKey]*list.Element
	order    *list.List
	capacity int
}

var (
	gridCache        = newCache(8)
	gridCacheEnabled = true
)

// SetGridCacheEnabled enables or disables the grid cache.
func SetGridCacheEnabled(enabled bool) {
	gridCacheEnabled = enabled
}

// Prewarm prewarms the grid cache with common grid sizes.
func Prewarm(characters string, sizes []image.Rectangle) {
	if !gridCacheEnabled {
		return
	}
	for _, r := range sizes {
		if _, ok := gridCache.get(characters, r); ok {
			continue
		}
		g := NewGrid(characters, r, zap.NewNop())
		_ = g
	}
}

func newCache(capacity int) *Cache {
	return &Cache{
		items:    make(map[gridCacheKey]*list.Element),
		order:    list.New(),
		capacity: capacity,
	}
}

func (c *Cache) get(characters string, bounds image.Rectangle) ([]*Cell, bool) {
	key := gridCacheKey{characters: characters, width: bounds.Dx(), height: bounds.Dy()}
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		if entry, ok := element.Value.(*gridCacheEntry); ok {
			entry.usedAt = time.Now()
			c.order.MoveToFront(element)
			return entry.cells, true
		}
	}
	return nil, false
}

func (c *Cache) put(characters string, bounds image.Rectangle, cells []*Cell) {
	key := gridCacheKey{characters: characters, width: bounds.Dx(), height: bounds.Dy()}
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		if entry, ok := element.Value.(*gridCacheEntry); ok {
			entry.cells = cells
			entry.usedAt = time.Now()
			entry.addedAt = entry.usedAt
			c.order.MoveToFront(element)
			return
		}
		// replace unexpected type
		newEntry := &gridCacheEntry{cells: cells, addedAt: time.Now(), usedAt: time.Now()}
		element.Value = newEntry
		c.order.MoveToFront(element)
		return
	}
	entry := &gridCacheEntry{cells: cells, addedAt: time.Now(), usedAt: time.Now()}
	element := c.order.PushFront(entry)
	c.items[key] = element
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
