package grid

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

// Cache implements an LRU cache for grid cells to improve performance by reusing previously computed grids.
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

// SetGridCacheEnabled enables or disables the grid cell caching mechanism.
func SetGridCacheEnabled(enabled bool) {
	gridCacheEnabled = enabled
}

// Prewarm initializes the grid cache with commonly used grid sizes to improve startup performance.
func Prewarm(characters string, sizes []image.Rectangle) {
	if !gridCacheEnabled {
		return
	}

	for _, rect := range sizes {
		if _, ok := gridCache.get(characters, rect); ok {
			continue
		}

		grid := NewGrid(characters, rect, zap.NewNop())
		_ = grid
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
	cacheKey := gridCacheKey{characters: characters, width: bounds.Dx(), height: bounds.Dy()}

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[cacheKey]

	if ok {
		if entry, ok := element.Value.(*gridCacheEntry); ok {
			entry.usedAt = time.Now()

			c.order.MoveToFront(element)

			return entry.cells, true
		}
	}

	return nil, false
}

func (c *Cache) put(characters string, bounds image.Rectangle, cells []*Cell) {
	cacheKey := gridCacheKey{characters: characters, width: bounds.Dx(), height: bounds.Dy()}

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[cacheKey]; ok {
		entry, ok := element.Value.(*gridCacheEntry)

		if ok {
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
