package accessibility

import (
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

const (
	// DefaultCacheSize is the default cache size.
	DefaultCacheSize = 100

	// CacheCleanupDivisor is the divisor for cleanup interval.
	CacheCleanupDivisor = 2

	// CacheDeletionEstimate is the estimate for deletion.
	CacheDeletionEstimate = 4

	// StaticElementTTL is the TTL for static UI elements (buttons, links, etc.).
	StaticElementTTL = 30 * time.Second

	// DynamicElementTTL is the TTL for dynamic UI elements (text fields, scrollable content, etc.).
	DynamicElementTTL = 2 * time.Second
)

// CachedInfo wraps ElementInfo with an expiration timestamp for TTL-based caching.
type CachedInfo struct {
	info      *ElementInfo
	expiresAt time.Time
}

// isStaticElement determines if an element should use static (longer) TTL based on its role.
func isStaticElement(info *ElementInfo) bool {
	if info == nil {
		return false
	}

	role := info.Role()

	// Static elements: buttons, links, menu items, tabs, etc. - these don't change often
	staticRoles := map[string]bool{
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
		"AXStaticText":         true, // Static text is usually labels
		"AXImage":              true, // Images are usually static
		"AXHeading":            true, // Headings are usually static
	}

	return staticRoles[role]
}

// InfoCache implements a thread-safe time-to-live cache for element information.
type InfoCache struct {
	mu      sync.RWMutex
	data    map[uintptr]*CachedInfo
	ttl     time.Duration
	stopCh  chan struct{}
	stopped bool
	logger  *zap.Logger
}

// NewInfoCache initializes a new cache with the specified time-to-live duration.
func NewInfoCache(ttl time.Duration, logger *zap.Logger) *InfoCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	cache := &InfoCache{
		data:   make(map[uintptr]*CachedInfo, DefaultCacheSize),
		ttl:    ttl,
		stopCh: make(chan struct{}),
		logger: logger,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached element information if it exists and hasn't expired.
func (c *InfoCache) Get(elem *Element) *ElementInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// #nosec G103 -- Using pointer address as map key is safe for cache
	key := uintptr(unsafe.Pointer(elem))
	cached, exists := c.data[key]

	if !exists {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		return nil
	}

	return cached.info
}

// Set stores element information in the cache with appropriate time-to-live based on element type.
func (c *InfoCache) Set(elem *Element, info *ElementInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// #nosec G103 -- Using pointer address as map key is safe for cache
	key := uintptr(unsafe.Pointer(elem))

	// Use different TTL based on element type
	var ttl time.Duration
	if isStaticElement(info) {
		ttl = StaticElementTTL
	} else {
		ttl = DynamicElementTTL
	}

	expiresAt := time.Now().Add(ttl)
	c.data[key] = &CachedInfo{
		info:      info,
		expiresAt: expiresAt,
	}

	c.logger.Debug("Caching element info",
		zap.Uintptr("element_ptr", key),
		zap.String("role", info.Role()),
		zap.String("title", info.Title()),
		zap.Duration("ttl", ttl),
		zap.Time("expires_at", expiresAt))
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
