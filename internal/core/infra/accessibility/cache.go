package accessibility

import (
	"container/heap"
	"container/list"
	"sync"
	"sync/atomic"
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

	// promotionBufSize is the capacity of the lock-free ring buffer used to
	// defer LRU promotions from the read-only Get() fast path. Hits beyond
	// this capacity are silently dropped (minor LRU accuracy loss under
	// extreme concurrency, but no correctness impact).
	promotionBufSize = 64
)

// cacheStats collects aggregate counters during cache operations.
// All fields use atomic operations for goroutine safety.
type cacheStats struct {
	hits           atomic.Int64
	misses         atomic.Int64
	sets           atomic.Int64
	updates        atomic.Int64
	evictions      atomic.Int64
	expiredRemoved atomic.Int64
	hashErrors     atomic.Int64
	cloneErrors    atomic.Int64
	currentSize    atomic.Int64
}

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
	stats           *cacheStats

	// promotionBuf collects deferred LRU promotions from the read-only
	// Get() fast path. Entries are flushed to the LRU list the next time
	// a write lock is acquired (Set, cleanup, or expired-entry removal).
	// The channel is non-blocking: sends that would block are dropped,
	// trading minor LRU accuracy for zero contention on the hot path.
	promotionBuf chan *list.Element
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
		data:         make(map[uint64][]*CachedInfo, DefaultCacheSize),
		lru:          list.New(),
		maxSize:      maxSize,
		stopCh:       make(chan struct{}),
		logger:       logger,
		stats:        &cacheStats{},
		promotionBuf: make(chan *list.Element, promotionBufSize),
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached element information if it exists and hasn't expired.
//
// It uses a two-phase locking strategy to reduce contention during parallel
// tree building: a read lock for the common cache-hit path, upgrading to a
// write lock only when an expired entry must be removed. On a cache hit the
// LRU promotion is deferred to a buffered channel and flushed the next time a
// write lock is acquired, keeping the hot path fully concurrent.
func (c *InfoCache) Get(elem *Element) *ElementInfo {
	if elem == nil {
		return nil
	}

	hash, err := elem.Hash()
	if err != nil {
		return nil
	}

	// --- Phase 1: optimistic read lock (no mutations) ---
	c.mu.RLock()

	if c.stopped {
		c.mu.RUnlock()

		return nil
	}

	bucket, exists := c.data[hash]
	if !exists {
		c.mu.RUnlock()

		return nil
	}
	// Scan the bucket under the read lock.
	var (
		foundInfo    *ElementInfo
		foundIdx     = -1
		foundExpired bool
	)
	for idx, cached := range bucket {
		if elem.Equal(cached.elementRef) {
			if time.Now().After(cached.expiresAt) {
				foundIdx = idx
				foundExpired = true
			} else {
				// Cache hit, not expired. Defer LRU promotion via the
				// non-blocking promotion buffer to stay under the read
				// lock. If the buffer is full the send is dropped — a
				// minor LRU accuracy loss with no correctness impact.
				foundInfo = cached.info
				select {
				case c.promotionBuf <- cached.elementNode:
				default:
				}
			}

			break
		}
	}

	c.mu.RUnlock()
	// Fast path: cache hit, not expired — return immediately.
	if foundInfo != nil {
		if c.stats != nil {
			c.stats.hits.Add(1)
		}

		return foundInfo
	}
	// Fast path: no match at all.
	if foundIdx == -1 && !foundExpired {
		if c.stats != nil {
			c.stats.misses.Add(1)
		}

		return nil
	}
	// --- Phase 2: write lock to remove the expired entry ---
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		if c.stats != nil {
			c.stats.misses.Add(1)
		}

		return nil
	}

	// Flush any deferred LRU promotions while we hold the write lock.
	c.drainPromotions()

	// Re-fetch the bucket; another goroutine may have modified it between
	// the RUnlock and Lock.
	bucket, exists = c.data[hash]

	if !exists {
		if c.stats != nil {
			c.stats.misses.Add(1)
		}

		return nil
	}

	for idx, cached := range bucket {
		if elem.Equal(cached.elementRef) {
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

				if c.stats != nil {
					c.stats.misses.Add(1)
					c.stats.expiredRemoved.Add(1)
					c.stats.currentSize.Store(int64(c.lru.Len()))
				}

				return nil
			}

			// Entry was expired when we checked under RLock but another
			// goroutine refreshed it before we acquired the write lock.
			// Promote in LRU since we already hold the write lock.
			c.lru.MoveToFront(cached.elementNode)

			if c.stats != nil {
				c.stats.hits.Add(1)
			}

			return cached.info
		}
	}

	if c.stats != nil {
		c.stats.misses.Add(1)
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
		if c.stats != nil {
			c.stats.hashErrors.Add(1)
		}

		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	// Flush any deferred LRU promotions while we hold the write lock.
	c.drainPromotions()

	bucket := c.data[hash]

	// Check if already in cache and update
	for _, cached := range bucket {
		if elem.Equal(cached.elementRef) {
			cached.info = info
			cached.expiresAt = time.Now().Add(c.getTTL(info))
			// Note: cached.removed is guaranteed false here (same invariant as Get).
			c.lru.MoveToFront(cached.elementNode)

			// Fix heap position in-place instead of pushing a duplicate entry.
			// heapIndex should always be >= 0 here since entries are pushed
			// onto the heap when created and only popped by cleanup().
			if cached.heapIndex >= 0 {
				heap.Fix(&c.expirationQueue, cached.heapIndex)
			} else {
				c.logger.Warn("Cache entry missing from expiration heap during update",
					zap.Uint64("hash", hash),
					zap.String("role", info.Role()))
				heap.Push(&c.expirationQueue, cached)
			}

			if c.stats != nil {
				c.stats.updates.Add(1)
			}

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
		if c.stats != nil {
			c.stats.cloneErrors.Add(1)
		}

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

	if c.stats != nil {
		c.stats.sets.Add(1)
		c.stats.currentSize.Store(int64(c.lru.Len()))
	}
}

// Size returns the current number of entries in the cache.
func (c *InfoCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stopped {
		return 0
	}

	return c.lru.Len()
}

// Stats returns the current cache statistics.
func (c *InfoCache) Stats() *cacheStats {
	return c.stats
}

// EmitStats logs aggregate cache statistics at debug level.
func (c *InfoCache) EmitStats() {
	if c.stats == nil {
		return
	}

	if ce := c.logger.Check(zap.DebugLevel, "Cache statistics"); ce != nil {
		ce.Write(
			zap.Int64("hits", c.stats.hits.Load()),
			zap.Int64("misses", c.stats.misses.Load()),
			zap.Int64("sets", c.stats.sets.Load()),
			zap.Int64("updates", c.stats.updates.Load()),
			zap.Int64("evictions", c.stats.evictions.Load()),
			zap.Int64("expired_removed", c.stats.expiredRemoved.Load()),
			zap.Int64("hash_errors", c.stats.hashErrors.Load()),
			zap.Int64("clone_errors", c.stats.cloneErrors.Load()),
			zap.Int64("current_size", c.stats.currentSize.Load()))
	}
}

// Clear removes all entries from the cache.
func (c *InfoCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Discard any deferred LRU promotions — the entire LRU list is about
	// to be replaced so promoting into the old list is pointless.
	c.drainPromotions()

	// Release all retained elements
	for _, bucket := range c.data {
		for _, cached := range bucket {
			cached.elementRef.Release()
		}
	}

	// Reset all data structures. No need to mark entries removed — the old
	// heap, bucket map, and LRU list are all discarded so no code path will
	// ever process the old CachedInfo pointers again.
	c.data = make(map[uint64][]*CachedInfo, DefaultCacheSize)
	c.lru = list.New()
	c.expirationQueue = nil

	if c.stats != nil {
		c.stats.currentSize.Store(0)
	}

	if ce := c.logger.Check(zap.DebugLevel, "Cache cleared"); ce != nil {
		ce.Write()
	}
}

// Stop terminates the cache cleanup goroutine and releases resources.
func (c *InfoCache) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	// Discard any deferred LRU promotions before tearing down.
	c.drainPromotions()

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

// drainPromotions flushes all pending LRU promotions from the promotion
// buffer. Must be called while c.mu is held for writing.
func (c *InfoCache) drainPromotions() {
	for {
		select {
		case node := <-c.promotionBuf:
			c.lru.MoveToFront(node)
		default:
			return
		}
	}
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

	// Flush any deferred LRU promotions while we hold the write lock.
	c.drainPromotions()

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

	if expiredCount > 0 && c.stats != nil {
		c.stats.expiredRemoved.Add(int64(expiredCount))
		c.stats.currentSize.Store(int64(c.lru.Len()))
		c.EmitStats()
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

	// Mark as removed for lazy heap cleanup instead of calling heap.Remove
	// (O(log n)) on the hot path. Ghost entries remain in the heap until their
	// expiresAt is reached, at which point cleanup() pops and discards them.
	// The number of ghosts is bounded by maxSize.
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

	if c.stats != nil {
		c.stats.evictions.Add(1)
		c.stats.currentSize.Store(int64(c.lru.Len()))
	}
}
