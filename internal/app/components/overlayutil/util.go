package overlayutil

import (
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

const (
	// DefaultCallbackMapSize is the default size for callback maps.
	DefaultCallbackMapSize = 8
	// DefaultCallbackTimeout is the default timeout for callbacks.
	DefaultCallbackTimeout = 2 * time.Second
	// DefaultCallbackIDStoreCapacity is the default initial capacity for callback ID store.
	DefaultCallbackIDStoreCapacity = 1024
)

var (
	// Global registry mapping callback IDs to CallbackManager instances.
	callbackManagerRegistry   = make(map[uint64]*CallbackManager)
	callbackManagerRegistryMu sync.RWMutex

	// freeCallbackIDs is a pool of available callback IDs for reuse.
	freeCallbackIDs   []uint64
	freeCallbackIDsMu sync.Mutex

	// callbackIDStore stores callback IDs in a fixed-size slice to allow safe pointer conversion.
	// The slice index is the callback ID, and the value is the same ID (for pointer stability).
	// We never reallocate this slice to keep pointers handed to C valid for the lifetime
	// of the process; bump DefaultCallbackIDStoreCapacity if you need more IDs.
	callbackIDStore   = make([]uint64, DefaultCallbackIDStoreCapacity)
	callbackIDStoreMu sync.Mutex
)

func init() {
	// Initialize the free ID pool with all available IDs.
	freeCallbackIDs = make([]uint64, 0, DefaultCallbackIDStoreCapacity)
	for i := range uint64(DefaultCallbackIDStoreCapacity) {
		freeCallbackIDs = append(freeCallbackIDs, i)
	}
}

// CompleteGlobalCallback completes a callback by ID using the global registry.
// This function is called by C callbacks that can't access instance methods.
func CompleteGlobalCallback(callbackID uint64) {
	callbackManagerRegistryMu.RLock()

	manager, ok := callbackManagerRegistry[callbackID]

	callbackManagerRegistryMu.RUnlock()

	if ok {
		manager.CompleteCallback(callbackID)

		// Clean up from global registry
		callbackManagerRegistryMu.Lock()
		delete(callbackManagerRegistry, callbackID)
		callbackManagerRegistryMu.Unlock()
	}

	// Return the ID to the free pool for reuse
	freeCallbackIDsMu.Lock()

	freeCallbackIDs = append(freeCallbackIDs, callbackID)

	freeCallbackIDsMu.Unlock()

	// Note: We don't clean up from callbackIDStore here because the pointer
	// may still be in use by C code. The ID is returned to the free pool for reuse.
}

// CallbackIDToPointer converts a callback ID to unsafe.Pointer in a way that go vet accepts.
// It stores the ID in a fixed-size slice and returns a pointer to the slice element.
// The pointer remains valid for the lifetime of the process since the slice is never reallocated.
func CallbackIDToPointer(callbackID uint64) unsafe.Pointer {
	if callbackID >= uint64(len(callbackIDStore)) {
		// Defensive: avoid silently corrupting memory if we ever run out of slots.
		panic("overlayutil: callbackID exceeds callback ID store capacity")
	}

	callbackIDStoreMu.Lock()

	callbackIDStore[callbackID] = callbackID
	ptr := unsafe.Pointer(&callbackIDStore[callbackID])

	callbackIDStoreMu.Unlock()

	return ptr
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
func (c *CallbackManager) StartResizeOperation(callbackFunc func(uint64)) {
	done := make(chan struct{})

	// Allocate an ID from the free pool
	freeCallbackIDsMu.Lock()

	if len(freeCallbackIDs) == 0 {
		freeCallbackIDsMu.Unlock()
		panic("overlayutil: no available callback IDs")
	}

	callbackID := freeCallbackIDs[len(freeCallbackIDs)-1]
	freeCallbackIDs = freeCallbackIDs[:len(freeCallbackIDs)-1]

	freeCallbackIDsMu.Unlock()

	// Store channel in instance map
	c.callbackMu.Lock()
	c.callbackMap[callbackID] = done
	c.callbackMu.Unlock()

	// Register this callback manager in global registry
	callbackManagerRegistryMu.Lock()

	callbackManagerRegistry[callbackID] = c

	callbackManagerRegistryMu.Unlock()

	if c.logger != nil {
		c.logger.Debug("Overlay resize started", zap.Uint64("callback_id", callbackID))
	}

	// Call the platform-specific resize function
	callbackFunc(callbackID)

	// Start background cleanup goroutine
	go c.handleResizeCallback(callbackID, done)
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

		// Clear the callback map
		c.callbackMu.Lock()
		c.callbackMap = make(map[uint64]chan struct{})
		c.callbackMu.Unlock()

		if c.logger != nil {
			c.logger.Debug("CallbackManager cleanup completed")
		}
	})
}

// handleResizeCallback manages the callback lifecycle.
func (c *CallbackManager) handleResizeCallback(callbackID uint64, done chan struct{}) {
	if c.logger != nil {
		c.logger.Debug(
			"Overlay resize background cleanup started",
			zap.Uint64("callback_id", callbackID),
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

		if c.logger != nil {
			c.logger.Debug(
				"Overlay resize cleanup timeout - removed callback from map",
				zap.Uint64("callback_id", callbackID),
			)
		}
	case <-c.cancelCh:
		// Manager is being cleaned up, clean up this callback
		c.callbackMu.Lock()
		delete(c.callbackMap, callbackID)
		c.callbackMu.Unlock()

		if c.logger != nil {
			c.logger.Debug(
				"Overlay resize callback canceled during cleanup",
				zap.Uint64("callback_id", callbackID),
			)
		}
	}
}
