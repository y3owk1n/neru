package accessibility

import (
	"container/list"
	"sync"
	"time"
	"unsafe"

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
	info      *ElementInfo
	expiresAt time.Time
	key       uintptr
	element   list.Element // For LRU tracking
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
// InfoCache provides thread-safe caching of element information with TTL-based expiration and LRU eviction.
type InfoCache struct {
	mu      sync.RWMutex
	data    map[uintptr]*CachedInfo
	lru     *list.List // For LRU eviction
	maxSize int        // Maximum cache size (0 = unlimited)
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
		data:    make(map[uintptr]*CachedInfo, DefaultCacheSize),
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// #nosec G103 -- Using pointer address as map key is safe for cache
	key := uintptr(unsafe.Pointer(elem))
	cached, exists := c.data[key]

	if !exists {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		// Remove expired entry
		delete(c.data, key)

		if cached.element.Prev() != nil || cached.element.Next() != nil {
			c.lru.Remove(&cached.element)
		}

		return nil
	}

	// Move to front of LRU list (most recently used)
	c.lru.MoveToFront(&cached.element)

	return cached.info
}

// Set stores element information in the cache with appropriate time-to-live based on element type.
func (c *InfoCache) Set(elem *Element, info *ElementInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// #nosec G103 -- Using pointer address as map key is safe for cache
	key := uintptr(unsafe.Pointer(elem))

	// Check if entry already exists
	existing, exists := c.data[key]
	if exists {
		// Update existing entry and move to front
		existing.info = info
		existing.expiresAt = time.Now().Add(c.getTTL(info))
		c.lru.MoveToFront(&existing.element)
		c.logger.Debug("Updated cached element info",
			zap.Uintptr("element_ptr", key),
			zap.String("role", info.Role()),
			zap.String("title", info.Title()))

		return
	}

	// Evict least recently used items if cache is full
	if c.maxSize > 0 && len(c.data) >= c.maxSize {
		c.evictLRU()
	}

	// Use different TTL based on element type
	ttl := c.getTTL(info)
	expiresAt := time.Now().Add(ttl)

	// Create new cache entry
	cachedInfo := &CachedInfo{
		info:      info,
		expiresAt: expiresAt,
		key:       key,
	}

	// Add to LRU list (front = most recently used)
	cachedInfo.element = *c.lru.PushFront(cachedInfo)
	c.data[key] = cachedInfo

	c.logger.Debug("Caching element info",
		zap.Uintptr("element_ptr", key),
		zap.String("role", info.Role()),
		zap.String("title", info.Title()),
		zap.Duration("ttl", ttl),
		zap.Time("expires_at", expiresAt),
		zap.Int("cache_size", len(c.data)))
}

// Size returns the current number of entries in the cache.
func (c *InfoCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}

// Clear removes all entries from the cache.
func (c *InfoCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[uintptr]*CachedInfo, DefaultCacheSize)
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

	c.logger.Debug("Cache stopped")
}

// cleanupLoop runs a periodic cleanup process to remove expired cache entries.
func (c *InfoCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / CacheCleanupDivisor) // Cleanup at half the TTL interval
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

	now := time.Now()
	// Pre-allocate slice for keys to delete
	toDelete := make(
		[]uintptr,
		0,
		len(c.data)/CacheDeletionEstimate,
	) // Estimate 25% might be expired

	for key, cached := range c.data {
		if now.After(cached.expiresAt) {
			toDelete = append(toDelete, key)
			// Remove from LRU list
			if cached.element.Prev() != nil || cached.element.Next() != nil {
				c.lru.Remove(&cached.element)
			}
		}
	}

	// Batch delete
	for _, key := range toDelete {
		delete(c.data, key)
	}

	if len(toDelete) > 0 {
		c.logger.Debug("Cache cleanup completed",
			zap.Int("removed_entries", len(toDelete)),
			zap.Int("remaining_entries", len(c.data)))
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

	// Remove from LRU list and data map
	c.lru.Remove(lruElement)
	delete(c.data, cachedInfo.key)

	c.logger.Debug("Evicted LRU cache entry",
		zap.Uintptr("element_ptr", cachedInfo.key),
		zap.String("role", cachedInfo.info.Role()),
		zap.String("title", cachedInfo.info.Title()))
}
