package accessibility

import (
	"container/heap"
	"container/list"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultCacheSize is the default cache size.
	DefaultCacheSize = 100

	// DefaultMaxCacheSize is the default maximum cache size with LRU eviction.
	DefaultMaxCacheSize = 1000

	// CacheCleanupDivisor is the divisor for cleanup interval.
	CacheCleanupDivisor = 2

	// CacheDeletionEstimate is the estimate for deletion.
	CacheDeletionEstimate = 4

	// StaticElementTTL is the TTL for static UI elements (buttons, links, etc.).
	StaticElementTTL = 30 * time.Second

	// DynamicElementTTL is the TTL for dynamic UI elements (text fields, scrollable content, etc.).
	DynamicElementTTL = 2 * time.Second
)

// CachedInfo wraps ElementInfo with an expiration timestamp and LRU tracking for caching.
type CachedInfo struct {
	info        *ElementInfo
	expiresAt   time.Time
	key         uint64
	elementRef  *Element      // Retained reference for equality checks
	elementNode *list.Element // For LRU tracking
	heapIndex   int           // Index in expirationHeap (-1 = not in heap)
	removed     bool          // Marked as removed from cache (lazy heap cleanup)
}

// expirationHeap implements a min-heap ordered by expiresAt for efficient expired entry removal.
type expirationHeap []*CachedInfo

func (h *expirationHeap) Len() int { return len(*h) }

func (h *expirationHeap) Less(i, j int) bool { return (*h)[i].expiresAt.Before((*h)[j].expiresAt) }

func (h *expirationHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
	(*h)[i].heapIndex = i
	(*h)[j].heapIndex = j
}

func (h *expirationHeap) Push(x any) {
	item := x.(*CachedInfo) //nolint:forcetypeassert
	item.heapIndex = len(*h)
	*h = append(*h, item)
}

func (h *expirationHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	item.heapIndex = -1

	return item
}

// staticRoles defines roles that should use longer (static) TTL.
var staticRoles = map[string]bool{
	"AXButton":             true,
	"AXLink":               true,
	"AXMenuItem":           true,
	"AXMenuButton":         true,
	"AXPopUpButton":        true,
	"AXTabButton":          true,
	"AXCheckBox":           true,
	"AXRadioButton":        true,
	"AXSwitch":             true,
	"AXDisclosureTriangle": true,
	"AXComboBox":           true,
	"AXSlider":             true,
	"AXStaticText":         true,
	"AXImage":              true,
	"AXHeading":            true,
}

// isStaticElement determines if an element should use static (longer) TTL based on its role.
func isStaticElement(info *ElementInfo) bool {
	if info == nil {
		return false
	}

	return staticRoles[info.Role()]
}

// InfoCache implements a thread-safe time-to-live cache for element information.
type InfoCache struct {
	mu              sync.RWMutex
	data            map[uint64][]*CachedInfo // Bucket for hash collisions
	lru             *list.List               // For LRU eviction
	expirationQueue expirationHeap           // Min-heap ordered by expiresAt for cleanup
	maxSize         int                      // Maximum cache size (0 = unlimited)
	stopCh          chan struct{}
	stopped         bool
	logger          *zap.Logger
}

// NewInfoCache initializes a new cache with per-role TTLs and the default maximum size.
func NewInfoCache(logger *zap.Logger) *InfoCache {
	return NewInfoCacheWithSize(DefaultMaxCacheSize, logger)
}

