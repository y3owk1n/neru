package scroll

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeScrollCompletionCallback(void* context);
*/
import "C"

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

var (
	scrollCallbackID   uint64
	scrollCallbackMap  = make(map[uint64]chan struct{})
	scrollCallbackLock sync.Mutex
)

//export resizeScrollCompletionCallback
func resizeScrollCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	scrollCallbackLock.Lock()
	if done, ok := scrollCallbackMap[id]; ok {
		close(done)
		delete(scrollCallbackMap, id)
	}
	scrollCallbackLock.Unlock()
}

// Overlay represents a scroll overlay.
type Overlay struct {
	window C.OverlayWindow
	config config.ScrollConfig
	logger *zap.Logger
}

// NewOverlay creates a new overlay.
func NewOverlay(cfg config.ScrollConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, errors.New("failed to create overlay window")
	}
	return &Overlay{
		window: window,
		config: cfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a scroll overlay using a shared window.
func NewOverlayWithWindow(
	cfg config.ScrollConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	return &Overlay{
		window: (C.OverlayWindow)(windowPtr),
		config: cfg,
		logger: logger,
	}, nil
}

// Show shows the overlay.
func (o *Overlay) Show() {
	o.logger.Debug("Showing scroll overlay")
	C.NeruShowOverlayWindow(o.window)
	o.logger.Debug("Scroll overlay shown successfully")
}

// Hide hides the overlay.
func (o *Overlay) Hide() {
	o.logger.Debug("Hiding scroll overlay")
	C.NeruHideOverlayWindow(o.window)
	o.logger.Debug("Scroll overlay hidden successfully")
}

// Clear clears all scroll from the overlay.
func (o *Overlay) Clear() {
	o.logger.Debug("Clearing scroll overlay")
	C.NeruClearOverlay(o.window)
	o.logger.Debug("Scroll overlay cleared successfully")
}

// ResizeToActiveScreen resizes the overlay window to the screen containing the mouse cursor.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// ResizeToActiveScreenSync resizes the overlay window synchronously with callback notification.
func (o *Overlay) ResizeToActiveScreenSync() {
	done := make(chan struct{})

	// Generate unique ID for this callback
	callbackID := atomic.AddUint64(&scrollCallbackID, 1)

	// Store channel in map
	scrollCallbackLock.Lock()
	scrollCallbackMap[callbackID] = done
	scrollCallbackLock.Unlock()

	if o.logger != nil {
		o.logger.Debug("Scroll overlay resize started", zap.Uint64("callback_id", callbackID))
	}

	// Pass ID as context (safe - no Go pointers)
	// Note: uintptr conversion must happen in same expression to satisfy go vet
	C.NeruResizeOverlayToActiveScreenWithCallback(
		o.window,
		(C.ResizeCompletionCallback)(
			unsafe.Pointer(C.resizeScrollCompletionCallback), //nolint:unconvert
		),
		*(*unsafe.Pointer)(unsafe.Pointer(&callbackID)),
	)

	// Don't wait for callback - continue immediately for better UX
	// The resize operation is typically fast and visually complete before callback
	// Start a goroutine to handle cleanup when callback eventually arrives
	go func() {
		if o.logger != nil {
			o.logger.Debug(
				"Scroll overlay resize background cleanup started",
				zap.Uint64("callback_id", callbackID),
			)
		}

		select {
		case <-done:
			// Callback received, normal cleanup already handled in callback
			if o.logger != nil {
				o.logger.Debug(
					"Scroll overlay resize callback received",
					zap.Uint64("callback_id", callbackID),
				)
			}
		case <-time.After(2 * time.Second):
			// Long timeout for cleanup only - callback likely failed
			scrollCallbackLock.Lock()
			delete(scrollCallbackMap, callbackID)
			scrollCallbackLock.Unlock()

			if o.logger != nil {
				o.logger.Debug("Scroll overlay resize cleanup timeout - removed callback from map",
					zap.Uint64("callback_id", callbackID))
			}
		}
	}()
}

// DrawScrollHighlight draws a highlight border around the screen.
func (o *Overlay) DrawScrollHighlight(xCoordinate, yCoordinate, width, height int) {
	o.logger.Debug("DrawScrollHighlight called")

	// Use action config for highlight color and width
	color := o.config.HighlightColor
	borderWidth := o.config.HighlightWidth

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))

	// Build 4 border lines around the rectangle
	lines := make([]C.CGRect, 4)

	// Bottom
	lines[0] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(width), height: C.double(borderWidth)},
	}

	// Top
	lines[1] = C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate + height - borderWidth),
		},
		size: C.CGSize{width: C.double(width), height: C.double(borderWidth)},
	}

	// Left
	lines[2] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(borderWidth), height: C.double(height)},
	}

	// Right
	lines[3] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate + width - borderWidth), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(borderWidth), height: C.double(height)},
	}

	C.NeruDrawGridLines(o.window, &lines[0], C.int(4), cColor, C.int(borderWidth), C.double(1.0))
}

// Destroy destroys the overlay.
func (o *Overlay) Destroy() {
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// CleanupCallbackMap cleans up any pending callbacks in the map
// CleanupCallbackMap removed: centralized overlay manager controls resizes
