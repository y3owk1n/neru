package accessibility

import (
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
	mu      sync.RWMutex
	data    map[uint64][]*CachedInfo // Bucket for hash collisions
	lru     *list.List               // For LRU eviction
	maxSize int                      // Maximum cache size (0 = unlimited)
	ttl     time.Duration
	stopCh  chan struct{}
	stopped bool
	logger  *zap.Logger
}

// NewInfoCache initializes a new cache with the specified time-to-live duration.
func NewInfoCache(ttl time.Duration, logger *zap.Logger) *InfoCache {
	return NewInfoCacheWithSize(ttl, DefaultMaxCacheSize, logger)
}

// NewInfoCacheWithSize initializes a new cache with the specified time-to-live duration and maximum size.
func NewInfoCacheWithSize(ttl time.Duration, maxSize int, logger *zap.Logger) *InfoCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	cache := &InfoCache{
		data:    make(map[uint64][]*CachedInfo, DefaultCacheSize),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
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
	for i, cached := range bucket {
		// Use Equal to verify it's the same underlying object
		if elem.Equal(cached.elementRef) {
			if time.Now().After(cached.expiresAt) {
				// Remove expired entry from bucket
				c.removeFromBucket(hash, i)

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
			c.lru.MoveToFront(cached.elementNode)
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
	}

	// Add to LRU list (front = most recently used)
	cachedInfo.elementNode = c.lru.PushFront(cachedInfo)

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

	c.logger.Debug("Cache stopped")
}

// cleanupLoop runs a periodic cleanup process to remove expired cache entries.
func (c *InfoCache) cleanupLoop() {
	// Use shortest TTL (dynamic elements) for cleanup interval to ensure
	// prompt cleanup of expired items regardless of cache-level TTL setting
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

// cleanup removes all expired entries from the cache.
func (c *InfoCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped || c.lru == nil {
		return
	}

	now := time.Now()
	expiredCount := 0

	// Full scan of all items to find expired entries
	// With only 1000 items max, full scan is simple, correct, and cache-friendly

	for element := c.lru.Front(); element != nil; {
		cached, ok := element.Value.(*CachedInfo)
		if !ok {
			// Should ensure we iterate correctly even if type is wrong, but safe to skip or log
			element = element.Next()

			continue
		}

		next := element.Next() // Save next since we might remove element

		if now.After(cached.expiresAt) {
			// Found expired item
			bucket := c.data[cached.key]

			// Find and remove from bucket
			for i, item := range bucket {
				if item == cached {
					c.removeFromBucket(cached.key, i)

					break
				}
			}

			c.lru.Remove(element)
			cached.elementRef.Release()

			expiredCount++
		}

		element = next
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