// NewInfoCacheWithSize initializes a new cache with per-role TTLs and the specified maximum size.
func NewInfoCacheWithSize(maxSize int, logger *zap.Logger) *InfoCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	cache := &InfoCache{
		data:    make(map[uint64][]*CachedInfo, DefaultCacheSize),
		lru:     list.New(),
		maxSize: maxSize,
		stopCh:  make(chan struct{}),
		logger:  logger,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached element information if it exists and hasn't expired.
func (c *InfoCache) Get(elem *Element) *ElementInfo {
	if elem == nil {
		return nil
	}

	hash, err := elem.Hash()
	if err != nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	bucket, exists := c.data[hash]
	if !exists {
		return nil
	}

	// Iterate bucket to find exact match
	for idx, cached := range bucket {
		// Use Equal to verify it's the same underlying object
		if elem.Equal(cached.elementRef) {
			// Note: no need to check cached.removed here — all code paths that
			// set removed=true also call removeFromBucket, so an entry found
			// in the bucket is guaranteed to have removed==false.
			if time.Now().After(cached.expiresAt) {
				// Remove expired entry from bucket
				c.removeFromBucket(hash, idx)

				// Mark as removed for lazy heap cleanup
				cached.removed = true

				// Remove from LRU and release
				if cached.elementNode != nil {
					c.lru.Remove(cached.elementNode)
				}

				cached.elementRef.Release()

				return nil
			}

			// Move to front of LRU list (most recently used)
			c.lru.MoveToFront(cached.elementNode)

			return cached.info
		}
	}

	return nil
}

// Set stores element information in the cache with appropriate time-to-live based on element type.
func (c *InfoCache) Set(elem *Element, info *ElementInfo) {
	if elem == nil || info == nil {
		return
	}

	hash, err := elem.Hash()
	if err != nil {
		c.logger.Debug("Failed to get element hash for caching", zap.Error(err))

		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	bucket := c.data[hash]

	// Check if already in cache and update
	for _, cached := range bucket {
		if elem.Equal(cached.elementRef) {
			cached.info = info
			cached.expiresAt = time.Now().Add(c.getTTL(info))
			cached.removed = false
			c.lru.MoveToFront(cached.elementNode)

			// Fix heap position in-place instead of pushing a duplicate entry
			if cached.heapIndex >= 0 {
				heap.Fix(&c.expirationQueue, cached.heapIndex)
			} else {
				heap.Push(&c.expirationQueue, cached)
			}

			c.logger.Debug("Updated cached element info",
				zap.Uint64("hash", hash),
				zap.String("role", info.Role()),
				zap.String("title", info.Title()))

			return
		}
	}

	// Not in cache, add new entry
	// Evict least recently used items if cache is full
	if c.maxSize > 0 && c.lru.Len() >= c.maxSize {
		c.evictLRU()
	}

	// Clone the element to retain it for the cache
	clonedElem, cloneErr := elem.Clone()
	if cloneErr != nil {
		c.logger.Debug("Failed to clone element for caching", zap.Error(cloneErr))

		return
	}

	// Use different TTL based on element type
	ttl := c.getTTL(info)
	expiresAt := time.Now().Add(ttl)

	// Create new cache entry
	cachedInfo := &CachedInfo{
		info:       info,
		expiresAt:  expiresAt,
		key:        hash,
		elementRef: clonedElem,
		heapIndex:  -1,
	}

	// Add to LRU list (front = most recently used)
	cachedInfo.elementNode = c.lru.PushFront(cachedInfo)

	// Add to expiration heap for efficient cleanup
	heap.Push(&c.expirationQueue, cachedInfo)

	// Add to bucket
	c.data[hash] = append(c.data[hash], cachedInfo)

	c.logger.Debug("Caching element info",
		zap.Uint64("hash", hash),
		zap.String("role", info.Role()),
		zap.String("title", info.Title()),
		zap.Duration("ttl", ttl),
		zap.Int("cache_size", c.lru.Len()))
}

// Size returns the current number of entries in the cache.
func (c *InfoCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lru.Len()
}

// Clear removes all entries from the cache.
func (c *InfoCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Release all retained elements
	for _, bucket := range c.data {
		for _, cached := range bucket {
			cached.elementRef.Release()
		}
	}

	c.data = make(map[uint64][]*CachedInfo, DefaultCacheSize)
	c.lru = list.New()
	c.expirationQueue = nil

	c.logger.Debug("Cache cleared")
}

// Stop terminates the cache cleanup goroutine and releases resources.
func (c *InfoCache) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	close(c.stopCh)
	c.stopped = true

	// Release resources
	for _, bucket := range c.data {
		for _, cached := range bucket {
			cached.elementRef.Release()
		}
	}

	c.data = nil
	c.lru = nil
	c.expirationQueue = nil

	c.logger.Debug("Cache stopped")
}

// cleanupLoop runs a periodic cleanup process to remove expired cache entries.
func (c *InfoCache) cleanupLoop() {
	// Tick at DynamicElementTTL/2 (1s) — the shortest per-role TTL — to ensure
	// dynamic elements holding Objective-C references are released promptly.
	// For static-only workloads most ticks are no-ops, but the heap peek is O(1)
	// so the overhead is negligible.
	ticker := time.NewTicker(DynamicElementTTL / CacheCleanupDivisor)

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			c.logger.Debug("Cache cleanup loop stopped")

			return
		}
	}
}

