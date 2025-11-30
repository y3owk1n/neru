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
		timer.Stop() // Stop timer immediately on success
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
		timer.Stop()
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

// WindowBorderBuilder builds window border rectangles for highlighting.
type WindowBorderBuilder struct{}

// BuildBorderRectangles creates 4 rectangles forming a border around an area.
func (w *WindowBorderBuilder) BuildBorderRectangles(
	xCoordinate, yCoordinate, rectWidth, rectHeight, borderWidth int,
) [4]Rectangle {
	return [4]Rectangle{
		// Bottom
		{
			X:      xCoordinate,
			Y:      yCoordinate,
			Width:  rectWidth,
			Height: borderWidth,
		},
		// Top
		{
			X:      xCoordinate,
			Y:      yCoordinate + rectHeight - borderWidth,
			Width:  rectWidth,
			Height: borderWidth,
		},
		// Left
		{
			X:      xCoordinate,
			Y:      yCoordinate,
			Width:  borderWidth,
			Height: rectHeight,
		},
		// Right
		{
			X:      xCoordinate + rectWidth - borderWidth,
			Y:      yCoordinate,
			Width:  borderWidth,
			Height: rectHeight,
		},
	}
}

// Rectangle represents a screen rectangle.
type Rectangle struct {
	X, Y, Width, Height int
}

// ToCGPoint converts to CoreGraphics point (for C interop).
func (r Rectangle) ToCGPoint() CGPoint {
	return CGPoint{
		X: float64(r.X),
		Y: float64(r.Y),
	}
}

// ToCGSize converts to CoreGraphics size (for C interop).
func (r Rectangle) ToCGSize() CGSize {
	return CGSize{
		Width:  float64(r.Width),
		Height: float64(r.Height),
	}
}

// CGPoint represents a CoreGraphics point.
type CGPoint struct {
	X, Y float64
}

// CGSize represents a CoreGraphics size.
type CGSize struct {
	Width, Height float64
}

// CGRectangle represents a CoreGraphics rectangle.
type CGRectangle struct {
	Origin CGPoint
	Size   CGSize
}

// ToCGRectangle converts Rectangle to CGRectangle.
func (r Rectangle) ToCGRectangle() CGRectangle {
	return CGRectangle{
		Origin: r.ToCGPoint(),
		Size:   r.ToCGSize(),
	}
}

// StyleStringCache provides caching for C strings used in styling.
type StyleStringCache struct {
	mu      sync.RWMutex
	strings map[string]unsafe.Pointer
}

// NewStyleStringCache creates a new style string cache.
func NewStyleStringCache() *StyleStringCache {
	return &StyleStringCache{
		strings: make(map[string]unsafe.Pointer),
	}
}

// GetOrCacheString returns a cached C string for the given Go string
// Note: This is a simplified version - actual implementation would need C interop.
func (s *StyleStringCache) GetOrCacheString(str string) unsafe.Pointer {
	s.mu.RLock()

	if cached, ok := s.strings[str]; ok {
		s.mu.RUnlock()

		return cached
	}

	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check
	if cached, ok := s.strings[str]; ok {
		return cached
	}

	// In real implementation, this would call C.CString(str) to create
	// a C-compatible string. For testing purposes, we store nil as placeholder.
	s.strings[str] = nil

	return nil
}

// Clear frees all cached strings.
func (s *StyleStringCache) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// In real implementation, this would free C strings
	s.strings = make(map[string]unsafe.Pointer)
}
