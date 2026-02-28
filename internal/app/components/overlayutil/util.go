package overlayutil

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"go.uber.org/zap"
)

const (
	// DefaultCallbackMapSize is the default size for callback maps.
	DefaultCallbackMapSize = 8
	// DefaultCallbackTimeout is the default timeout for callbacks.
	DefaultCallbackTimeout = 2 * time.Second
	// DefaultCallbackIDStoreCapacity is the default initial capacity for the callback ID pool.
	DefaultCallbackIDStoreCapacity = 1024
	// StaleCallbackGracePeriod is how long we keep a timed-out/canceled callback
	// ID allocated before forcibly releasing it. This gives late C callbacks time
	// to arrive and be validated via generation mismatch, while preventing
	// permanent ID leaks if the C side never invokes the callback.
	StaleCallbackGracePeriod = 30 * time.Second
)

// registryEntry holds the manager and its generation for validation.
type registryEntry struct {
	manager    *CallbackManager
	generation uint64
}

var (
	// globalGeneration is a process-wide monotonic counter so that no two
	// operations (even across different CallbackManager instances) ever share
	// the same generation value. This prevents a reused callback ID from
	// matching a stale C callback that originated from a different manager.
	globalGeneration atomic.Uint64

	// Global registry mapping callback IDs to registryEntry (manager + generation).
	callbackManagerRegistry   = make(map[uint64]registryEntry)
	callbackManagerRegistryMu sync.RWMutex

	// freeCallbackIDs is a pool of available callback IDs for reuse.
	freeCallbackIDs   []uint64
	freeCallbackIDsMu sync.Mutex

	// allocatedCallbackIDs tracks which IDs are currently allocated to prevent double-release.
	// This is a safety net - IDs should only be released once, but this guards against bugs.
	allocatedCallbackIDs   = make(map[uint64]bool)
	allocatedCallbackIDsMu sync.Mutex
)

// CallbackContext holds both the callback ID and its generation for validation.
type CallbackContext struct {
	CallbackID uint64
	Generation uint64
}

func init() {
	// Initialize the free ID pool with all available IDs.
	freeCallbackIDs = make([]uint64, 0, DefaultCallbackIDStoreCapacity)
	for i := range uint64(DefaultCallbackIDStoreCapacity) {
		freeCallbackIDs = append(freeCallbackIDs, i)
	}
}

// releaseCallbackID releases a callback ID back to the free pool and removes it from the global registry.
// This is safe to call even if the callback ID is not currently registered.
// It includes a guard against double-release by tracking allocated IDs.
func releaseCallbackID(callbackID uint64) {
	// Check if ID is actually allocated before releasing (guards against double-release)
	allocatedCallbackIDsMu.Lock()

	if !allocatedCallbackIDs[callbackID] {
		allocatedCallbackIDsMu.Unlock()
		// ID is not allocated, nothing to release
		return
	}

	delete(allocatedCallbackIDs, callbackID)
	allocatedCallbackIDsMu.Unlock()

	// Remove from global registry
	callbackManagerRegistryMu.Lock()
	delete(callbackManagerRegistry, callbackID)
	callbackManagerRegistryMu.Unlock()

	// Return the ID to the free pool for reuse
	freeCallbackIDsMu.Lock()

	freeCallbackIDs = append(freeCallbackIDs, callbackID)

	freeCallbackIDsMu.Unlock()
}

// deferredReleaseCallbackID schedules a callback ID to be released after StaleCallbackGracePeriod.
// This is used on timeout/cancel paths: the ID stays allocated long enough for a late C callback
// to arrive and be rejected via generation mismatch, but is eventually reclaimed to prevent leaks.
// If the C callback arrives before the grace period expires and calls CompleteGlobalCallback,
// the double-release guard in releaseCallbackID ensures the deferred release is a no-op.
func deferredReleaseCallbackID(callbackID uint64) {
	time.AfterFunc(StaleCallbackGracePeriod, func() {
		releaseCallbackID(callbackID)
	})
}

