package overlayutil

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

const (
	// DefaultCallbackMapSize is the default size for callback maps.
	DefaultCallbackMapSize = 8
	// DefaultCallbackTimeout is the default timeout for callbacks.
	DefaultCallbackTimeout = 2 * time.Second
)

// CallbackManager manages asynchronous callbacks for overlay operations.
type CallbackManager struct {
	logger      *zap.Logger
	callbackID  uint64
	callbackMap map[uint64]chan struct{}
	callbackMu  sync.Mutex
}

// NewCallbackManager creates a new callback manager.
func NewCallbackManager(logger *zap.Logger) *CallbackManager {
	return &CallbackManager{
		logger:      logger,
		callbackMap: make(map[uint64]chan struct{}, DefaultCallbackMapSize),
	}
}

// StartResizeOperation begins a resize operation with callback tracking.
func (c *CallbackManager) StartResizeOperation(callbackFunc func(uint64)) {
	done := make(chan struct{})

	// Generate unique ID for this callback
	callbackID := atomic.AddUint64(&c.callbackID, 1)

	// Store channel in map
	c.callbackMu.Lock()
	c.callbackMap[callbackID] = done
	c.callbackMu.Unlock()

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
