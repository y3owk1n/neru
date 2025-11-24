package action

/*
#cgo CFLAGS: -x objective-c
#include "../../infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeActionCompletionCallback(void* context);
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"go.uber.org/zap"
)

var (
	actionCallbackID   uint64
	actionCallbackMap  = make(map[uint64]chan struct{}, 8) // Pre-size for typical usage
	actionCallbackLock sync.Mutex
)

//export resizeActionCompletionCallback
func resizeActionCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	actionCallbackLock.Lock()
	if done, ok := actionCallbackMap[id]; ok {
		close(done)
		delete(actionCallbackMap, id)
	}
	actionCallbackLock.Unlock()
}

// Overlay manages the rendering of action mode overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.ActionConfig
	logger *zap.Logger
}

// NewOverlay initializes a new action overlay instance with its own window.
func NewOverlay(cfg config.ActionConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeOverlayFailed, "failed to create overlay window")
	}

	return &Overlay{
		window: window,
		config: cfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow initializes an action overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.ActionConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	return &Overlay{
		window: (C.OverlayWindow)(windowPtr),
		config: config,
		logger: logger,
	}, nil
}

// GetWindow returns the underlying C overlay window.
func (o *Overlay) GetWindow() C.OverlayWindow { return o.window }

// GetConfig returns the action configuration.
func (o *Overlay) GetConfig() config.ActionConfig { return o.config }

// GetLogger returns the logger.
func (o *Overlay) GetLogger() *zap.Logger { return o.logger }

// Show displays the action overlay window.
func (o *Overlay) Show() {
	o.logger.Debug("Showing action overlay")
	C.NeruShowOverlayWindow(o.window)
	o.logger.Debug("Action overlay shown successfully")
}

// Hide conceals the action overlay window.
func (o *Overlay) Hide() {
	o.logger.Debug("Hiding action overlay")
	C.NeruHideOverlayWindow(o.window)
	o.logger.Debug("Action overlay hidden successfully")
}

// Clear removes all action highlights from the overlay.
func (o *Overlay) Clear() {
	o.logger.Debug("Clearing action overlay")
	C.NeruClearOverlay(o.window)
	o.logger.Debug("Action overlay cleared successfully")
}

// ResizeToActiveScreen adjusts the overlay window size to match the screen containing the mouse cursor.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// ResizeToActiveScreenSync adjusts the overlay window size synchronously with callback notification.
func (o *Overlay) ResizeToActiveScreenSync() {
	done := make(chan struct{})

	// Generate unique ID for this callback
	callbackID := atomic.AddUint64(&actionCallbackID, 1)

	// Store channel in map
	actionCallbackLock.Lock()
	actionCallbackMap[callbackID] = done
	actionCallbackLock.Unlock()

	if o.logger != nil {
		o.logger.Debug("Action overlay resize started", zap.Uint64("callback_id", callbackID))
	}

	// Pass ID as context (safe - no Go pointers)
	// Note: uintptr conversion must happen in same expression to satisfy go vet
	C.NeruResizeOverlayToActiveScreenWithCallback(
		o.window,
		(C.ResizeCompletionCallback)(
			unsafe.Pointer(C.resizeActionCompletionCallback), //nolint:unconvert
		),
		*(*unsafe.Pointer)(unsafe.Pointer(&callbackID)),
	)

	// Don't wait for callback - continue immediately for better UX
	// The resize operation is typically fast and visually complete before callback
	// Start a goroutine to handle cleanup when callback eventually arrives
	go func() {
		if o.logger != nil {
			o.logger.Debug(
				"Action overlay resize background cleanup started",
				zap.Uint64("callback_id", callbackID),
			)
		}

		// Use timer instead of time.After to prevent memory leaks
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		select {
		case <-done:
			timer.Stop() // Stop timer immediately on success
			// Callback received, normal cleanup already handled in callback
			if o.logger != nil {
				o.logger.Debug(
					"Action overlay resize callback received",
					zap.Uint64("callback_id", callbackID),
				)
			}
		case <-timer.C:
			// Long timeout for cleanup only - callback likely failed
			actionCallbackLock.Lock()
			delete(actionCallbackMap, callbackID)
			actionCallbackLock.Unlock()

			if o.logger != nil {
				o.logger.Debug("Action overlay resize cleanup timeout - removed callback from map",
					zap.Uint64("callback_id", callbackID))
			}
		}
	}()
}

// DrawActionHighlight renders a highlight border around the specified screen area.
func (o *Overlay) DrawActionHighlight(xCoordinate, yCoordinate, width, height int) {
	o.logger.Debug("DrawActionHighlight called")

	// Use action config for highlight color and width
	color := o.config.HighlightColor
	highlightWidth := o.config.HighlightWidth

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	// Build 4 border lines around the rectangle
	lines := make([]C.CGRect, 4)

	// Bottom
	lines[0] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(width), height: C.double(highlightWidth)},
	}

	// Top
	lines[1] = C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate + height - highlightWidth),
		},
		size: C.CGSize{width: C.double(width), height: C.double(highlightWidth)},
	}

	// Left
	lines[2] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(highlightWidth), height: C.double(height)},
	}

	// Right
	lines[3] = C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate + width - highlightWidth),
			y: C.double(yCoordinate),
		},
		size: C.CGSize{width: C.double(highlightWidth), height: C.double(height)},
	}

	C.NeruDrawGridLines(o.window, &lines[0], C.int(4), cColor, C.int(highlightWidth), C.double(1.0))
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(cfg config.ActionConfig) {
	o.config = cfg
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}