// CompleteGlobalCallback completes a callback by ID using the global registry.
// This function is called by C callbacks that can't access instance methods.
// It validates the generation to ensure the callback is still valid (not stale).
func CompleteGlobalCallback(callbackID uint64, expectedGeneration uint64) {
	callbackManagerRegistryMu.RLock()

	entry, ok := callbackManagerRegistry[callbackID]

	callbackManagerRegistryMu.RUnlock()

	if !ok {
		return
	}

	// Validate generation to detect stale callbacks (ID was reused)
	if entry.generation != expectedGeneration {
		return
	}

	entry.manager.CompleteCallback(callbackID)

	// Release the callback ID back to the free pool
	releaseCallbackID(callbackID)
}

// CallbackIDToPointer allocates a CallbackContext on the C heap via bridge.MallocCallbackContext
// and returns an unsafe.Pointer to it. Because the memory lives on the C heap (not the Go heap),
// it is safe for C code to retain across async dispatch boundaries and is not subject to Go's GC.
// The caller's C callback must call FreeCallbackContext after reading the values to avoid leaks.
func CallbackIDToPointer(callbackID uint64, generation uint64) unsafe.Pointer {
	return bridge.MallocCallbackContext(callbackID, generation)
}

// FreeCallbackContext frees a C-heap-allocated CallbackContext previously created by CallbackIDToPointer.
// Must be called exactly once per CallbackIDToPointer call, typically in the C callback after reading
// the CallbackID and Generation fields. Safe to call with nil.
func FreeCallbackContext(ptr unsafe.Pointer) {
	bridge.FreeCallbackContext(ptr)
}

// CallbackManager manages asynchronous callbacks for overlay operations.
type CallbackManager struct {
	logger      *zap.Logger
	callbackMap map[uint64]chan struct{}
	callbackMu  sync.Mutex
	cancelCh    chan struct{}
	cleanupOnce sync.Once
}

// NewCallbackManager creates a new callback manager.
func NewCallbackManager(logger *zap.Logger) *CallbackManager {
	return &CallbackManager{
		logger:      logger,
		callbackMap: make(map[uint64]chan struct{}, DefaultCallbackMapSize),
		cancelCh:    make(chan struct{}),
	}
}

// StartResizeOperation begins a resize operation with callback tracking.
// Returns true if the operation was started, false if the callback ID pool
// was exhausted. Callers should fall back to a non-callback resize when false
// is returned so the overlay is still resized correctly.
func (c *CallbackManager) StartResizeOperation(callbackFunc func(uint64, uint64)) bool {
	done := make(chan struct{})

	// Allocate an ID from the free pool
	freeCallbackIDsMu.Lock()

	if len(freeCallbackIDs) == 0 {
		freeCallbackIDsMu.Unlock()

		if c.logger != nil {
			c.logger.Warn(
				"No available callback IDs, skipping resize operation (pool temporarily exhausted by deferred releases)",
			)
		}

		return false
	}

	callbackID := freeCallbackIDs[len(freeCallbackIDs)-1]
	freeCallbackIDs = freeCallbackIDs[:len(freeCallbackIDs)-1]

	freeCallbackIDsMu.Unlock()

	// Mark ID as allocated to guard against double-release
	allocatedCallbackIDsMu.Lock()

	allocatedCallbackIDs[callbackID] = true

	allocatedCallbackIDsMu.Unlock()

	// Mint a globally unique generation for this operation.
	// Using a global counter (not per-manager) ensures that even if two
	// different managers reuse the same callback ID, their generations
	// will never collide.
	currentGeneration := globalGeneration.Add(1)

	// Store channel in instance map
	c.callbackMu.Lock()
	c.callbackMap[callbackID] = done
	c.callbackMu.Unlock()

	// Register this callback manager in global registry with current generation
	callbackManagerRegistryMu.Lock()

	callbackManagerRegistry[callbackID] = registryEntry{
		manager:    c,
		generation: currentGeneration,
	}

	callbackManagerRegistryMu.Unlock()

	if c.logger != nil {
		c.logger.Debug(
			"Overlay resize started",
			zap.Uint64("callback_id", callbackID),
			zap.Uint64("generation", currentGeneration),
		)
	}

	// Call the platform-specific resize function with both ID and generation
	callbackFunc(callbackID, currentGeneration)

	// Start background cleanup goroutine
	go c.handleResizeCallback(callbackID, currentGeneration, done)

	return true
}

