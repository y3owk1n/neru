package accessibility

import (
	"sync"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"go.uber.org/zap"
)

const (
	// DefaultCacheSize is the default cache size.
	DefaultCacheSize = 100

	// CacheCleanupDivisor is the divisor for cleanup interval.
	CacheCleanupDivisor = 2

	// CacheDeletionEstimate is the estimate for deletion.
	CacheDeletionEstimate = 4
)

// CachedInfo wraps ElementInfo with an expiration timestamp for TTL-based caching.
type CachedInfo struct {
	info      *ElementInfo
	expiresAt time.Time
}

// InfoCache implements a thread-safe time-to-live cache for element information.
type InfoCache struct {
	mu      sync.RWMutex
	data    map[uintptr]*CachedInfo
	ttl     time.Duration
	stopCh  chan struct{}
	stopped bool
}

// NewInfoCache initializes a new cache with the specified time-to-live duration.
func NewInfoCache(ttl time.Duration) *InfoCache {
	cache := &InfoCache{
		data:   make(map[uintptr]*CachedInfo, DefaultCacheSize),
		ttl:    ttl,
		stopCh: make(chan struct{}),
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

// Set stores element information in the cache with the configured time-to-live.
func (c *InfoCache) Set(elem *Element, info *ElementInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// #nosec G103 -- Using pointer address as map key is safe for cache
	key := uintptr(unsafe.Pointer(elem))
	expiresAt := time.Now().Add(c.ttl)
	c.data[key] = &CachedInfo{
		info:      info,
		expiresAt: expiresAt,
	}

	logger.Debug("Caching element info",
		zap.Uintptr("element_ptr", key),
		zap.String("role", info.Role()),
		zap.String("title", info.Title()),
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

	logger.Debug("Cache cleared")
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

	logger.Debug("Cache stopped")
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
			logger.Debug("Cache cleanup loop stopped")

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
		logger.Debug("Cache cleanup completed",
			zap.Int("removed_entries", len(toDelete)),
			zap.Int("remaining_entries", len(c.data)))
	}
}