// cleanup removes all expired entries from the cache using the expiration heap.
func (c *InfoCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped || c.lru == nil {
		return
	}

	now := time.Now()
	expiredCount := 0

	// Use min-heap to efficiently find and remove expired items
	// Pop items whose expiration time has passed
	for c.expirationQueue.Len() > 0 {
		cached := c.expirationQueue[0] // Peek at earliest expiration

		// If already removed via LRU eviction, pop and continue (lazy cleanup)
		if cached.removed {
			heap.Pop(&c.expirationQueue)

			continue
		}

		// If not expired yet, we're done (heap is sorted by expiration)
		if !now.After(cached.expiresAt) {
			break
		}

		// Pop the expired item from heap
		heap.Pop(&c.expirationQueue)

		// Remove from bucket
		bucket := c.data[cached.key]
		for itemIdx, item := range bucket {
			if item == cached {
				c.removeFromBucket(cached.key, itemIdx)

				break
			}
		}

		// Remove from LRU list
		if cached.elementNode != nil {
			c.lru.Remove(cached.elementNode)
		}

		// Release element reference
		cached.elementRef.Release()

		// Mark as removed to prevent double-Release from duplicate heap entries
		cached.removed = true

		expiredCount++
	}

	if expiredCount > 0 {
		c.logger.Debug("Cache cleanup completed",
			zap.Int("removed_entries", expiredCount),
			zap.Int("remaining_entries", c.lru.Len()))
	}
}

// removeFromBucket removes an item from a bucket at index i.
func (c *InfoCache) removeFromBucket(key uint64, index int) {
	bucket := c.data[key]
	if index < 0 || index >= len(bucket) {
		return
	}

	// Optimized delete from slice
	lastIdx := len(bucket) - 1
	bucket[index] = bucket[lastIdx]
	bucket[lastIdx] = nil // Avoid memory leak
	c.data[key] = bucket[:lastIdx]

	// If bucket is empty, delete key
	if len(c.data[key]) == 0 {
		delete(c.data, key)
	}
}

// getTTL returns the appropriate TTL duration based on element type.
func (c *InfoCache) getTTL(info *ElementInfo) time.Duration {
	if isStaticElement(info) {
		return StaticElementTTL
	}

	return DynamicElementTTL
}

// evictLRU removes the least recently used item from the cache.
func (c *InfoCache) evictLRU() {
	// Get the least recently used item (back of the list)
	lruElement := c.lru.Back()
	if lruElement == nil {
		return
	}

	// Get the cached info from the element
	cachedInfo, ok := lruElement.Value.(*CachedInfo)
	if !ok {
		c.logger.Error("Invalid cache entry type in LRU list")

		return
	}

	// Mark as removed for lazy heap cleanup
	cachedInfo.removed = true

	// Remove from bucket
	hash := cachedInfo.key

	bucket := c.data[hash]
	for i, item := range bucket {
		if item == cachedInfo {
			c.removeFromBucket(hash, i)

			break
		}
	}

	// Remove from LRU list and release
	c.lru.Remove(lruElement)
	cachedInfo.elementRef.Release()

	c.logger.Debug("Evicted LRU cache entry",
		zap.Uint64("hash", hash),
		zap.String("role", cachedInfo.info.Role()),
		zap.String("title", cachedInfo.info.Title()))
}