// CompleteCallback marks a callback as complete.
func (c *CallbackManager) CompleteCallback(callbackID uint64) {
	c.callbackMu.Lock()

	if done, ok := c.callbackMap[callbackID]; ok {
		close(done)
		delete(c.callbackMap, callbackID)
	}

	c.callbackMu.Unlock()
}

// Cleanup cancels all pending callbacks and stops background goroutines.
// This should be called when the overlay is being destroyed.
// Safe to call multiple times - only executes cleanup once.
func (c *CallbackManager) Cleanup() {
	// Use sync.Once to ensure cleanup only happens once
	// This prevents panic from double-close of the cancel channel
	c.cleanupOnce.Do(func() {
		// Close the cancel channel to stop all background goroutines
		close(c.cancelCh)

		// Remove this manager's entries from the global registry so that
		// late C callbacks find no entry in CompleteGlobalCallback and are
		// rejected immediately, rather than being routed to this cleaned-up
		// manager. The deferred release timers will still fire and call
		// releaseCallbackID, which is a no-op for already-deleted entries.
		c.callbackMu.Lock()
		callbackManagerRegistryMu.Lock()
		for id := range c.callbackMap {
			delete(callbackManagerRegistry, id)
		}

		callbackManagerRegistryMu.Unlock()

		c.callbackMap = make(map[uint64]chan struct{})
		c.callbackMu.Unlock()

		if c.logger != nil {
			c.logger.Debug("CallbackManager cleanup completed")
		}
	})
}

// handleResizeCallback manages the callback lifecycle.
func (c *CallbackManager) handleResizeCallback(
	callbackID uint64,
	generation uint64,
	done chan struct{},
) {
	if c.logger != nil {
		c.logger.Debug(
			"Overlay resize background cleanup started",
			zap.Uint64("callback_id", callbackID),
			zap.Uint64("generation", generation),
		)
	}

	// Use timer instead of time.After to prevent memory leaks
	timer := time.NewTimer(DefaultCallbackTimeout)
	defer timer.Stop()

	select {
	case <-done:
		// Callback received, normal cleanup already handled in callback
		if c.logger != nil {
			c.logger.Debug(
				"Overlay resize callback received",
				zap.Uint64("callback_id", callbackID),
			)
		}
	case <-timer.C:
		// Long timeout for cleanup only - callback likely failed
		c.callbackMu.Lock()
		delete(c.callbackMap, callbackID)
		c.callbackMu.Unlock()

		// Schedule deferred release: keep the ID allocated for StaleCallbackGracePeriod
		// so late C callbacks can still be validated and rejected via generation mismatch,
		// then reclaim the ID to prevent permanent leaks.
		deferredReleaseCallbackID(callbackID)

		if c.logger != nil {
			c.logger.Debug(
				"Overlay resize cleanup timeout - removed callback from map, deferred ID release scheduled",
				zap.Uint64("callback_id", callbackID),
				zap.Uint64("generation", generation),
			)
		}
	case <-c.cancelCh:
		// Manager is being cleaned up, clean up this callback
		c.callbackMu.Lock()
		delete(c.callbackMap, callbackID)
		c.callbackMu.Unlock()

		// Schedule deferred release - same reasoning as timeout case
		deferredReleaseCallbackID(callbackID)

		if c.logger != nil {
			c.logger.Debug(
				"Overlay resize callback canceled during cleanup, deferred ID release scheduled",
				zap.Uint64("callback_id", callbackID),
				zap.Uint64("generation", generation),
			)
		}
	}
}
