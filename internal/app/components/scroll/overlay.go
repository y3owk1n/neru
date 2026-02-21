package scroll

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeScrollCompletionCallback(void* context);
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

const (
	// defaultIndicatorWidth is the default width for the scroll indicator.
	defaultIndicatorWidth = 60
	// defaultIndicatorHeight is the default height for the scroll indicator.
	defaultIndicatorHeight = 20
)

//export resizeScrollCompletionCallback
func resizeScrollCompletionCallback(context unsafe.Pointer) {
	// Read callback ID from the pointer (points to a slice element in callbackIDStore)
	id := *(*uint64)(context)

	// Delegate to global callback manager
	overlayutil.CompleteGlobalCallback(id)
}

// Overlay manages the rendering of scroll mode overlays using native platform APIs.
type Overlay struct {
	window          C.OverlayWindow
	scrollConfig    config.ScrollConfig
	indicatorConfig config.ModeIndicatorConfig
	logger          *zap.Logger
	callbackManager *overlayutil.CallbackManager
	styleCache      *overlayutil.StyleCache
}

// NewOverlay initializes a new scroll overlay instance with its own window.
func NewOverlay(
	scrollCfg config.ScrollConfig,
	indicatorCfg config.ModeIndicatorConfig,
	logger *zap.Logger,
) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		scrollConfig:    scrollCfg,
		indicatorConfig: indicatorCfg,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
	}, nil
}

// NewOverlayWithWindow initializes a scroll overlay instance using a shared window.
func NewOverlayWithWindow(
	scrollCfg config.ScrollConfig,
	indicatorCfg config.ModeIndicatorConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		scrollConfig:    scrollCfg,
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

// Config returns the scroll configuration.
func (o *Overlay) Config() config.ScrollConfig {
	return o.scrollConfig
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// Show displays the scroll overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the scroll overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all scroll highlights from the overlay.
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
			(C.ResizeCompletionCallback)(C.resizeScrollCompletionCallback),
			contextPtr,
		)
	})
}

// DrawModeIndicator draws a mode label at the specified position.
func (o *Overlay) DrawModeIndicator(labelText string, xCoordinate, yCoordinate int) {
	// Ensure the indicator window is visible before drawing.
	C.NeruShowOverlayWindow(o.window)

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
		) // No matching in scroll mode
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
		showArrow:        0, // No arrow for scroll indicator
	}

	// Reuse NeruDrawHints which can draw arbitrary labels
	C.NeruDrawHints(o.window, &hint, 1, style)
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(
	scrollCfg config.ScrollConfig,
	indicatorCfg config.ModeIndicatorConfig,
) {
	o.scrollConfig = scrollCfg
	o.indicatorConfig = indicatorCfg
	// Invalidate style cache when config changes
	o.styleCache.Free()
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
