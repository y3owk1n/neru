// Package modeindicator provides the mode indicator overlay component.
package modeindicator

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeModeIndicatorCompletionCallback(void* context);
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

const (
	// defaultIndicatorWidth is the default width for the mode indicator.
	defaultIndicatorWidth = 60
	// defaultIndicatorHeight is the default height for the mode indicator.
	defaultIndicatorHeight = 20

	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

//export resizeModeIndicatorCompletionCallback
func resizeModeIndicatorCompletionCallback(context unsafe.Pointer) {
	// Read callback ID from the pointer (points to a slice element in callbackIDStore)
	id := *(*uint64)(context)

	// Delegate to global callback manager
	overlayutil.CompleteGlobalCallback(id)
}

// Overlay manages the rendering of mode indicator overlays using native platform APIs.
type Overlay struct {
	window          C.OverlayWindow
	indicatorConfig config.ModeIndicatorConfig
	logger          *zap.Logger
	callbackManager *overlayutil.CallbackManager
	styleCache      *overlayutil.StyleCache
}

// NewOverlay initializes a new mode indicator overlay instance with its own window.
func NewOverlay(
	indicatorCfg config.ModeIndicatorConfig,
	logger *zap.Logger,
) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		indicatorConfig: indicatorCfg,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
	}, nil
}

// NewOverlayWithWindow initializes a mode indicator overlay instance using a shared window.
func NewOverlayWithWindow(
	indicatorCfg config.ModeIndicatorConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		indicatorConfig: indicatorCfg,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
	}, nil
}

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// Show displays the mode indicator overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the mode indicator overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all highlights from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen adjusts the overlay window size with callback notification.
func (o *Overlay) ResizeToActiveScreen() {
	o.callbackManager.StartResizeOperation(func(callbackID uint64) {
		// Pass integer ID as opaque pointer context for C callback.
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID)

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.resizeModeIndicatorCompletionCallback),
			contextPtr,
		)
	})
}

// DrawModeIndicator draws a mode label at the specified position.
// The caller is responsible for calling Show() once before the first draw
// (e.g. in startModeIndicatorPolling) rather than showing every tick.
func (o *Overlay) DrawModeIndicator(labelText string, xCoordinate, yCoordinate int) {
	// Offset from cursor to avoid covering it
	xOffset := o.indicatorConfig.IndicatorXOffset
	yOffset := o.indicatorConfig.IndicatorYOffset

	label := C.CString(labelText)
	defer C.free(unsafe.Pointer(label)) //nolint:nlreturn

	// Create a single hint for the indicator
	hint := C.HintData{
		label: label,
		position: C.CGPoint{
			x: C.double(xCoordinate + xOffset),
			y: C.double(yCoordinate + yOffset),
		},
		size: C.CGSize{
			// Size is not strictly needed for just drawing the label, but providing reasonable defaults
			width:  defaultIndicatorWidth,
			height: defaultIndicatorHeight,
		},
		matchedPrefixLength: 0,
	}

	// Use cached style strings to avoid repeated allocations and fix use-after-free
	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(o.indicatorConfig.FontFamily))
		cached.BgColor = unsafe.Pointer(C.CString(o.indicatorConfig.BackgroundColor))
		cached.TextColor = unsafe.Pointer(C.CString(o.indicatorConfig.TextColor))
		cached.MatchedTextColor = unsafe.Pointer(
			C.CString(o.indicatorConfig.TextColor),
		) // No matching in indicator mode
		cached.BorderColor = unsafe.Pointer(C.CString(o.indicatorConfig.BorderColor))
	})

	style := C.HintStyle{
		fontSize:         C.int(o.indicatorConfig.FontSize),
		fontFamily:       (*C.char)(cachedStyle.FontFamily),
		backgroundColor:  (*C.char)(cachedStyle.BgColor),
		textColor:        (*C.char)(cachedStyle.TextColor),
		matchedTextColor: (*C.char)(cachedStyle.MatchedTextColor),
		borderColor:      (*C.char)(cachedStyle.BorderColor),
		borderRadius:     C.int(o.indicatorConfig.BorderRadius),
		borderWidth:      C.int(o.indicatorConfig.BorderWidth),
		padding:          C.int(o.indicatorConfig.Padding),
		showArrow:        0, // No arrow for mode indicator
	}

	// Reuse NeruDrawHints which can draw arbitrary labels
	C.NeruDrawHints(o.window, &hint, 1, style)
}

// SetConfig sets the overlay configuration.
func (o *Overlay) SetConfig(indicatorCfg config.ModeIndicatorConfig) {
	o.indicatorConfig = indicatorCfg
	// Invalidate style cache when config changes
	o.styleCache.Free()
}

// SetSharingType sets the window sharing type for screen sharing visibility.
func (o *Overlay) SetSharingType(hide bool) {
	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(o.window, sharingType)
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	// Clean up callback manager first to stop background goroutines
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}

	if o.window != nil {
		o.styleCache.Free()
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}
